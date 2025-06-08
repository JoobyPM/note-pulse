package crypto

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/go-playground/validator/v10"
	"golang.org/x/crypto/bcrypt"
)

// Pre-compiled regexes for password strength validation
var (
	reUpper = regexp.MustCompile(`[A-Z]`)
	reLower = regexp.MustCompile(`[a-z]`)
	reDigit = regexp.MustCompile(`[0-9]`)

	// ErrPasswordWeak is returned when a supplied password does not satisfy
	// the minimum strength rules (≥8 chars, 1 upper, 1 lower, 1 digit).
	ErrPasswordWeak = errors.New("password is too weak")
)

type passwordErrKey struct{}

// PasswordErrKey is the single global key the password rule
// will use to drop ErrPasswordWeak into the ctx.
var PasswordErrKey passwordErrKey

// HashPassword hashes a password using bcrypt with the given cost
func HashPassword(password string, cost int) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword verifies a password against its hash
func CheckPassword(password, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

// IsStrong checks if a password meets minimum strength requirements
// Requirements: ≥8 chars, 1 upper, 1 lower, 1 digit
func IsStrong(password string) bool {
	if len(password) < 8 {
		return false
	}

	hasUpper := reUpper.MatchString(password)
	hasLower := reLower.MatchString(password)
	hasDigit := reDigit.MatchString(password)

	return hasUpper && hasLower && hasDigit
}

// RegisterPasswordValidator registers the "password" validation tag with the validator
func RegisterPasswordValidator(v *validator.Validate) error {
	err := v.RegisterValidationCtx("password",
		func(ctx context.Context, fl validator.FieldLevel) bool {
			pwd := fl.Field().String()

			if IsStrong(pwd) {
				return true
			}

			if bucket, ok := ctx.Value(PasswordErrKey).(*error); ok && bucket != nil {
				*bucket = ErrPasswordWeak
			}
			return false
		})
	if err != nil {
		return fmt.Errorf("failed to register password validator: %w", err)
	}
	return nil
}
