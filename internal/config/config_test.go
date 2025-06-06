package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_LoadDefaults(t *testing.T) {
	t.Parallel()
	clearConfigEnvVars(t)
	ResetCache()

	// Set DEV_MODE=true to bypass JWT_SECRET requirement for tests
	err := os.Setenv("DEV_MODE", "true")
	require.NoError(t, err)
	defer func() {
		err := os.Unsetenv("DEV_MODE")
		require.NoError(t, err)
	}()

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, 8080, cfg.AppPort)
	assert.Equal(t, 8, cfg.BcryptCost)
	assert.Equal(t, 5, cfg.SignInRatePerMin)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, "json", cfg.LogFormat)
	assert.Equal(t, "mongodb://mongo:27017", cfg.MongoURI)
	assert.Equal(t, "notepulse", cfg.MongoDBName)
	assert.Equal(t, "", cfg.JWTSecret) // No default JWT secret anymore
	assert.Equal(t, "HS256", cfg.JWTAlgorithm)
	assert.Equal(t, true, cfg.DevMode)
	assert.Equal(t, 900, cfg.WSMaxSessionSec)
	assert.Equal(t, 256, cfg.WSOutboxBuffer)
	assert.Equal(t, true, cfg.RequestLoggingEnabled)
}

func TestConfig_LoadWithOverride(t *testing.T) {
	t.Parallel()
	clearConfigEnvVars(t)
	ResetCache()

	err := os.Setenv("APP_PORT", "9999")
	require.NoError(t, err)
	defer func() {
		err := os.Unsetenv("APP_PORT")
		require.NoError(t, err)
	}()

	// Set DEV_MODE=true to bypass JWT_SECRET requirement for tests
	err = os.Setenv("DEV_MODE", "true")
	require.NoError(t, err)
	defer func() {
		err := os.Unsetenv("DEV_MODE")
		require.NoError(t, err)
	}()

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, 9999, cfg.AppPort)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, "json", cfg.LogFormat)
	assert.Equal(t, "mongodb://mongo:27017", cfg.MongoURI)
	assert.Equal(t, "notepulse", cfg.MongoDBName)
	assert.Equal(t, "", cfg.JWTSecret) // No default JWT secret anymore
	assert.Equal(t, true, cfg.DevMode)
}

