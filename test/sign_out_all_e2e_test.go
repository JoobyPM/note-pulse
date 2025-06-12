//go:build e2e

package test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

var testCredentials = map[string]string{
	"email":    "signoutall@test.com",
	"password": "Password123",
}

func TestSignOutAllE2E(t *testing.T) {
	extraEnv := map[string]string{
		"AUTH_RATE_PER_MIN": "1000",
	}

	env := SetupTestEnvironmentWithEnv(t, extraEnv)

	authSteps := []HTTPJSONStep{
		{
			Name:           "sign up a user",
			Method:         "POST",
			URL:            signUpEndpoint,
			Body:           testCredentials,
			ExpectedStatus: http.StatusCreated,
			Validator:      AuthTokenValidator("token", "refresh_token"),
		},
		{
			Name:           "sign in again to get a second set of tokens (simulating another device)",
			Method:         "POST",
			URL:            signInEndpoint,
			Body:           testCredentials,
			ExpectedStatus: http.StatusOK,
			Validator:      AuthTokenValidator("token", "refresh_token"),
		},
	}

	results := ExecuteHTTPJSONSteps(t, authSteps, env.BaseURL)

	accessToken1 := GetTokenFromResponse(t, results[0], "token")
	refreshToken1 := GetTokenFromResponse(t, results[0], "refresh_token")
	refreshToken2 := GetTokenFromResponse(t, results[1], "refresh_token")

	assert.NotEqual(t, refreshToken1, refreshToken2, "should have different refresh tokens")

	verificationSteps := []HTTPJSONStep{
		{
			Name:   "verify first refresh token works",
			Method: "POST",
			URL:    refreshEndpoint,
			Body: map[string]string{
				"refresh_token": refreshToken1,
			},
			ExpectedStatus: http.StatusOK,
		},
		{
			Name:   "verify second refresh token works",
			Method: "POST",
			URL:    refreshEndpoint,
			Body: map[string]string{
				"refresh_token": refreshToken2,
			},
			ExpectedStatus: http.StatusOK,
		},
		{
			Name:   "call sign-out-all using one of the access tokens",
			Method: "POST",
			URL:    signOutAllEndpoint,
			Body:   nil,
			Headers: map[string]string{
				"Authorization": "Bearer " + accessToken1,
			},
			ExpectedStatus: http.StatusOK,
			Validator:      MessageValidator("Signed out everywhere"),
		},
	}

	ExecuteHTTPJSONSteps(t, verificationSteps, env.BaseURL)

	invalidationSteps := []HTTPJSONStep{
		{
			Name:   "verify first refresh token is now invalid",
			Method: "POST",
			URL:    refreshEndpoint,
			Body: map[string]string{
				"refresh_token": refreshToken1,
			},
			ExpectedStatus: http.StatusUnauthorized,
		},
		{
			Name:   "verify second refresh token is now invalid",
			Method: "POST",
			URL:    refreshEndpoint,
			Body: map[string]string{
				"refresh_token": refreshToken2,
			},
			ExpectedStatus: http.StatusUnauthorized,
		},
	}

	ExecuteHTTPJSONSteps(t, invalidationSteps, env.BaseURL)
}
