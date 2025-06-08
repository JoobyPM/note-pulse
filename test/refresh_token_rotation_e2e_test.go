//go:build e2e

package test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	signUpEndpoint  = "/api/v1/auth/sign-up"
	refreshEndpoint = "/api/v1/auth/refresh"
)

func TestRefreshTokenRotationE2E(t *testing.T) {
	env := SetupTestEnvironment(t)

	steps := []HTTPJSONStep{
		{
			Name:   "sign up a new user",
			Method: "POST",
			URL:    signUpEndpoint,
			Body: map[string]string{
				"email":    "rotation@test.com",
				"password": "Password123",
			},
			ExpectedStatus: http.StatusCreated,
			Validator:      AuthTokenValidator("refresh_token"),
		},
	}

	results := ExecuteHTTPJSONSteps(t, steps, env.BaseURL)
	initialRefreshToken := GetTokenFromResponse(t, results[0], "refresh_token")

	refreshSteps := []HTTPJSONStep{
		{
			Name:   "use refresh token to get new tokens (rotation should occur)",
			Method: "POST",
			URL:    refreshEndpoint,
			Body: map[string]string{
				"refresh_token": initialRefreshToken,
			},
			ExpectedStatus: http.StatusOK,
			Validator:      AuthTokenValidator("refresh_token"),
		},
	}

	refreshResults := ExecuteHTTPJSONSteps(t, refreshSteps, env.BaseURL)
	newRefreshToken := GetTokenFromResponse(t, refreshResults[0], "refresh_token")

	assert.NotEqual(t, initialRefreshToken, newRefreshToken, "Refresh token should have rotated")

	invalidationSteps := []HTTPJSONStep{
		{
			Name:   "try to use the OLD refresh token again",
			Method: "POST",
			URL:    refreshEndpoint,
			Body: map[string]string{
				"refresh_token": initialRefreshToken,
			},
			ExpectedStatus: http.StatusUnauthorized,
			Validator:      ErrorMessageValidator("Unauthorized"),
		},
		{
			Name:   "verify the NEW refresh token still works",
			Method: "POST",
			URL:    refreshEndpoint,
			Body: map[string]string{
				"refresh_token": newRefreshToken,
			},
			ExpectedStatus: http.StatusOK,
			Validator:      AuthTokenValidator("refresh_token"),
		},
	}

	ExecuteHTTPJSONSteps(t, invalidationSteps, env.BaseURL)
}
