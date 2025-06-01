package config

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_LoadDefaults(t *testing.T) {
	clearConfigEnvVars(t)
	ResetCache()

	cfg, err := Load(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 8080, cfg.AppPort)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, "json", cfg.LogFormat)
	assert.Equal(t, "mongodb://mongo:27017", cfg.MongoURI)
	assert.Equal(t, "notepulse", cfg.MongoDBName)
	assert.Equal(t, "change-me", cfg.JWTSecret)
	assert.Equal(t, 60, cfg.JWTExpiryMinutes)
}

func TestConfig_LoadWithOverride(t *testing.T) {
	clearConfigEnvVars(t)
	ResetCache()

	err := os.Setenv("APP_PORT", "9999")
	require.NoError(t, err)
	defer func() {
		err := os.Unsetenv("APP_PORT")
		require.NoError(t, err)
	}()

	cfg, err := Load(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 9999, cfg.AppPort)
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
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: Config{
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
			config: Config{
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
			config: Config{
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
			config: Config{
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
	clearConfigEnvVars(t)
	ResetCache()

	cfg1, err := Load(context.Background())
	require.NoError(t, err)

	cfg2, err := Load(context.Background())
	require.NoError(t, err)

	assert.Equal(t, cfg1, cfg2)
}

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
