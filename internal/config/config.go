package config

import (
	"errors"
	"strings"
	"sync"

	"github.com/spf13/viper"
)

var (
	ErrJWTSecretRequired          = errors.New("JWT_SECRET is required (see .env.template)")
	ErrJWTSecretTooShort          = errors.New("JWT_SECRET must be ≥32 chars")
	ErrAppPortRange               = errors.New("APP_PORT must be between 1 and 65535")
	ErrBcryptCostRange            = errors.New("BCRYPT_COST must be between 8 and 16")
	ErrAuthRatePerMin             = errors.New("AUTH_RATE_PER_MIN must be greater than or equal to 1")
	ErrAppRatePerMin              = errors.New("APP_RATE_PER_MIN must be greater than or equal to 0, 0 means no rate limiting")
	ErrLogLevelEmpty              = errors.New("LOG_LEVEL cannot be empty")
	ErrLogFormatEmpty             = errors.New("LOG_FORMAT cannot be empty")
	ErrMongoURIEmpty              = errors.New("MONGO_URI cannot be empty")
	ErrMongoDBNameEmpty           = errors.New("MONGO_DB_NAME cannot be empty")
	ErrWSMaxSessionSecPositive    = errors.New("WS_MAX_SESSION_SEC must be greater than 0")
	ErrWSOutboxBufferPositive     = errors.New("WS_OUTBOX_BUFFER must be greater than 0")
	ErrAccessTokenMinutesPositive = errors.New("ACCESS_TOKEN_MINUTES must be greater than 0")
	ErrRefreshTokenDaysPositive   = errors.New("REFRESH_TOKEN_DAYS must be greater than 0")
	ErrJWTAlgorithmUnsupported    = errors.New("JWT_ALGORITHM must be HS256")
)

// Config holds all application configuration.
type Config struct {
	AppPort               int    `mapstructure:"APP_PORT"`
	BcryptCost            int    `mapstructure:"BCRYPT_COST"`
	AuthRatePerMin        int    `mapstructure:"AUTH_RATE_PER_MIN"`
	AppRatePerMin         int    `mapstructure:"APP_RATE_PER_MIN"`
	LogLevel              string `mapstructure:"LOG_LEVEL"`
	LogFormat             string `mapstructure:"LOG_FORMAT"`
	MongoURI              string `mapstructure:"MONGO_URI"`
	MongoDBName           string `mapstructure:"MONGO_DB_NAME"`
	JWTSecret             string `mapstructure:"JWT_SECRET"`
	JWTAlgorithm          string `mapstructure:"JWT_ALGORITHM"`
	WSMaxSessionSec       int    `mapstructure:"WS_MAX_SESSION_SEC"`
	AccessTokenMinutes    int    `mapstructure:"ACCESS_TOKEN_MINUTES"`
	RefreshTokenDays      int    `mapstructure:"REFRESH_TOKEN_DAYS"`
	RefreshTokenRotate    bool   `mapstructure:"REFRESH_TOKEN_ROTATE"`
	WSOutboxBuffer        int    `mapstructure:"WS_OUTBOX_BUFFER"`
	RouteMetricsEnabled   bool   `mapstructure:"ROUTE_METRICS_ENABLED"`
	RequestLoggingEnabled bool   `mapstructure:"REQUEST_LOGGING_ENABLED"`
	PprofEnabled          bool   `mapstructure:"PPROF_ENABLED"`
	PyroscopeEnabled      bool   `mapstructure:"PYROSCOPE_ENABLED"`
	PyroscopeServerAddr   string `mapstructure:"PYROSCOPE_SERVER_ADDR"`
	PyroscopeAppName      string `mapstructure:"PYROSCOPE_APP_NAME"`
	DevMode               bool   `mapstructure:"DEV_MODE"`
}

var (
	cachedConfig *Config
	configMutex  sync.RWMutex
)

