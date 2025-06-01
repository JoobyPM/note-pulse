package config

import (
	"errors"
	"sync"

	"github.com/spf13/viper"
)

// Config holds all application configuration
type Config struct {
	AppPort          int    `mapstructure:"APP_PORT"`
	LogLevel         string `mapstructure:"LOG_LEVEL"`
	LogFormat        string `mapstructure:"LOG_FORMAT"`
	MongoURI         string `mapstructure:"MONGO_URI"`
	MongoDBName      string `mapstructure:"MONGO_DB_NAME"`
	JWTSecret        string `mapstructure:"JWT_SECRET"`
	JWTExpiryMinutes int    `mapstructure:"JWT_EXPIRY_MINUTES"`
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
	v.SetDefault("LOG_LEVEL", "info")
	v.SetDefault("LOG_FORMAT", "json")
	v.SetDefault("MONGO_URI", "mongodb://mongo:27017")
	v.SetDefault("MONGO_DB_NAME", "notepulse")
	v.SetDefault("JWT_SECRET", "change-me")
	v.SetDefault("JWT_EXPIRY_MINUTES", 60)

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
	if c.JWTExpiryMinutes <= 0 {
		return errors.New("JWT_EXPIRY_MINUTES must be greater than 0")
	}
	return nil
}
