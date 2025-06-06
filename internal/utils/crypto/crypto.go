package crypto

import (
	"regexp"

	"golang.org/x/crypto/bcrypt"
)

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

// TODO: [pref] easy win - «`crypto.IsStrong` compiles three regexes per call; pre-compile them at package init.»
// IsStrong checks if a password meets minimum strength requirements
// Requirements: ≥8 chars, 1 upper, 1 lower, 1 digit
func IsStrong(password string) bool {
	if len(password) < 8 {
		return false
	}

	hasUpper := regexp.MustCompile(`[A-Z]`).MatchString(password)
	hasLower := regexp.MustCompile(`[a-z]`).MatchString(password)
	hasDigit := regexp.MustCompile(`[0-9]`).MatchString(password)

	return hasUpper && hasLower && hasDigit
}
