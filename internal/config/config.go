package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	DatabaseURL       string
	APIKey            string
	SigningSecret     string
	Port              int
	WorkerConcurrency int
	MaxRetries        int
	RequestTimeout    time.Duration
	PollInterval      time.Duration
	MetricsEnabled    bool
	LogLevel          string
	DashboardEnabled  bool
}

// Load reads configuration from environment variables with sensible defaults.
func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		APIKey:            os.Getenv("API_KEY"),
		SigningSecret:     os.Getenv("SIGNING_SECRET"),
		Port:              getEnvInt("PORT", 4000),
		WorkerConcurrency: getEnvInt("WORKER_CONCURRENCY", 20),
		MaxRetries:        getEnvInt("MAX_RETRIES", 5),
		RequestTimeout:    getEnvDuration("REQUEST_TIMEOUT", 10*time.Second),
		PollInterval:      getEnvDuration("POLL_INTERVAL", 1*time.Second),
		MetricsEnabled:    getEnvBool("METRICS_ENABLED", false),
		LogLevel:          getEnvString("LOG_LEVEL", "info"),
		DashboardEnabled:  getEnvBool("DASHBOARD_ENABLED", true),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("config: DATABASE_URL is required")
	}
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("config: API_KEY is required")
	}
	if cfg.SigningSecret == "" {
		return nil, fmt.Errorf("config: SIGNING_SECRET is required")
	}

	return cfg, nil
}

func getEnvString(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func getEnvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}
