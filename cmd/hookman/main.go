package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/david-ouatedem/hookman/internal/api"
	"github.com/david-ouatedem/hookman/internal/config"
	"github.com/david-ouatedem/hookman/internal/store"
	"github.com/david-ouatedem/hookman/internal/worker"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
)

var version = "0.4.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		runServe()
	case "migrate":
		runMigrate()
	case "version":
		fmt.Printf("hookman %s\n", version)
	default:
		printUsage()
		os.Exit(1)
	}
}

func runServe() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	setupLogger(cfg.LogLevel)

	slog.Info("hookman starting",
		"version", version,
		"port", cfg.Port,
		"workers", cfg.WorkerConcurrency,
	)

	// Connect to Postgres
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		slog.Error("failed to ping database", "error", err)
		os.Exit(1)
	}
	slog.Info("connected to database")

	// Initialize store
	s := store.NewPostgresStore(pool)

	// Initialize worker pool
	wp := worker.NewPool(cfg.WorkerConcurrency, cfg.MaxRetries, s, cfg.RequestTimeout)
	wp.Start(ctx)

	// Initialize poller
	poller := worker.NewPoller(s, cfg.PollInterval, cfg.MaxRetries, wp.Jobs())
	go poller.Start(ctx)

	// Initialize API server
	srv := api.NewServer(cfg, s)

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("HTTP server error", "error", err)
			os.Exit(1)
		}
	}()

	slog.Info("hookman ready", "port", cfg.Port)

	<-sigCh
	slog.Info("shutting down...")

	// Signal health/ready endpoints to return 503
	srv.NotifyShutdown()

	// Grace period for load balancers to stop routing
	time.Sleep(5 * time.Second)

	// Stop poller and workers
	cancel()

	// Drain in-flight HTTP requests
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("HTTP server shutdown error", "error", err)
	}

	// Wait for workers to finish current jobs
	wp.Stop()
	slog.Info("hookman stopped")
}

func runMigrate() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	setupLogger(cfg.LogLevel)

	m, err := migrate.New("file://migrations", cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to create migrate instance", "error", err)
		os.Exit(1)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		slog.Error("migration failed", "error", err)
		os.Exit(1)
	}

	slog.Info("migrations applied successfully")
}

func setupLogger(level string) {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	slog.SetDefault(slog.New(handler))
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `hookman — webhook delivery service

Usage:
  hookman serve      Start the HTTP server and worker pool
  hookman migrate    Run database migrations
  hookman version    Print version
`)
}
