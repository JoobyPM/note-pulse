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

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, 8080, cfg.AppPort)
	assert.Equal(t, 12, cfg.BcryptCost)
	assert.Equal(t, 5, cfg.SignInRatePerMin)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, "json", cfg.LogFormat)
	assert.Equal(t, "mongodb://mongo:27017", cfg.MongoURI)
	assert.Equal(t, "notepulse", cfg.MongoDBName)
	assert.Equal(t, "this-is-a-default-jwt-secret-key-with-32-plus-characters", cfg.JWTSecret)
	assert.Equal(t, "HS256", cfg.JWTAlgorithm)
	assert.Equal(t, 900, cfg.WSMaxSessionSec)
	assert.Equal(t, 256, cfg.WSOutboxBuffer)
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

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, 9999, cfg.AppPort)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, "json", cfg.LogFormat)
	assert.Equal(t, "mongodb://mongo:27017", cfg.MongoURI)
	assert.Equal(t, "notepulse", cfg.MongoDBName)
	assert.Equal(t, "this-is-a-default-jwt-secret-key-with-32-plus-characters", cfg.JWTSecret)
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
			name: "invalid port",
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
			errMsg:  "APP_PORT must be greater than 0",
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
			},
			wantErr: true,
			errMsg:  "JWT_SECRET cannot be empty",
		},
		{
			name: "bcrypt cost too low",
			config: Config{
				AppPort:            8080,
				BcryptCost:         9,
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
			errMsg:  "BCRYPT_COST must be between 10 and 16",
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
			errMsg:  "BCRYPT_COST must be between 10 and 16",
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
			},
			wantErr: true,
			errMsg:  "JWT_SECRET must be at least 32 characters for HS256",
		},
		{
			name: "valid RS256 algorithm",
			config: Config{
				AppPort:            8080,
				BcryptCost:         12,
				SignInRatePerMin:   5,
				LogLevel:           "info",
				LogFormat:          "json",
				MongoURI:           "mongodb://localhost:27017",
				MongoDBName:        "test",
				JWTSecret:          "this-is-a-super-secret-jwt-key-with-32-plus-chars",
				JWTAlgorithm:       "RS256",
				AccessTokenMinutes: 15,
				RefreshTokenDays:   30,
				RefreshTokenRotate: true,
				WSMaxSessionSec:    900,
				WSOutboxBuffer:     256,
			},
			wantErr: false,
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
			errMsg:  "JWT_ALGORITHM must be either HS256 or RS256",
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

	cfg1, err := Load()
	require.NoError(t, err)

	// Don't reset cache - test that subsequent calls return cached result
	cfg2, err := Load()
	require.NoError(t, err)

	assert.Equal(t, cfg1, cfg2)
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
	}

	for _, envVar := range envVars {
		if err := os.Unsetenv(envVar); err != nil {
			t.Logf("Warning: failed to unset %s: %v", envVar, err)
		}
	}
}
