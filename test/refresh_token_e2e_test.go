//go:build e2e

package test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"note-pulse/internal/services/auth"
)

func TestRefreshTokenFlow_E2E(t *testing.T) {
	env := SetupTestEnvironment(t)

	email := "refresh-test@example.com"
	password := "TestPassword123"

	t.Log("Step 1: Sign up a new user")
	signupBody := map[string]string{
		"email":    email,
		"password": password,
	}
	signupResp, err := httpJSON("POST", env.BaseURL+"/api/v1/auth/sign-up", signupBody, nil)
	require.NoError(t, err, "should sign up")
	defer signupResp.Body.Close()
	require.Equal(t, http.StatusCreated, signupResp.StatusCode, "should sign up")

	var signupResult auth.AuthResponse
	err = json.NewDecoder(signupResp.Body).Decode(&signupResult)
	require.NoError(t, err, "should decode signup response")
	require.NotEmpty(t, signupResult.Token, "should have token")
	require.NotEmpty(t, signupResult.RefreshToken, "should have refresh token")

	originalAccessToken := signupResult.Token
	refreshToken := signupResult.RefreshToken

	t.Log("Step 2: Use refresh token to get new access token")
	refreshBody := map[string]string{
		"refresh_token": refreshToken,
	}
	refreshResp, err := httpJSON("POST", env.BaseURL+"/api/v1/auth/refresh", refreshBody, nil)
	require.NoError(t, err, "should refresh")
	defer refreshResp.Body.Close()
	require.Equal(t, http.StatusOK, refreshResp.StatusCode, "should refresh")

	var refreshResult auth.AuthResponse
	err = json.NewDecoder(refreshResp.Body).Decode(&refreshResult)
	require.NoError(t, err, "should decode refresh response")
	require.NotEmpty(t, refreshResult.Token, "should have token")
	require.NotEmpty(t, refreshResult.RefreshToken, "should have refresh token")

	assert.NotEqual(t, originalAccessToken, refreshResult.Token, "should have new token")
	assert.NotEqual(t, refreshToken, refreshResult.RefreshToken, "should have new refresh token")

	newAccessToken := refreshResult.Token
	newRefreshToken := refreshResult.RefreshToken

	t.Log("Step 3: Use new access token to access protected route")
	headers := map[string]string{
		"Authorization": "Bearer " + newAccessToken,
	}
	profileResp, err := httpJSON("GET", env.BaseURL+"/api/v1/me", nil, headers)
	require.NoError(t, err, "should get profile")
	defer profileResp.Body.Close()
	require.Equal(t, http.StatusOK, profileResp.StatusCode, "should get profile")

	var profileResult map[string]interface{}
	err = json.NewDecoder(profileResp.Body).Decode(&profileResult)
	require.NoError(t, err, "should decode profile response")
	assert.Equal(t, email, profileResult["email"], "should have email")

	t.Log("Step 4: Sign out with refresh token")
	signoutBody := map[string]string{
		"refresh_token": newRefreshToken,
	}
	signoutResp, err := httpJSON("POST", env.BaseURL+"/api/v1/auth/sign-out", signoutBody, headers)
	require.NoError(t, err, "should sign out")
	defer signoutResp.Body.Close()
	require.Equal(t, http.StatusOK, signoutResp.StatusCode, "should sign out")

	var signoutResult map[string]string
	err = json.NewDecoder(signoutResp.Body).Decode(&signoutResult)
	require.NoError(t, err, "should decode signout response")
	assert.Equal(t, "Successfully signed out", signoutResult["message"], "should have message")

	t.Log("Step 5: Try to use the refresh token again (should fail)")
	refreshResp2, err := httpJSON("POST", env.BaseURL+"/api/v1/auth/refresh", refreshBody, nil)
	require.NoError(t, err, "should refresh")
	defer refreshResp2.Body.Close()
	require.Equal(t, http.StatusUnauthorized, refreshResp2.StatusCode, "should be unauthorized")

	t.Log("Step 6: Try to use new refresh token (should also fail since it was revoked)")
	refreshBody2 := map[string]string{
		"refresh_token": newRefreshToken,
	}
	refreshResp3, err := httpJSON("POST", env.BaseURL+"/api/v1/auth/refresh", refreshBody2, nil)
	require.NoError(t, err, "should refresh")
	defer refreshResp3.Body.Close()
	require.Equal(t, http.StatusUnauthorized, refreshResp3.StatusCode, "should be unauthorized")
}

func TestRefreshTokenExpiry_E2E(t *testing.T) {
	// This test would require manipulating time or using very short-lived tokens
	// For now, we'll test with invalid refresh tokens
	env := SetupTestEnvironment(t)

	// Test with completely invalid refresh token
	refreshBody := map[string]string{
		"refresh_token": "invalid-refresh-token",
	}
	refreshResp, err := httpJSON("POST", env.BaseURL+"/api/v1/auth/refresh", refreshBody, nil)
	require.NoError(t, err)
	defer refreshResp.Body.Close()
	require.Equal(t, http.StatusUnauthorized, refreshResp.StatusCode)
}
