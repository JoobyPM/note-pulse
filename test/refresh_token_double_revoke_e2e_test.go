//go:build e2e

package test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRefreshTokenDoubleRevoke_E2E verifies that the same refresh token cannot
// be used twice in /auth/sign-out.  The first call must succeed; the second one must be rejected (401 Unauthorized).
func TestRefreshTokenDoubleRevoke_E2E(t *testing.T) {
	env := SetupTestEnvironment(t)

	const (
		email    = "double-revoke@test.com"
		password = "Password123"
	)

	t.Log("Step 1: Sign up a new user")
	signUpBody := map[string]string{
		"email":    email,
		"password": password,
	}
	resp, err := httpJSON("POST", env.BaseURL+"/api/v1/auth/sign-up", signUpBody, nil)
	require.NoError(t, err)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("failed to close response body: %v", err)
		}
	}()
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var signUpResp map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&signUpResp))

	accessToken := signUpResp["token"].(string)
	refreshToken := signUpResp["refresh_token"].(string)
	require.NotEmpty(t, accessToken)
	require.NotEmpty(t, refreshToken)

	authHdr := map[string]string{
		"Authorization": "Bearer " + accessToken,
	}

	t.Log("Step 2: First /auth/sign-out (should succeed)")
	body := map[string]string{"refresh_token": refreshToken}
	signOut1, err := httpJSON("POST", env.BaseURL+"/api/v1/auth/sign-out", body, authHdr)
	require.NoError(t, err)
	defer signOut1.Body.Close()
	require.Equal(t, http.StatusOK, signOut1.StatusCode)

	t.Log("Step 3: Second /auth/sign-out with the *same* token (MUST fail)")
	signOut2, err := httpJSON("POST", env.BaseURL+"/api/v1/auth/sign-out", body, authHdr)
	require.NoError(t, err)
	defer signOut2.Body.Close()

	t.Log("Step 4: Second /auth/sign-out with the *same* token (MUST fail)")
	require.Equalf(
		t,
		http.StatusUnauthorized,
		signOut2.StatusCode,
		"second sign-out with an already-revoked refresh-token should be rejected",
	)
}
