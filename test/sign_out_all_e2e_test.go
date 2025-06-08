//go:build e2e

package test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSignOutAll_E2E(t *testing.T) {
	env := SetupTestEnvironment(t)

	t.Log("step 1: sign up a user")
	signUpReq := map[string]string{
		"email":    "signoutall@test.com",
		"password": "Password123",
	}

	resp, err := httpJSON("POST", env.BaseURL+"/api/v1/auth/sign-up", signUpReq, nil)
	require.NoError(t, err)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("failed to close response body: %v", err)
		}
	}()
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var signUpResp map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&signUpResp))

	accessToken1 := signUpResp["token"].(string)
	refreshToken1 := signUpResp["refresh_token"].(string)
	require.NotEmpty(t, accessToken1)
	require.NotEmpty(t, refreshToken1)

	t.Log("step 2: sign in again to get a second set of tokens (simulating another device)")
	signInReq := map[string]string{
		"email":    "signoutall@test.com",
		"password": "Password123",
	}

	resp, err = httpJSON("POST", env.BaseURL+"/api/v1/auth/sign-in", signInReq, nil)
	require.NoError(t, err)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("failed to close response body: %v", err)
		}
	}()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var signInResp map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&signInResp))

	accessToken2 := signInResp["token"].(string)
	refreshToken2 := signInResp["refresh_token"].(string)
	require.NotEmpty(t, accessToken2)
	require.NotEmpty(t, refreshToken2)

	t.Log("step 3: verify we have different refresh tokens")
	assert.NotEqual(t, refreshToken1, refreshToken2, "should have different refresh tokens")

	t.Log("step 4: verify both refresh tokens work")
	refreshReq1 := map[string]string{
		"refresh_token": refreshToken1,
	}
	resp, err = httpJSON("POST", env.BaseURL+"/api/v1/auth/refresh", refreshReq1, nil)
	require.NoError(t, err)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("failed to close response body: %v", err)
		}
	}()
	assert.Equal(t, http.StatusOK, resp.StatusCode, "First refresh token should work")

	refreshReq2 := map[string]string{
		"refresh_token": refreshToken2,
	}
	resp, err = httpJSON("POST", env.BaseURL+"/api/v1/auth/refresh", refreshReq2, nil)
	require.NoError(t, err)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("failed to close response body: %v", err)
		}
	}()
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Second refresh token should work")

	t.Log("step 5: call sign-out-all using one of the access tokens")
	headers := map[string]string{
		"Authorization": "Bearer " + accessToken1,
	}

	resp, err = httpJSON("POST", env.BaseURL+"/api/v1/auth/sign-out-all", nil, headers)
	require.NoError(t, err)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("failed to close response body: %v", err)
		}
	}()
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Sign-out-all should succeed")

	var signOutResp map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&signOutResp))
	assert.Equal(t, "Signed out everywhere", signOutResp["message"])

	t.Log("step 6: verify both refresh tokens are now invalid")
	resp, err = httpJSON("POST", env.BaseURL+"/api/v1/auth/refresh", refreshReq1, nil)
	require.NoError(t, err)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("failed to close response body: %v", err)
		}
	}()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
		"First refresh token should be invalid after sign-out-all")

	resp, err = httpJSON("POST", env.BaseURL+"/api/v1/auth/refresh", refreshReq2, nil)
	require.NoError(t, err)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("failed to close response body: %v", err)
		}
	}()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
		"Second refresh token should be invalid after sign-out-all")
}
