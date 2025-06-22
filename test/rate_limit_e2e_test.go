//go:build e2e

package test

import (
	"fmt"
	"net/http"
	"testing"
)

const (
	testEmail    = "ratelimit@example.com"
	testPassword = "Passw0rd123"
	maxPerMinute = 3 // small quota so we hit 429 quickly
)

func getTestingUrl(env *TestEnvironment) string {
	return env.BaseURL + notesPath + "?" + limitParam + "1"
}

func TestRateLimitE2E(t *testing.T) {
	extraEnv := map[string]string{
		authRateLimitEnv: fmt.Sprint(maxPerMinute),
	}

	env1 := SetupTestEnvironmentWithEnv(t, extraEnv)

	t.Run("setup_user", func(t *testing.T) {
		signUp(t, env1.Client, env1.BaseURL, testEmail, testPassword)
	})

	t.Run("rate_limit_sign_in", func(t *testing.T) {
		for i := 0; i < maxPerMinute-1; i++ {
			signInExpect(t, env1.Client, env1.BaseURL, testEmail, testPassword, http.StatusOK)
		}
		signInExpectTooManyRequests(t, env1.Client, env1.BaseURL, testEmail, testPassword)
	})

	t.Run("rate_limit_reset", func(t *testing.T) {
		env2 := SetupTestEnvironmentWithEnv(t, extraEnv)

		// Use a different email for the second environment to avoid conflicts
		resetTestEmail := "ratelimit-reset@example.com"
		signUp(t, env2.Client, env2.BaseURL, resetTestEmail, testPassword)
		signInExpect(t, env2.Client, env2.BaseURL, resetTestEmail, testPassword, http.StatusOK)
	})
}

func TestV1RateLimitDefaultOffE2E(t *testing.T) {
	env := SetupTestEnvironment(t)

	token := setupTestUser(t, env, "v1-default-off@example.com", "Password123")
	h := getAuthHeaders(t, token)

	createAndVerifyNote(t, env, h, NoteParams{
		Title: "app-rate-limit-default-off",
		Body:  "Won't match search",
		Color: testColor,
	})

	for range 10 { // far above typical defaults
		makeHTTPRequest(t, "GET", getTestingUrl(env), nil, h, http.StatusOK)
	}

	makeHTTPRequest(t, "GET", getTestingUrl(env), nil, h, http.StatusOK)
}

func TestV1RateLimitEnabledE2E(t *testing.T) {
	extraEnv := map[string]string{
		appRateLimitEnv: fmt.Sprint(maxPerMinute),
	}

	env := SetupTestEnvironmentWithEnv(t, extraEnv)

	token := setupTestUser(t, env, "app-rate-limit-enabled@example.com", "Password123")
	h := getAuthHeaders(t, token)

	for range maxPerMinute {
		makeHTTPRequest(t, "GET", getTestingUrl(env), nil, h, http.StatusOK)
	}

	makeHTTPRequest(t, "GET", getTestingUrl(env), nil, h, http.StatusTooManyRequests)
}
