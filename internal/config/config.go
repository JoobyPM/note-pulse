package config

import (
	"errors"
	"sync"

	"github.com/spf13/viper"
)

// Config holds all application configuration
type Config struct {
	AppPort             int    `mapstructure:"APP_PORT"`
	BcryptCost          int    `mapstructure:"BCRYPT_COST"`
	SignInRatePerMin    int    `mapstructure:"SIGNIN_RATE_PER_MIN"`
	LogLevel            string `mapstructure:"LOG_LEVEL"`
	LogFormat           string `mapstructure:"LOG_FORMAT"`
	MongoURI            string `mapstructure:"MONGO_URI"`
	MongoDBName         string `mapstructure:"MONGO_DB_NAME"`
	JWTSecret           string `mapstructure:"JWT_SECRET"`
	JWTAlgorithm        string `mapstructure:"JWT_ALGORITHM"`
	WSMaxSessionSec     int    `mapstructure:"WS_MAX_SESSION_SEC"`
	AccessTokenMinutes  int    `mapstructure:"ACCESS_TOKEN_MINUTES"`
	RefreshTokenDays    int    `mapstructure:"REFRESH_TOKEN_DAYS"`
	RefreshTokenRotate  bool   `mapstructure:"REFRESH_TOKEN_ROTATE"`
	WSOutboxBuffer      int    `mapstructure:"WS_OUTBOX_BUFFER"`
	RouteMetricsEnabled bool   `mapstructure:"ROUTE_METRICS_ENABLED"`
}

var (
	cachedConfig *Config
	configMutex  sync.RWMutex
)

// Load loads configuration from environment variables and .env file
// It caches the result for subsequent calls
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
	v.SetDefault("BCRYPT_COST", 12)
	v.SetDefault("SIGNIN_RATE_PER_MIN", 5)
	v.SetDefault("LOG_LEVEL", "info")
	v.SetDefault("LOG_FORMAT", "json")
	v.SetDefault("MONGO_URI", "mongodb://mongo:27017")
	v.SetDefault("MONGO_DB_NAME", "notepulse")
	v.SetDefault("JWT_SECRET", "this-is-a-default-jwt-secret-key-with-32-plus-characters")
	v.SetDefault("JWT_ALGORITHM", "HS256")
	v.SetDefault("WS_MAX_SESSION_SEC", 900)
	v.SetDefault("ACCESS_TOKEN_MINUTES", 15)
	v.SetDefault("REFRESH_TOKEN_DAYS", 30)
	v.SetDefault("REFRESH_TOKEN_ROTATE", true)
	v.SetDefault("WS_OUTBOX_BUFFER", 256) // WebSocket channel buffer size
	v.SetDefault("ROUTE_METRICS_ENABLED", true)

	// Configure Viper to read from .env file (if present)
	v.SetConfigName(".env")
	v.SetConfigType("env")
	v.AddConfigPath(".")

	// Try to read .env file (it's okay if it doesn't exist)
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return Config{}, err
		}
	}

	// Override with OS environment variables
	v.AutomaticEnv()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return Config{}, err
	}

	// Validate the configuration
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	// Cache the configuration
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
	if c.AppPort <= 0 {
		return errors.New("APP_PORT must be greater than 0")
	}
	if c.BcryptCost < 10 || c.BcryptCost > 16 {
		return errors.New("BCRYPT_COST must be between 10 and 16")
	}
	if c.SignInRatePerMin < 1 {
		return errors.New("SIGNIN_RATE_PER_MIN must be greater than or equal to 1")
	}
	if c.LogLevel == "" {
		return errors.New("LOG_LEVEL cannot be empty")
	}
	if c.LogFormat == "" {
		return errors.New("LOG_FORMAT cannot be empty")
	}
	if c.MongoURI == "" {
		return errors.New("MONGO_URI cannot be empty")
	}
	if c.MongoDBName == "" {
		return errors.New("MONGO_DB_NAME cannot be empty")
	}
	if c.JWTSecret == "" {
		return errors.New("JWT_SECRET cannot be empty")
	}
	if c.JWTAlgorithm == "HS256" && len(c.JWTSecret) < 32 {
		return errors.New("JWT_SECRET must be at least 32 characters for HS256")
	}
	if c.WSMaxSessionSec <= 0 {
		return errors.New("WS_MAX_SESSION_SEC must be greater than 0")
	}
	if c.WSOutboxBuffer <= 0 {
		return errors.New("WS_OUTBOX_BUFFER must be greater than 0")
	}
	if c.AccessTokenMinutes <= 0 {
		return errors.New("ACCESS_TOKEN_MINUTES must be greater than 0")
	}
	if c.RefreshTokenDays <= 0 {
		return errors.New("REFRESH_TOKEN_DAYS must be greater than 0")
	}
	// Validate JWT algorithm
	switch c.JWTAlgorithm {
	case "HS256", "RS256":
		// Valid algorithms
	default:
		return errors.New("JWT_ALGORITHM must be either HS256 or RS256")
	}
	return nil
}
