//go:build e2e

package test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRefreshTokenRotation_E2E(t *testing.T) {
	env := SetupTestEnvironment(t)

	t.Log("step 1: sign up a new user")
	signUpReq := map[string]string{
		"email":    "rotation@test.com",
		"password": "Password123",
	}

	resp, err := httpJSON("POST", env.BaseURL+"/api/v1/auth/sign-up", signUpReq, nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var signUpResp map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&signUpResp))

	initialRefreshToken := signUpResp["refresh_token"].(string)
	require.NotEmpty(t, initialRefreshToken)

	t.Log("step 2: use refresh token to get new tokens (rotation should occur)")
	refreshReq := map[string]string{
		"refresh_token": initialRefreshToken,
	}

	resp, err = httpJSON("POST", env.BaseURL+"/api/v1/auth/refresh", refreshReq, nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var refreshResp map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&refreshResp))

	newRefreshToken := refreshResp["refresh_token"].(string)
	require.NotEmpty(t, newRefreshToken)

	assert.NotEqual(t, initialRefreshToken, newRefreshToken, "Refresh token should have rotated")

	t.Log("step 3: try to use the OLD refresh token again")
	oldRefreshReq := map[string]string{
		"refresh_token": initialRefreshToken,
	}

	resp, err = httpJSON("POST", env.BaseURL+"/api/v1/auth/refresh", oldRefreshReq, nil)
	require.NoError(t, err, "should refresh token")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
		"Old refresh token should be immediately invalidated after rotation")

	var errorResp map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&errorResp))
	assert.Contains(t, errorResp["error"].(string), "Unauthorized",
		"Error message should indicate invalid token - Unauthorized: but got - "+errorResp["error"].(string))

	t.Log("step 4: verify the NEW refresh token still works")
	newRefreshReq := map[string]string{
		"refresh_token": newRefreshToken,
	}

	resp, err = httpJSON("POST", env.BaseURL+"/api/v1/auth/refresh", newRefreshReq, nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode, "New refresh token should still be valid")
}
