package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// -----------------------------------------------------------------------------
// helpers
// -----------------------------------------------------------------------------

// baseValidConfig returns a fully-valid configuration object that callers
// can tweak inside table tests.
func baseValidConfig() Config {
	return Config{
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
	}
}

// clearConfigEnvVars removes every environment variable that the Config loader
// consumes so each test starts with a clean slate.
func clearConfigEnvVars(t *testing.T) {
	t.Helper()

	for _, k := range []string{
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
		"DEV_MODE",
	} {
		if err := os.Unsetenv(k); err != nil {
			t.Logf("warning: failed to unset %s: %v", k, err)
		}
	}
}

func TestConfigLoadDefaults(t *testing.T) {
	clearConfigEnvVars(t)
	ResetCache()

	t.Setenv("DEV_MODE", "true") // bypass JWT secret requirement

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, 8080, cfg.AppPort)
	assert.Equal(t, 8, cfg.BcryptCost)
	assert.Equal(t, 5, cfg.SignInRatePerMin)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, "json", cfg.LogFormat)
	assert.Equal(t, "mongodb://mongo:27017", cfg.MongoURI)
	assert.Equal(t, "notepulse", cfg.MongoDBName)
	assert.Equal(t, "", cfg.JWTSecret) // no default secret
	assert.Equal(t, "HS256", cfg.JWTAlgorithm)
	assert.True(t, cfg.DevMode)
	assert.Equal(t, 900, cfg.WSMaxSessionSec)
	assert.Equal(t, 256, cfg.WSOutboxBuffer)
	assert.True(t, cfg.RequestLoggingEnabled)
}

func TestConfigLoadWithOverride(t *testing.T) {
	clearConfigEnvVars(t)
	ResetCache()

	t.Setenv("DEV_MODE", "true")
	t.Setenv("APP_PORT", "9999")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, 9999, cfg.AppPort)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, "json", cfg.LogFormat)
	assert.Equal(t, "mongodb://mongo:27017", cfg.MongoURI)
	assert.Equal(t, "notepulse", cfg.MongoDBName)
	assert.Equal(t, "", cfg.JWTSecret)
	assert.True(t, cfg.DevMode)
}

func TestConfigCaching(t *testing.T) {
	clearConfigEnvVars(t)
	ResetCache()

	t.Setenv("DEV_MODE", "true")

	cfg1, err := Load()
	require.NoError(t, err)

	// second call should hit the cache
	cfg2, err := Load()
	require.NoError(t, err)

	assert.Equal(t, cfg1, cfg2)
}

func TestConfigRequestLoggingDisabled(t *testing.T) {
	clearConfigEnvVars(t)
	ResetCache()

	t.Setenv("DEV_MODE", "true")
	t.Setenv("REQUEST_LOGGING_ENABLED", "false")

	cfg, err := Load()
	require.NoError(t, err)

	assert.False(t, cfg.RequestLoggingEnabled)
}

// -----------------------------------------------------------------------------
// Validate() unit tests (table-driven)
// -----------------------------------------------------------------------------

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Config) // mutates the baseValidConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			modify: func(*Config) {
				// No-op: baseValidConfig already returns a valid configuration.
				// Leaving this empty makes the test exercise the happy-path
				// scenario where Validate should succeed without any mutations.
			},
		},
		{
			name: "invalid port - zero",
			modify: func(c *Config) {
				c.AppPort = 0
			},
			wantErr: true,
			errMsg:  ErrAppPortRange.Error(),
		},
		{
			name: "invalid port - negative",
			modify: func(c *Config) {
				c.AppPort = -1
			},
			wantErr: true,
			errMsg:  ErrAppPortRange.Error(),
		},
		{
			name: "invalid port - too high",
			modify: func(c *Config) {
				c.AppPort = 70000
			},
			wantErr: true,
			errMsg:  ErrAppPortRange.Error(),
		},
		{
			name: "empty log level",
			modify: func(c *Config) {
				c.LogLevel = ""
			},
			wantErr: true,
			errMsg:  ErrLogLevelEmpty.Error(),
		},
		{
			name: "empty JWT secret",
			modify: func(c *Config) {
				c.JWTSecret = ""
				c.DevMode = false
			},
			wantErr: true,
			errMsg:  ErrJWTSecretRequired.Error(),
		},
		{
			name: "bcrypt cost too low",
			modify: func(c *Config) {
				c.BcryptCost = 7
			},
			wantErr: true,
			errMsg:  ErrBcryptCostRange.Error(),
		},
		{
			name: "bcrypt cost too high",
			modify: func(c *Config) {
				c.BcryptCost = 17
			},
			wantErr: true,
			errMsg:  ErrBcryptCostRange.Error(),
		},
		{
			name: "signin rate too low",
			modify: func(c *Config) {
				c.SignInRatePerMin = 0
			},
			wantErr: true,
			errMsg:  ErrSignInRatePerMin.Error(),
		},
		{
			name: "JWT secret too short for HS256",
			modify: func(c *Config) {
				c.JWTSecret = "short"
				c.DevMode = false
			},
			wantErr: true,
			errMsg:  ErrJWTSecretTooShort.Error(),
		},
		{
			name: "invalid JWT algorithm",
			modify: func(c *Config) {
				c.JWTAlgorithm = "INVALID"
			},
			wantErr: true,
			errMsg:  ErrJWTAlgorithmUnsupported.Error(),
		},
	}

	for _, tt := range tests {
		tt := tt // capture range variable
		t.Run(tt.name, func(t *testing.T) {
			cfg := baseValidConfig()
			tt.modify(&cfg)

			err := cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Equal(t, tt.errMsg, err.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
