package test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"note-pulse/internal/config"
)

func TestConfig_LoadDefaults(t *testing.T) {
	// Clear any environment variables that might interfere
	clearConfigEnvVars(t)

	// Reset the cached config to ensure fresh load
	config.ResetCache()

	cfg, err := config.Load(context.Background())
	require.NoError(t, err)

	// Verify all defaults are loaded correctly
	assert.Equal(t, 8080, cfg.AppPort)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, "json", cfg.LogFormat)
	assert.Equal(t, "mongodb://mongo:27017", cfg.MongoURI)
	assert.Equal(t, "notepulse", cfg.MongoDBName)
	assert.Equal(t, "change-me", cfg.JWTSecret)
	assert.Equal(t, 60, cfg.JWTExpiryMinutes)
}

func TestConfig_LoadWithOverride(t *testing.T) {
	// Clear any environment variables that might interfere
	clearConfigEnvVars(t)

	// Reset the cached config to ensure fresh load
	config.ResetCache()

	// Set an override for APP_PORT
	err := os.Setenv("APP_PORT", "9999")
	require.NoError(t, err)
	defer func() {
		err := os.Unsetenv("APP_PORT")
		require.NoError(t, err)
	}()

	cfg, err := config.Load(context.Background())
	require.NoError(t, err)

	// Verify the override worked
	assert.Equal(t, 9999, cfg.AppPort)
	
	// Verify other defaults remain unchanged
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, "json", cfg.LogFormat)
	assert.Equal(t, "mongodb://mongo:27017", cfg.MongoURI)
	assert.Equal(t, "notepulse", cfg.MongoDBName)
	assert.Equal(t, "change-me", cfg.JWTSecret)
	assert.Equal(t, 60, cfg.JWTExpiryMinutes)
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  config.Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: config.Config{
				AppPort:          8080,
				LogLevel:         "info",
				LogFormat:        "json",
				MongoURI:         "mongodb://localhost:27017",
				MongoDBName:      "test",
				JWTSecret:        "secret",
				JWTExpiryMinutes: 60,
			},
			wantErr: false,
		},
		{
			name: "invalid port",
			config: config.Config{
				AppPort:          0,
				LogLevel:         "info",
				LogFormat:        "json",
				MongoURI:         "mongodb://localhost:27017",
				MongoDBName:      "test",
				JWTSecret:        "secret",
				JWTExpiryMinutes: 60,
			},
			wantErr: true,
			errMsg:  "APP_PORT must be greater than 0",
		},
		{
			name: "empty log level",
			config: config.Config{
				AppPort:          8080,
				LogLevel:         "",
				LogFormat:        "json",
				MongoURI:         "mongodb://localhost:27017",
				MongoDBName:      "test",
				JWTSecret:        "secret",
				JWTExpiryMinutes: 60,
			},
			wantErr: true,
			errMsg:  "LOG_LEVEL cannot be empty",
		},
		{
			name: "empty JWT secret",
			config: config.Config{
				AppPort:          8080,
				LogLevel:         "info",
				LogFormat:        "json",
				MongoURI:         "mongodb://localhost:27017",
				MongoDBName:      "test",
				JWTSecret:        "",
				JWTExpiryMinutes: 60,
			},
			wantErr: true,
			errMsg:  "JWT_SECRET cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_Caching(t *testing.T) {
	// Clear any environment variables that might interfere
	clearConfigEnvVars(t)

	// Reset the cached config to ensure fresh load
	config.ResetCache()

	// Load config first time
	cfg1, err := config.Load(context.Background())
	require.NoError(t, err)

	// Load config second time - should be cached
	cfg2, err := config.Load(context.Background())
	require.NoError(t, err)

	// Verify they are the same
	assert.Equal(t, cfg1, cfg2)
}

// Helper function to clear config-related environment variables
func clearConfigEnvVars(t *testing.T) {
	envVars := []string{
		"APP_PORT",
		"LOG_LEVEL", 
		"LOG_FORMAT",
		"MONGO_URI",
		"MONGO_DB_NAME",
		"JWT_SECRET",
		"JWT_EXPIRY_MINUTES",
	}

	for _, envVar := range envVars {
		if err := os.Unsetenv(envVar); err != nil {
			t.Logf("Warning: failed to unset %s: %v", envVar, err)
		}
	}
}

// Helper function to reset the cached config
// This is a test helper to ensure clean test state
func resetConfigCache() {
	// Note: This is accessing package-level variables for testing.
	// In a production environment, we'd need to implement a Reset() function
	// in the config package, but since the requirements don't ask for it,
	// we'll work around it by using a fresh process context for each test.
}
