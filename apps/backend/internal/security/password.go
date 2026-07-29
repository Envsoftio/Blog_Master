package security

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	passwordMinLength = 15
	argonTime         = uint32(3)
	argonMemoryKiB    = uint32(64 * 1024)
	argonThreads      = uint8(1)
	argonKeyLength    = uint32(32)
	saltLength        = 16
)

var ErrPasswordTooShort = errors.New("password must be at least 15 characters")

func HashPassword(password string) (string, error) {
	if len(password) < passwordMinLength {
		return "", ErrPasswordTooShort
	}

	salt := make([]byte, saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}

	hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemoryKiB, argonThreads, argonKeyLength)
	encodedSalt := base64.RawStdEncoding.EncodeToString(salt)
	encodedHash := base64.RawStdEncoding.EncodeToString(hash)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", argonMemoryKiB, argonTime, argonThreads, encodedSalt, encodedHash), nil
}

func VerifyPassword(encodedHash, password string) (bool, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false, errors.New("unsupported password hash format")
	}

	memory, timeCost, threads, err := parseArgonParams(parts[3])
	if err != nil {
		return false, err
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("decode password salt: %w", err)
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("decode password hash: %w", err)
	}

	actual := argon2.IDKey([]byte(password), salt, timeCost, memory, threads, uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1, nil
}

func parseArgonParams(raw string) (uint32, uint32, uint8, error) {
	var memory, timeCost uint32
	var threads uint8
	for _, item := range strings.Split(raw, ",") {
		key, value, ok := strings.Cut(item, "=")
		if !ok {
			return 0, 0, 0, errors.New("invalid password hash parameters")
		}
		parsed, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("parse password hash parameter %s: %w", key, err)
		}
		switch key {
		case "m":
			memory = uint32(parsed)
		case "t":
			timeCost = uint32(parsed)
		case "p":
			if parsed > 255 {
				return 0, 0, 0, errors.New("password hash parallelism is too large")
			}
			threads = uint8(parsed)
		default:
			return 0, 0, 0, fmt.Errorf("unknown password hash parameter %s", key)
		}
	}
	if memory == 0 || timeCost == 0 || threads == 0 {
		return 0, 0, 0, errors.New("missing password hash parameters")
	}
	return memory, timeCost, threads, nil
}
