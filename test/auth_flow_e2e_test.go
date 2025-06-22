//go:build e2e

package test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthFlowE2E(t *testing.T) {
	env := SetupTestEnvironment(t)

	testEmail := "bob@example.com"
	testPassword := "Password123"

	t.Run("sign_up", func(t *testing.T) {
		signUpPayload := map[string]string{
			"email":    testEmail,
			"password": testPassword,
		}

		resp, err := httpJSON("POST", env.BaseURL+signUpEndpoint, signUpPayload, nil)
		require.NoError(t, err)
		defer func() {
			if err := resp.Body.Close(); err != nil {
				t.Errorf(msgFailedToCloseResponseBody, err)
			}
		}()

		require.Equal(t, http.StatusCreated, resp.StatusCode)

		var signUpResp map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&signUpResp))

		assert.Contains(t, signUpResp, "user", "user should be present")
		assert.Contains(t, signUpResp, "token", "token should be present")

		user := signUpResp["user"].(map[string]any)
		assert.Equal(t, testEmail, user["email"])
		assert.Contains(t, user, "id")
		assert.NotEmpty(t, signUpResp["token"])
	})

	var authToken string
	t.Run("sign_in", func(t *testing.T) {
		signInPayload := map[string]string{
			"email":    testEmail,
			"password": testPassword,
		}

		resp, err := httpJSON("POST", env.BaseURL+signInEndpoint, signInPayload, nil)
		require.NoError(t, err)
		defer func() {
			if err := resp.Body.Close(); err != nil {
				t.Errorf(msgFailedToCloseResponseBody, err)
			}
		}()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var signInResp map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&signInResp))

		assert.Contains(t, signInResp, "user", "user should be present")
		assert.Contains(t, signInResp, "token", "token should be present")

		user := signInResp["user"].(map[string]any)
		assert.Equal(t, testEmail, user["email"])
		assert.Contains(t, user, "id")

		token, ok := signInResp["token"].(string)
		require.True(t, ok, "token should be a string")
		require.NotEmpty(t, token, "token should not be empty")
		authToken = token
	})

	t.Run("me_endpoint", func(t *testing.T) {
		headers := map[string]string{
			"Authorization": "Bearer " + authToken,
		}

		resp, err := httpJSON("GET", env.BaseURL+meEndpoint, nil, headers)
		require.NoError(t, err)
		defer func() {
			if err := resp.Body.Close(); err != nil {
				t.Errorf(msgFailedToCloseResponseBody, err)
			}
		}()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var meResp map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&meResp))

		assert.Contains(t, meResp, "uid")
		assert.Contains(t, meResp, "email")
		assert.Equal(t, testEmail, meResp["email"])
		assert.NotEmpty(t, meResp["uid"])
	})
}
