package security

import "testing"

func TestPasswordHashRoundTrip(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}

	ok, err := VerifyPassword(hash, "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected password to verify")
	}

	ok, err = VerifyPassword(hash, "wrong horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected wrong password to fail")
	}
}

func TestPasswordHashRejectsShortPassword(t *testing.T) {
	if _, err := HashPassword("too-short"); err != ErrPasswordTooShort {
		t.Fatalf("expected ErrPasswordTooShort, got %v", err)
	}
}
