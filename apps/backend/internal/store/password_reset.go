package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

const PasswordResetTTL = time.Hour

var ErrInvalidPasswordReset = errors.New("invalid or expired password reset")

type PasswordResetTarget struct {
	UserID string
	Email  string
}

func (s *Store) CreatePasswordReset(
	ctx context.Context,
	email string,
	tokenHash string,
	now time.Time,
) (PasswordResetTarget, bool, error) {
	var target PasswordResetTarget
	err := s.db.QueryRowContext(ctx, `
		SELECT id, email_normalized
		FROM users
		WHERE email_normalized = ?
		  AND status = 'active'
		  AND password_hash IS NOT NULL
	`, normalizeEmail(email)).Scan(&target.UserID, &target.Email)
	if errors.Is(err, sql.ErrNoRows) {
		return PasswordResetTarget{}, false, nil
	}
	if err != nil {
		return PasswordResetTarget{}, false, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return PasswordResetTarget{}, false, err
	}
	defer tx.Rollback()

	nowValue := now.UTC().Format(timeFormat)
	if _, err := tx.ExecContext(ctx, `
		UPDATE password_resets
		SET used_at = ?
		WHERE user_id = ?
		  AND used_at IS NULL
	`, nowValue, target.UserID); err != nil {
		return PasswordResetTarget{}, false, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO password_resets(token_hash, user_id, expires_at, created_at)
		VALUES (?, ?, ?, ?)
	`, tokenHash, target.UserID, now.Add(PasswordResetTTL).UTC().Format(timeFormat), nowValue); err != nil {
		return PasswordResetTarget{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return PasswordResetTarget{}, false, err
	}
	return target, true, nil
}

func (s *Store) CompletePasswordReset(
	ctx context.Context,
	tokenHash string,
	passwordHash string,
	now time.Time,
) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	nowValue := now.UTC().Format(timeFormat)
	var userID string
	err = tx.QueryRowContext(ctx, `
		SELECT reset.user_id
		FROM password_resets reset
		JOIN users user ON user.id = reset.user_id
		WHERE reset.token_hash = ?
		  AND reset.used_at IS NULL
		  AND reset.expires_at > ?
		  AND user.status = 'active'
	`, tokenHash, nowValue).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrInvalidPasswordReset
	}
	if err != nil {
		return err
	}

	result, err := tx.ExecContext(ctx, `
		UPDATE password_resets
		SET used_at = ?
		WHERE token_hash = ?
		  AND used_at IS NULL
		  AND expires_at > ?
	`, nowValue, tokenHash, nowValue)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return ErrInvalidPasswordReset
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE users
		SET password_hash = ?,
		    password_changed_at = ?,
		    updated_at = ?
		WHERE id = ?
	`, passwordHash, nowValue, nowValue, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE password_resets
		SET used_at = ?
		WHERE user_id = ?
		  AND used_at IS NULL
	`, nowValue, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE sessions
		SET revoked_at = ?
		WHERE user_id = ?
		  AND revoked_at IS NULL
	`, nowValue, userID); err != nil {
		return err
	}
	return tx.Commit()
}
