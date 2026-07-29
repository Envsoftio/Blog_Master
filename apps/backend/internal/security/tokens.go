package security

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

func RandomToken(byteLength int) (string, error) {
	if byteLength < 16 {
		byteLength = 16
	}
	buf := make([]byte, byteLength)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate random token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func TokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func RandomID(prefix string) (string, error) {
	token, err := RandomToken(12)
	if err != nil {
		return "", err
	}
	if prefix == "" {
		return token, nil
	}
	return prefix + "_" + token, nil
}
