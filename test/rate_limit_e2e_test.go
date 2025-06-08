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

func TestRateLimitE2E(t *testing.T) {
	extraEnv := map[string]string{
		"SIGNIN_RATE_PER_MIN": fmt.Sprint(maxPerMinute),
	}

	env1 := SetupTestEnvironmentWithEnv(t, extraEnv)

	t.Run("setup_user", func(t *testing.T) {
		signUp(t, env1.Client, env1.BaseURL, testEmail, testPassword)
	})

	t.Run("rate_limit_sign_in", func(t *testing.T) {
		for i := 0; i < maxPerMinute; i++ {
			signInExpect(t, env1.Client, env1.BaseURL, testEmail, testPassword, http.StatusOK)
		}
		signInExpect(t, env1.Client, env1.BaseURL, testEmail, testPassword, http.StatusTooManyRequests)
	})

	t.Run("rate_limit_reset", func(t *testing.T) {
		env2 := SetupTestEnvironmentWithEnv(t, extraEnv)

		// Use a different email for the second environment to avoid conflicts
		resetTestEmail := "ratelimit-reset@example.com"
		signUp(t, env2.Client, env2.BaseURL, resetTestEmail, testPassword)
		signInExpect(t, env2.Client, env2.BaseURL, resetTestEmail, testPassword, http.StatusOK)
	})
}
