package crypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHashPassword(t *testing.T) {
	password := "TestPassword123"
	cost := 12

	hash, err := HashPassword(password, cost)
	assert.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.NotEqual(t, password, hash)
}

func TestCheckPassword(t *testing.T) {
	password := "TestPassword123"
	cost := 12

	hash, err := HashPassword(password, cost)
	assert.NoError(t, err)

	err = CheckPassword(password, hash)
	assert.NoError(t, err, "correct password should pass")

	err = CheckPassword("WrongPassword", hash)
	assert.Error(t, err, "wrong password should fail")
}

func TestIsStrong(t *testing.T) {
	tests := []struct {
		name     string
		password string
		expected bool
	}{
		{"Valid password", "Password123", true},
		{"Too short", "Pass1", false},
		{"No uppercase", "password123", false},
		{"No lowercase", "PASSWORD123", false},
		{"No digit", "Password", false},
		{"Minimum valid", "Passw0rd", true},
		{"Long valid", "MyVeryLongPassword123", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsStrong(tt.password)
			assert.Equal(t, tt.expected, result)
		})
	}
}
