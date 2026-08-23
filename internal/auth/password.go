package auth

import (
	"errors"
	"fmt"
	"golang.org/x/crypto/bcrypt"
)

const (
	// DefaultBcryptCost provides high resistance to offline brute-force attacks while maintaining fast response times.
	DefaultBcryptCost = 12
)

var (
	ErrPasswordTooShort = errors.New("password must be at least 8 characters long")
	ErrPasswordTooLong  = errors.New("password must not exceed 72 bytes (bcrypt limitation)")
	ErrInvalidPassword  = errors.New("invalid username or password")
)

// HashPassword generates a bcrypt hash of the provided plaintext password.
func HashPassword(password string) (string, error) {
	if len(password) < 8 {
		return "", ErrPasswordTooShort
	}
	if len(password) > 72 {
		return "", ErrPasswordTooLong
	}

	bytes, err := bcrypt.GenerateFromPassword([]byte(password), DefaultBcryptCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(bytes), nil
}

// CheckPassword verifies that a plaintext password matches the stored bcrypt hash.
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
