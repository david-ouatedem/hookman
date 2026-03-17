package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad_RequiredVars(t *testing.T) {
	// Clear all required vars
	os.Unsetenv("DATABASE_URL")
	os.Unsetenv("API_KEY")
	os.Unsetenv("SIGNING_SECRET")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when DATABASE_URL is missing")
	}

	os.Setenv("DATABASE_URL", "postgres://localhost/test")
	_, err = Load()
	if err == nil {
		t.Fatal("expected error when API_KEY is missing")
	}

	os.Setenv("API_KEY", "test-key")
	_, err = Load()
	if err == nil {
		t.Fatal("expected error when SIGNING_SECRET is missing")
	}

	os.Setenv("SIGNING_SECRET", "test-secret")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Port != 4000 {
		t.Errorf("expected default port 4000, got %d", cfg.Port)
	}
	if cfg.WorkerConcurrency != 20 {
		t.Errorf("expected default concurrency 20, got %d", cfg.WorkerConcurrency)
	}
	if cfg.MaxRetries != 5 {
		t.Errorf("expected default max retries 5, got %d", cfg.MaxRetries)
	}
	if cfg.RequestTimeout != 10*time.Second {
		t.Errorf("expected default timeout 10s, got %v", cfg.RequestTimeout)
	}
	if cfg.PollInterval != 1*time.Second {
		t.Errorf("expected default poll interval 1s, got %v", cfg.PollInterval)
	}

	// Cleanup
	os.Unsetenv("DATABASE_URL")
	os.Unsetenv("API_KEY")
	os.Unsetenv("SIGNING_SECRET")
}

func TestLoad_CustomValues(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://localhost/test")
	os.Setenv("API_KEY", "my-key")
	os.Setenv("SIGNING_SECRET", "my-secret")
	os.Setenv("PORT", "8080")
	os.Setenv("WORKER_CONCURRENCY", "50")
	os.Setenv("MAX_RETRIES", "10")
	os.Setenv("REQUEST_TIMEOUT", "30s")
	os.Setenv("POLL_INTERVAL", "5s")
	os.Setenv("METRICS_ENABLED", "true")
	os.Setenv("LOG_LEVEL", "debug")
	os.Setenv("DASHBOARD_ENABLED", "false")

	defer func() {
		for _, k := range []string{"DATABASE_URL", "API_KEY", "SIGNING_SECRET", "PORT",
			"WORKER_CONCURRENCY", "MAX_RETRIES", "REQUEST_TIMEOUT", "POLL_INTERVAL",
			"METRICS_ENABLED", "LOG_LEVEL", "DASHBOARD_ENABLED"} {
			os.Unsetenv(k)
		}
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Port != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.Port)
	}
	if cfg.WorkerConcurrency != 50 {
		t.Errorf("expected concurrency 50, got %d", cfg.WorkerConcurrency)
	}
	if cfg.MaxRetries != 10 {
		t.Errorf("expected max retries 10, got %d", cfg.MaxRetries)
	}
	if cfg.RequestTimeout != 30*time.Second {
		t.Errorf("expected timeout 30s, got %v", cfg.RequestTimeout)
	}
	if cfg.PollInterval != 5*time.Second {
		t.Errorf("expected poll interval 5s, got %v", cfg.PollInterval)
	}
	if !cfg.MetricsEnabled {
		t.Error("expected metrics enabled")
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected log level debug, got %s", cfg.LogLevel)
	}
	if cfg.DashboardEnabled {
		t.Error("expected dashboard disabled")
	}
}
