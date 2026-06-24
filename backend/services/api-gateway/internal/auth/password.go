// Package auth implements credential handling: bcrypt password hashing,
// JWT access tokens, and refresh-token rotation with reuse detection.
// Design rationale in docs/adr/0007-auth-design.md.
package auth

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// bcryptCost trades hash time for brute-force resistance; 12 is the current
// sensible default (~250ms on commodity hardware).
const bcryptCost = 12

// bcrypt silently truncates inputs beyond 72 bytes; reject instead.
const maxPasswordBytes = 72

// ErrPasswordTooLong rejects passwords beyond bcrypt's input limit.
var ErrPasswordTooLong = errors.New("auth: password exceeds 72 bytes")

// HashPassword derives a bcrypt hash for storage.
func HashPassword(password string) (string, error) {
	if len(password) > maxPasswordBytes {
		return "", ErrPasswordTooLong
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("auth: hash password: %w", err)
	}
	return string(hash), nil
}

// VerifyPassword reports whether password matches the stored hash.
func VerifyPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
