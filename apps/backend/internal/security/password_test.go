package security

import "testing"

func TestPasswordHashRoundTrip(t *testing.T) {
	hash, err := HashPassword("123456789012345")
	if err != nil {
		t.Fatal(err)
	}

	ok, err := VerifyPassword(hash, "123456789012345")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected password to verify")
	}

	ok, err = VerifyPassword(hash, "543210987654321")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected wrong password to fail")
	}
}

func TestPasswordHashRejectsShortPassword(t *testing.T) {
	if _, err := HashPassword("12345678901234"); err != ErrPasswordTooShort {
		t.Fatalf("expected ErrPasswordTooShort, got %v", err)
	}
	if _, err := HashPassword("🔐🔐🔐🔐🔐"); err != ErrPasswordTooShort {
		t.Fatalf("expected unicode passwords to use character count, got %v", err)
	}
}
