package security

import (
	"strings"
	"testing"
)

func TestSecretEnvelopeRoundTripAndTamperDetection(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	envelope, err := EncryptSecret(key, "webhook-secret")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(envelope, "webhook-secret") {
		t.Fatal("encrypted envelope exposed plaintext")
	}
	plaintext, err := DecryptSecret(key, envelope)
	if err != nil {
		t.Fatal(err)
	}
	if plaintext != "webhook-secret" {
		t.Fatalf("expected decrypted secret, got %q", plaintext)
	}

	last := envelope[len(envelope)-1]
	replacement := byte('A')
	if last == replacement {
		replacement = 'B'
	}
	if _, err := DecryptSecret(key, envelope[:len(envelope)-1]+string(replacement)); err == nil {
		t.Fatal("expected tampered envelope to be rejected")
	}
}

func TestSecretEnvelopeRequiresAES256Key(t *testing.T) {
	if _, err := EncryptSecret([]byte("too-short"), "secret"); err == nil {
		t.Fatal("expected invalid key length to fail")
	}
}
