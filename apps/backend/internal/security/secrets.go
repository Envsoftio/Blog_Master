package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

const secretEnvelopeVersion = "v1"

func EncryptSecret(key []byte, plaintext string) (string, error) {
	aead, err := secretAEAD(key)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate secret nonce: %w", err)
	}
	sealed := aead.Seal(nil, nonce, []byte(plaintext), []byte(secretEnvelopeVersion))
	envelope := append(nonce, sealed...)
	return secretEnvelopeVersion + "." + base64.RawURLEncoding.EncodeToString(envelope), nil
}

func DecryptSecret(key []byte, envelope string) (string, error) {
	version, encoded, found := strings.Cut(strings.TrimSpace(envelope), ".")
	if !found || version != secretEnvelopeVersion {
		return "", fmt.Errorf("unsupported encrypted secret envelope")
	}
	ciphertext, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decode encrypted secret: %w", err)
	}
	aead, err := secretAEAD(key)
	if err != nil {
		return "", err
	}
	if len(ciphertext) <= aead.NonceSize() {
		return "", fmt.Errorf("encrypted secret is truncated")
	}
	plaintext, err := aead.Open(
		nil,
		ciphertext[:aead.NonceSize()],
		ciphertext[aead.NonceSize():],
		[]byte(secretEnvelopeVersion),
	)
	if err != nil {
		return "", fmt.Errorf("decrypt secret: %w", err)
	}
	return string(plaintext), nil
}

func secretAEAD(key []byte) (cipher.AEAD, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("secret encryption key must be exactly 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("initialize secret cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("initialize secret envelope: %w", err)
	}
	return aead, nil
}
