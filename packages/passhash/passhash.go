package passhash

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

const saltBytes = 32

// GenerateSalt returns a cryptographically random salt encoded as base64.
func GenerateSalt() (string, error) {
	b := make([]byte, saltBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	return base64.RawStdEncoding.EncodeToString(b), nil
}

// HashPassword hashes password combined with the per-user salt using bcrypt.
func HashPassword(password, salt string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(salt+password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

// HashPasswordPair generates a new salt and returns salt + hash for storage.
func HashPasswordPair(password string) (salt, hash string, err error) {
	salt, err = GenerateSalt()
	if err != nil {
		return "", "", err
	}
	hash, err = HashPassword(password, salt)
	if err != nil {
		return "", "", err
	}
	return salt, hash, nil
}

// VerifyPassword checks a password against stored hash and salt.
// When salt is empty, legacy bcrypt-only hashes (password without extra salt) are accepted.
func VerifyPassword(password, salt, hash string) bool {
	if salt == "" {
		return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(salt+password)) == nil
}
