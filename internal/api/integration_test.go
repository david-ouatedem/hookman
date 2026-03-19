//go:build integration

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/david-ouatedem/hookman/internal/config"
	"github.com/david-ouatedem/hookman/internal/store"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
)

func setupIntegrationServer(t *testing.T) (*Server, func()) {
	t.Helper()

	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}

	// Run migrations
	m, err := migrate.New("file://../../migrations", dbURL)
	if err != nil {
		pool.Close()
		t.Fatalf("failed to create migrator: %v", err)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		pool.Close()
		t.Fatalf("migration failed: %v", err)
	}

	// Clean tables
	_, _ = pool.Exec(ctx, "DELETE FROM delivery_attempts")
	_, _ = pool.Exec(ctx, "DELETE FROM events")
	_, _ = pool.Exec(ctx, "DELETE FROM endpoints")

	s := store.NewPostgresStore(pool)
	cfg := &config.Config{
		Port:             4000,
		APIKey:           "test-api-key",
		SigningSecret:    "test-signing-secret",
		DashboardEnabled: false,
		MetricsEnabled:   false,
		RateLimitRPS:     0,
	}

	srv := NewServer(cfg, s)
	return srv, func() { pool.Close() }
}

func doIntegrationRequest(srv *Server, method, path string, body string) *httptest.ResponseRecorder {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req.Header.Set("Authorization", "Bearer test-api-key")
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)
	return w
}

func TestIntegrationAPIEventLifecycle(t *testing.T) {
	srv, cleanup := setupIntegrationServer(t)
	defer cleanup()

	// Create event
	w := doIntegrationRequest(srv, "POST", "/api/events", `{"topic":"order.created","payload":{"id":1}}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var createResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &createResp)
	eventID := createResp["id"].(string)

	// Get event
	w = doIntegrationRequest(srv, "GET", "/api/events/"+eventID, "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var getResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &getResp)
	if getResp["status"] != "pending" {
		t.Errorf("expected pending, got %s", getResp["status"])
	}

	// List events
	w = doIntegrationRequest(srv, "GET", "/api/events", "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var listResp []interface{}
	json.Unmarshal(w.Body.Bytes(), &listResp)
	if len(listResp) != 1 {
		t.Errorf("expected 1 event, got %d", len(listResp))
	}
}

func TestIntegrationAPIEndpointCRUD(t *testing.T) {
	srv, cleanup := setupIntegrationServer(t)
	defer cleanup()

	// Create endpoint
	w := doIntegrationRequest(srv, "POST", "/api/endpoints", `{
		"url": "https://example.com/webhook",
		"topics": ["order.created"]
	}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var createResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &createResp)
	epID := createResp["id"].(string)

	// List endpoints
	w = doIntegrationRequest(srv, "GET", "/api/endpoints", "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Disable endpoint
	w = doIntegrationRequest(srv, "PATCH", "/api/endpoints/"+epID, `{"enabled": false}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Delete endpoint
	w = doIntegrationRequest(srv, "DELETE", "/api/endpoints/"+epID, "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
}

func TestIntegrationAPIStats(t *testing.T) {
	srv, cleanup := setupIntegrationServer(t)
	defer cleanup()

	// Create some events
	doIntegrationRequest(srv, "POST", "/api/events", `{"topic":"t","payload":{}}`)
	doIntegrationRequest(srv, "POST", "/api/events", `{"topic":"t","payload":{}}`)

	w := doIntegrationRequest(srv, "GET", "/api/stats", "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var stats map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &stats)
	total := stats["total"].(float64)
	if total != 2 {
		t.Errorf("expected total 2, got %v", total)
	}
}

func TestIntegrationAPIRateLimiting(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer pool.Close()

	m, _ := migrate.New("file://../../migrations", dbURL)
	_ = m.Up()
	_, _ = pool.Exec(ctx, "DELETE FROM delivery_attempts")
	_, _ = pool.Exec(ctx, "DELETE FROM events")
	_, _ = pool.Exec(ctx, "DELETE FROM endpoints")

	s := store.NewPostgresStore(pool)
	cfg := &config.Config{
		Port:           4000,
		APIKey:         "test-api-key",
		SigningSecret:  "test-signing-secret",
		RateLimitRPS:   5,
		RateLimitBurst: 5,
	}

	srv := NewServer(cfg, s)

	// Send burst of requests — first 5 should succeed, rest should be 429
	got429 := false
	for i := 0; i < 20; i++ {
		w := doIntegrationRequest(srv, "GET", "/api/stats", "")
		if w.Code == http.StatusTooManyRequests {
			got429 = true
			break
		}
	}
	if !got429 {
		t.Error("expected at least one 429 response from rate limiting")
	}
}

func TestIntegrationAPIIdempotency(t *testing.T) {
	srv, cleanup := setupIntegrationServer(t)
	defer cleanup()

	body := `{"topic":"order.created","payload":{"id":1}}`

	// First request with idempotency key
	req := httptest.NewRequest("POST", "/api/events", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-api-key")
	req.Header.Set("Idempotency-Key", "unique-key-123")
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("first request: expected 201, got %d", w.Code)
	}

	var resp1 map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp1)

	// Second request with same key — should return same event
	req = httptest.NewRequest("POST", "/api/events", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-api-key")
	req.Header.Set("Idempotency-Key", "unique-key-123")
	w = httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("second request: expected 200, got %d", w.Code)
	}

	var resp2 map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp2)
	if resp1["id"] != resp2["id"] {
		t.Errorf("expected same event ID, got %s vs %s", resp1["id"], resp2["id"])
	}
}

func TestIntegrationAPIHealthAndReady(t *testing.T) {
	srv, cleanup := setupIntegrationServer(t)
	defer cleanup()

	// Health should return 200
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("health: expected 200, got %d", w.Code)
	}

	// Ready should return 200
	req = httptest.NewRequest("GET", "/ready", nil)
	w = httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("ready: expected 200, got %d", w.Code)
	}

	// After shutdown notification, both should return 503
	srv.NotifyShutdown()
	time.Sleep(10 * time.Millisecond)

	req = httptest.NewRequest("GET", "/health", nil)
	w = httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("health after shutdown: expected 503, got %d", w.Code)
	}

	req = httptest.NewRequest("GET", "/ready", nil)
	w = httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("ready after shutdown: expected 503, got %d", w.Code)
	}
}

func TestIntegrationAPIDeadLetter(t *testing.T) {
	srv, cleanup := setupIntegrationServer(t)
	defer cleanup()

	// Create events and mark as dead via replay (need direct DB for this)
	for i := 0; i < 3; i++ {
		w := doIntegrationRequest(srv, "POST", "/api/events", fmt.Sprintf(`{"topic":"t","payload":{"i":%d}}`, i))
		if w.Code != http.StatusCreated {
			t.Fatalf("create event %d: expected 201, got %d", i, w.Code)
		}
	}

	// List dead events (should be empty initially)
	w := doIntegrationRequest(srv, "GET", "/api/events/dead", "")
	if w.Code != http.StatusOK {
		t.Fatalf("list dead: expected 200, got %d", w.Code)
	}
}
