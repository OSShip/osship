package passhash

import "testing"

func TestHashAndVerifyPassword(t *testing.T) {
	salt, hash, err := HashPasswordPair("password123")
	if err != nil {
		t.Fatal(err)
	}
	if salt == "" || hash == "" {
		t.Fatal("expected non-empty salt and hash")
	}
	if !VerifyPassword("password123", salt, hash) {
		t.Fatal("expected password to verify")
	}
	if VerifyPassword("wrong", salt, hash) {
		t.Fatal("expected wrong password to fail")
	}
}

func TestUniqueSaltsPerHash(t *testing.T) {
	salt1, _, _ := HashPasswordPair("same-password")
	salt2, _, _ := HashPasswordPair("same-password")
	if salt1 == salt2 {
		t.Fatal("expected unique salts for each hash")
	}
}

func TestVerifyLegacyBcryptOnly(t *testing.T) {
	legacy, err := HashPassword("password123", "")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword("password123", "", legacy) {
		t.Fatal("expected legacy bcrypt verification")
	}
}