// Load loads configuration from environment variables and optional .env file.
// The result is cached for subsequent calls.
func Load() (Config, error) {
	configMutex.RLock()
	if cachedConfig != nil {
		defer configMutex.RUnlock()
		return *cachedConfig, nil
	}
	configMutex.RUnlock()

	configMutex.Lock()
	defer configMutex.Unlock()

	// Double-check in case another goroutine loaded it while we waited for the lock
	if cachedConfig != nil {
		return *cachedConfig, nil
	}

	v := viper.New()

	// Set defaults
	v.SetDefault("APP_PORT", 8080)
	v.SetDefault("BCRYPT_COST", 8)
	v.SetDefault("AUTH_RATE_PER_MIN", 5)
	v.SetDefault("APP_RATE_PER_MIN", 0)
	v.SetDefault("LOG_LEVEL", "info")
	v.SetDefault("LOG_FORMAT", "json")
	v.SetDefault("MONGO_URI", "mongodb://mongo:27017")
	v.SetDefault("MONGO_DB_NAME", "notepulse")
	v.SetDefault("DEV_MODE", false)
	v.SetDefault("JWT_ALGORITHM", "HS256")
	v.SetDefault("WS_MAX_SESSION_SEC", 900)
	v.SetDefault("ACCESS_TOKEN_MINUTES", 15)
	v.SetDefault("REFRESH_TOKEN_DAYS", 30)
	v.SetDefault("REFRESH_TOKEN_ROTATE", true)
	v.SetDefault("WS_OUTBOX_BUFFER", 256) // WebSocket channel buffer size
	v.SetDefault("ROUTE_METRICS_ENABLED", true)
	v.SetDefault("REQUEST_LOGGING_ENABLED", true)
	v.SetDefault("PPROF_ENABLED", false)
	v.SetDefault("PYROSCOPE_ENABLED", false)
	v.SetDefault("PYROSCOPE_SERVER_ADDR", "http://pyroscope:4040")
	v.SetDefault("PYROSCOPE_APP_NAME", "notepulse-server")
	v.SetDefault("JWT_SECRET", "")

	// Configure Viper to read from .env file (if present)
	v.SetConfigName(".env")
	v.SetConfigType("env")
	v.AddConfigPath(".")

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return Config{}, err
		}
	}

	// Override with OS environment variables
	v.AutomaticEnv()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return Config{}, err
	}

	// Normalize JWT algorithm to uppercase
	cfg.JWTAlgorithm = strings.ToUpper(cfg.JWTAlgorithm)

	// Validate the configuration
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	cachedConfig = &cfg
	return cfg, nil
}

// ResetCache clears the cached configuration (for testing purposes)
func ResetCache() {
	configMutex.Lock()
	defer configMutex.Unlock()
	cachedConfig = nil
}

// Validate checks if required configuration fields are properly set
func (c Config) Validate() error {
	if err := c.validateJWT(); err != nil {
		return err
	}
	if err := c.validateRanges(); err != nil {
		return err
	}
	if err := c.validateStringFields(); err != nil {
		return err
	}
	if err := c.validatePositiveNumbers(); err != nil {
		return err
	}
	return nil
}

// validateJWT validates JWT-related configuration
func (c Config) validateJWT() error {
	if c.JWTSecret == "" && !c.DevMode {
		return ErrJWTSecretRequired
	}
	if len(c.JWTSecret) < 32 && !c.DevMode {
		return ErrJWTSecretTooShort
	}
	// Validate JWT algorithm, in future I may add support RS256
	switch c.JWTAlgorithm {
	case "HS256":
		// ok
	default:
		return ErrJWTAlgorithmUnsupported
	}
	return nil
}

// validateRanges validates numeric fields that must be within specific ranges
func (c Config) validateRanges() error {
	if c.AppPort <= 0 || c.AppPort > 65535 {
		return ErrAppPortRange
	}
	if c.BcryptCost < 8 || c.BcryptCost > 16 {
		return ErrBcryptCostRange
	}
	if c.AuthRatePerMin < 1 {
		return ErrAuthRatePerMin
	}
	if c.AppRatePerMin < 0 {
		return ErrAppRatePerMin
	}
	return nil
}

// validateStringFields validates string fields that cannot be empty
func (c Config) validateStringFields() error {
	if c.LogLevel == "" {
		return ErrLogLevelEmpty
	}
	if c.LogFormat == "" {
		return ErrLogFormatEmpty
	}
	if c.MongoURI == "" {
		return ErrMongoURIEmpty
	}
	if c.MongoDBName == "" {
		return ErrMongoDBNameEmpty
	}
	return nil
}

// validatePositiveNumbers validates numeric fields that must be positive
func (c Config) validatePositiveNumbers() error {
	if c.WSMaxSessionSec <= 0 {
		return ErrWSMaxSessionSecPositive
	}
	if c.WSOutboxBuffer <= 0 {
		return ErrWSOutboxBufferPositive
	}
	if c.AccessTokenMinutes <= 0 {
		return ErrAccessTokenMinutesPositive
	}
	if c.RefreshTokenDays <= 0 {
		return ErrRefreshTokenDaysPositive
	}
	return nil
}