func TestConfig_Validate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: Config{
				AppPort:            8080,
				BcryptCost:         12,
				SignInRatePerMin:   5,
				LogLevel:           "info",
				LogFormat:          "json",
				MongoURI:           "mongodb://localhost:27017",
				MongoDBName:        "test",
				JWTSecret:          "this-is-a-super-secret-jwt-key-with-32-plus-chars",
				JWTAlgorithm:       "HS256",
				AccessTokenMinutes: 15,
				RefreshTokenDays:   30,
				RefreshTokenRotate: true,
				WSMaxSessionSec:    900,
				WSOutboxBuffer:     256,
			},
			wantErr: false,
		},
		{
			name: "invalid port - zero",
			config: Config{
				AppPort:            0,
				BcryptCost:         12,
				SignInRatePerMin:   5,
				LogLevel:           "info",
				LogFormat:          "json",
				MongoURI:           "mongodb://localhost:27017",
				MongoDBName:        "test",
				JWTSecret:          "this-is-a-super-secret-jwt-key-with-32-plus-chars",
				JWTAlgorithm:       "HS256",
				AccessTokenMinutes: 15,
				RefreshTokenDays:   30,
				RefreshTokenRotate: true,
				WSMaxSessionSec:    900,
				WSOutboxBuffer:     256,
			},
			wantErr: true,
			errMsg:  "APP_PORT must be between 1 and 65535",
		},
		{
			name: "invalid port - negative",
			config: Config{
				AppPort:            -1,
				BcryptCost:         12,
				SignInRatePerMin:   5,
				LogLevel:           "info",
				LogFormat:          "json",
				MongoURI:           "mongodb://localhost:27017",
				MongoDBName:        "test",
				JWTSecret:          "this-is-a-super-secret-jwt-key-with-32-plus-chars",
				JWTAlgorithm:       "HS256",
				AccessTokenMinutes: 15,
				RefreshTokenDays:   30,
				RefreshTokenRotate: true,
				WSMaxSessionSec:    900,
				WSOutboxBuffer:     256,
			},
			wantErr: true,
			errMsg:  "APP_PORT must be between 1 and 65535",
		},
		{
			name: "invalid port - too high",
			config: Config{
				AppPort:            70000,
				BcryptCost:         12,
				SignInRatePerMin:   5,
				LogLevel:           "info",
				LogFormat:          "json",
				MongoURI:           "mongodb://localhost:27017",
				MongoDBName:        "test",
				JWTSecret:          "this-is-a-super-secret-jwt-key-with-32-plus-chars",
				JWTAlgorithm:       "HS256",
				AccessTokenMinutes: 15,
				RefreshTokenDays:   30,
				RefreshTokenRotate: true,
				WSMaxSessionSec:    900,
				WSOutboxBuffer:     256,
			},
			wantErr: true,
			errMsg:  "APP_PORT must be between 1 and 65535",
		},
		{
			name: "empty log level",
			config: Config{
				AppPort:            8080,
				BcryptCost:         12,
				SignInRatePerMin:   5,
				LogLevel:           "",
				LogFormat:          "json",
				MongoURI:           "mongodb://localhost:27017",
				MongoDBName:        "test",
				JWTSecret:          "this-is-a-super-secret-jwt-key-with-32-plus-chars",
				JWTAlgorithm:       "HS256",
				AccessTokenMinutes: 15,
				RefreshTokenDays:   30,
				RefreshTokenRotate: true,
				WSMaxSessionSec:    900,
				WSOutboxBuffer:     256,
			},
			wantErr: true,
			errMsg:  "LOG_LEVEL cannot be empty",
		},
		{
			name: "empty JWT secret",
			config: Config{
				AppPort:            8080,
				BcryptCost:         12,
				SignInRatePerMin:   5,
				LogLevel:           "info",
				LogFormat:          "json",
				MongoURI:           "mongodb://localhost:27017",
				MongoDBName:        "test",
				JWTSecret:          "",
				JWTAlgorithm:       "HS256",
				AccessTokenMinutes: 15,
				RefreshTokenDays:   30,
				RefreshTokenRotate: true,
				WSMaxSessionSec:    900,
				WSOutboxBuffer:     256,
				DevMode:            false,
			},
			wantErr: true,
			errMsg:  "JWT_SECRET is required (see .env.template)",
		},
		{
			name: "bcrypt cost too low",
			config: Config{
				AppPort:            8080,
				BcryptCost:         7,
				SignInRatePerMin:   5,
				LogLevel:           "info",
				LogFormat:          "json",
				MongoURI:           "mongodb://localhost:27017",
				MongoDBName:        "test",
				JWTSecret:          "this-is-a-super-secret-jwt-key-with-32-plus-chars",
				JWTAlgorithm:       "HS256",
				AccessTokenMinutes: 15,
				RefreshTokenDays:   30,
				RefreshTokenRotate: true,
				WSMaxSessionSec:    900,
				WSOutboxBuffer:     256,
			},
			wantErr: true,
			errMsg:  "BCRYPT_COST must be between 8 and 16",
		},
		{
			name: "bcrypt cost too high",
			config: Config{
				AppPort:            8080,
				BcryptCost:         17,
				SignInRatePerMin:   5,
				LogLevel:           "info",
				LogFormat:          "json",
				MongoURI:           "mongodb://localhost:27017",
				MongoDBName:        "test",
				JWTSecret:          "this-is-a-super-secret-jwt-key-with-32-plus-chars",
				JWTAlgorithm:       "HS256",
				AccessTokenMinutes: 15,
				RefreshTokenDays:   30,
				RefreshTokenRotate: true,
				WSMaxSessionSec:    900,
				WSOutboxBuffer:     256,
			},
			wantErr: true,
			errMsg:  "BCRYPT_COST must be between 8 and 16",
		},
		{
			name: "signin rate too low",
			config: Config{
				AppPort:            8080,
				BcryptCost:         12,
				SignInRatePerMin:   0,
				LogLevel:           "info",
				LogFormat:          "json",
				MongoURI:           "mongodb://localhost:27017",
				MongoDBName:        "test",
				JWTSecret:          "this-is-a-super-secret-jwt-key-with-32-plus-chars",
				JWTAlgorithm:       "HS256",
				AccessTokenMinutes: 15,
				RefreshTokenDays:   30,
				RefreshTokenRotate: true,
				WSMaxSessionSec:    900,
				WSOutboxBuffer:     256,
			},
			wantErr: true,
			errMsg:  "SIGNIN_RATE_PER_MIN must be greater than or equal to 1",
		},
		{
			name: "JWT secret too short for HS256",
			config: Config{
				AppPort:            8080,
				BcryptCost:         12,
				SignInRatePerMin:   5,
				LogLevel:           "info",
				LogFormat:          "json",
				MongoURI:           "mongodb://localhost:27017",
				MongoDBName:        "test",
				JWTSecret:          "short",
				JWTAlgorithm:       "HS256",
				AccessTokenMinutes: 15,
				RefreshTokenDays:   30,
				RefreshTokenRotate: true,
				WSMaxSessionSec:    900,
				WSOutboxBuffer:     256,
				DevMode:            false,
			},
			wantErr: true,
			errMsg:  "JWT_SECRET must be ≥32 chars",
		},
		{
			name: "invalid JWT algorithm",
			config: Config{
				AppPort:            8080,
				BcryptCost:         12,
				SignInRatePerMin:   5,
				LogLevel:           "info",
				LogFormat:          "json",
				MongoURI:           "mongodb://localhost:27017",
				MongoDBName:        "test",
				JWTSecret:          "this-is-a-super-secret-jwt-key-with-32-plus-chars",
				JWTAlgorithm:       "INVALID",
				AccessTokenMinutes: 15,
				RefreshTokenDays:   30,
				RefreshTokenRotate: true,
				WSMaxSessionSec:    900,
				WSOutboxBuffer:     256,
			},
			wantErr: true,
			errMsg:  "JWT_ALGORITHM must be either HS256",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if err != nil {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_Caching(t *testing.T) {
	t.Parallel()
	clearConfigEnvVars(t)
	ResetCache()

	// Set DEV_MODE=true to bypass JWT_SECRET requirement for tests
	err := os.Setenv("DEV_MODE", "true")
	require.NoError(t, err)
	defer func() {
		err := os.Unsetenv("DEV_MODE")
		require.NoError(t, err)
	}()

	cfg1, err := Load()
	require.NoError(t, err)

	// Don't reset cache - test that subsequent calls return cached result
	cfg2, err := Load()
	require.NoError(t, err)

	assert.Equal(t, cfg1, cfg2)
}

func TestConfig_RequestLoggingDisabled(t *testing.T) {
	t.Parallel()
	clearConfigEnvVars(t)
	ResetCache()

	err := os.Setenv("REQUEST_LOGGING_ENABLED", "false")
	require.NoError(t, err)
	defer func() {
		err := os.Unsetenv("REQUEST_LOGGING_ENABLED")
		require.NoError(t, err)
	}()

	// Set DEV_MODE=true to bypass JWT_SECRET requirement for tests
	err = os.Setenv("DEV_MODE", "true")
	require.NoError(t, err)
	defer func() {
		err := os.Unsetenv("DEV_MODE")
		require.NoError(t, err)
	}()

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, false, cfg.RequestLoggingEnabled)
}

func clearConfigEnvVars(t *testing.T) {
	envVars := []string{
		"APP_PORT",
		"BCRYPT_COST",
		"SIGNIN_RATE_PER_MIN",
		"LOG_LEVEL",
		"LOG_FORMAT",
		"MONGO_URI",
		"MONGO_DB_NAME",
		"JWT_SECRET",
		"JWT_ALGORITHM",
		"WS_MAX_SESSION_SEC",
		"WS_OUTBOX_BUFFER",
		"REQUEST_LOGGING_ENABLED",
	}

	for _, envVar := range envVars {
		if err := os.Unsetenv(envVar); err != nil {
			t.Logf("Warning: failed to unset %s: %v", envVar, err)
		}
	}
}
