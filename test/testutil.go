//go:build e2e

package test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// HTTPJSONStep represents a single HTTP JSON request step in a test
type HTTPJSONStep struct {
	Name           string
	Method         string
	URL            string
	Body           any
	Headers        map[string]string
	ExpectedStatus int
	Validator      func(*testing.T, map[string]any) // Optional response validator
}

// ExecuteHTTPJSONStep executes a single HTTP JSON step and handles all the common boilerplate
func ExecuteHTTPJSONStep(t *testing.T, step HTTPJSONStep, baseURL string) map[string]any {
	t.Helper()
	t.Logf("step: %s", step.Name)

	url := baseURL + step.URL
	resp, err := httpJSON(step.Method, url, step.Body, step.Headers)
	require.NoError(t, err)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("failed to close response body: %v", err)
		}
	}()

	assert.Equal(t, step.ExpectedStatus, resp.StatusCode)

	var respData map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&respData))

	if step.Validator != nil {
		step.Validator(t, respData)
	}

	return respData
}

// ExecuteHTTPJSONSteps executes a sequence of HTTP JSON steps
func ExecuteHTTPJSONSteps(t *testing.T, steps []HTTPJSONStep, baseURL string) []map[string]any {
	t.Helper()
	var results []map[string]any

	for _, step := range steps {
		result := ExecuteHTTPJSONStep(t, step, baseURL)
		results = append(results, result)
	}

	return results
}

// AuthTokenValidator validates that a response contains the expected auth tokens
func AuthTokenValidator(expectedFields ...string) func(*testing.T, map[string]any) {
	return func(t *testing.T, respData map[string]any) {
		t.Helper()
		for _, field := range expectedFields {
			value, exists := respData[field]
			require.True(t, exists, "Expected field %s to exist in response", field)
			require.NotEmpty(t, value, "Expected field %s to not be empty", field)
		}
	}
}

// ErrorMessageValidator validates that an error response contains expected message content
func ErrorMessageValidator(expectedSubstring string) func(*testing.T, map[string]any) {
	return func(t *testing.T, respData map[string]any) {
		t.Helper()
		errorMsg, exists := respData["error"]
		require.True(t, exists, "Expected error field to exist in response")
		assert.Contains(t, errorMsg.(string), expectedSubstring,
			"Expected error message to contain '%s', but got: %s", expectedSubstring, errorMsg)
	}
}

// TokenNotEqualValidator validates that two tokens are different (for rotation testing)
func TokenNotEqualValidator(token1, token2 any, message string) func(*testing.T, map[string]any) {
	return func(t *testing.T, respData map[string]any) {
		t.Helper()
		assert.NotEqual(t, token1, token2, message)
	}
}

// MessageValidator validates that a response contains a specific message
func MessageValidator(expectedMessage string) func(*testing.T, map[string]any) {
	return func(t *testing.T, respData map[string]any) {
		t.Helper()
		message, exists := respData["message"]
		require.True(t, exists, "Expected message field to exist in response")
		assert.Equal(t, expectedMessage, message)
	}
}

// GetTokenFromResponse safely extracts a token field from response data
func GetTokenFromResponse(t *testing.T, respData map[string]any, fieldName string) string {
	t.Helper()
	token, exists := respData[fieldName]
	require.True(t, exists, "Expected %s field to exist in response", fieldName)
	tokenStr, ok := token.(string)
	require.True(t, ok, "Expected %s to be a string", fieldName)
	require.NotEmpty(t, tokenStr, "Expected %s to not be empty", fieldName)
	return tokenStr
}
