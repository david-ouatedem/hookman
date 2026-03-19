//go:build integration

package store

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/david-ouatedem/hookman/internal/id"
	"github.com/david-ouatedem/hookman/internal/store/queries"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

func setupTestStore(t *testing.T) *PostgresStore {
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
	t.Cleanup(func() { pool.Close() })

	// Run migrations
	m, err := migrate.New("file://../../migrations", dbURL)
	if err != nil {
		t.Fatalf("failed to create migrator: %v", err)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		t.Fatalf("migration failed: %v", err)
	}

	// Clean tables before each test
	_, _ = pool.Exec(ctx, "DELETE FROM delivery_attempts")
	_, _ = pool.Exec(ctx, "DELETE FROM events")
	_, _ = pool.Exec(ctx, "DELETE FROM endpoints")

	return NewPostgresStore(pool)
}

func TestIntegrationEventRoundTrip(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	eventID := id.NewEvent()
	err := s.CreateEvent(ctx, queries.CreateEventParams{
		ID:      eventID,
		Topic:   "order.created",
		Payload: []byte(`{"order_id": 123}`),
		Status:  "pending",
	})
	if err != nil {
		t.Fatalf("CreateEvent: %v", err)
	}

	got, err := s.GetEvent(ctx, eventID)
	if err != nil {
		t.Fatalf("GetEvent: %v", err)
	}
	if got.ID != eventID {
		t.Errorf("expected ID %s, got %s", eventID, got.ID)
	}
	if got.Topic != "order.created" {
		t.Errorf("expected topic order.created, got %s", got.Topic)
	}
	if got.Status != "pending" {
		t.Errorf("expected status pending, got %s", got.Status)
	}
}

func TestIntegrationEventStatusLifecycle(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	eventID := id.NewEvent()
	_ = s.CreateEvent(ctx, queries.CreateEventParams{
		ID: eventID, Topic: "test.topic", Payload: []byte(`{}`), Status: "pending",
	})

	// pending → processing
	err := s.UpdateEventStatus(ctx, queries.UpdateEventStatusParams{ID: eventID, Status: "processing"})
	if err != nil {
		t.Fatalf("update to processing: %v", err)
	}
	evt, _ := s.GetEvent(ctx, eventID)
	if evt.Status != "processing" {
		t.Errorf("expected processing, got %s", evt.Status)
	}

	// processing → delivered
	err = s.UpdateEventStatus(ctx, queries.UpdateEventStatusParams{ID: eventID, Status: "delivered"})
	if err != nil {
		t.Fatalf("update to delivered: %v", err)
	}
	evt, _ = s.GetEvent(ctx, eventID)
	if evt.Status != "delivered" {
		t.Errorf("expected delivered, got %s", evt.Status)
	}
}

func TestIntegrationDeadLetterFlow(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	// Create 3 events, mark 2 as dead
	for i := 0; i < 3; i++ {
		eid := id.NewEvent()
		_ = s.CreateEvent(ctx, queries.CreateEventParams{
			ID: eid, Topic: "test.topic", Payload: []byte(`{}`), Status: "pending",
		})
		if i < 2 {
			_ = s.UpdateEventStatus(ctx, queries.UpdateEventStatusParams{ID: eid, Status: "dead"})
		}
	}

	// Count by status
	counts, err := s.CountEventsByStatus(ctx)
	if err != nil {
		t.Fatalf("CountEventsByStatus: %v", err)
	}
	deadCount := int64(0)
	for _, c := range counts {
		if c.Status == "dead" {
			deadCount = c.Count
		}
	}
	if deadCount != 2 {
		t.Errorf("expected 2 dead events, got %d", deadCount)
	}

	// Bulk replay
	replayed, err := s.BulkReplayDeadEvents(ctx)
	if err != nil {
		t.Fatalf("BulkReplayDeadEvents: %v", err)
	}
	if replayed != 2 {
		t.Errorf("expected 2 replayed, got %d", replayed)
	}
}

func TestIntegrationEndpointCRUD(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	epID := id.NewEndpoint()
	err := s.CreateEndpoint(ctx, queries.CreateEndpointParams{
		ID:            epID,
		Url:           "https://example.com/hook",
		Topics:        []string{"order.created", "order.updated"},
		SigningSecret: "whsec_test123",
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("CreateEndpoint: %v", err)
	}

	ep, err := s.GetEndpoint(ctx, epID)
	if err != nil {
		t.Fatalf("GetEndpoint: %v", err)
	}
	if ep.Url != "https://example.com/hook" {
		t.Errorf("expected URL https://example.com/hook, got %s", ep.Url)
	}
	if !ep.Enabled {
		t.Error("expected endpoint to be enabled")
	}

	// Disable
	err = s.UpdateEndpoint(ctx, queries.UpdateEndpointParams{
		ID:      epID,
		Enabled: pgtype.Bool{Bool: false, Valid: true},
	})
	if err != nil {
		t.Fatalf("UpdateEndpoint: %v", err)
	}
	ep, _ = s.GetEndpoint(ctx, epID)
	if ep.Enabled {
		t.Error("expected endpoint to be disabled")
	}

	// List
	eps, err := s.ListEndpoints(ctx)
	if err != nil {
		t.Fatalf("ListEndpoints: %v", err)
	}
	if len(eps) != 1 {
		t.Errorf("expected 1 endpoint, got %d", len(eps))
	}

	// Get by topic
	byTopic, err := s.GetEndpointsByTopic(ctx, "order.created")
	if err != nil {
		t.Fatalf("GetEndpointsByTopic: %v", err)
	}
	if len(byTopic) != 1 {
		t.Errorf("expected 1 endpoint for topic, got %d", len(byTopic))
	}

	// Delete
	err = s.DeleteEndpoint(ctx, epID)
	if err != nil {
		t.Fatalf("DeleteEndpoint: %v", err)
	}
	eps, _ = s.ListEndpoints(ctx)
	if len(eps) != 0 {
		t.Errorf("expected 0 endpoints after delete, got %d", len(eps))
	}
}

func TestIntegrationDeliveryAttempts(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	// Create event and endpoint
	eventID := id.NewEvent()
	epID := id.NewEndpoint()
	_ = s.CreateEvent(ctx, queries.CreateEventParams{
		ID: eventID, Topic: "test.topic", Payload: []byte(`{}`), Status: "processing",
	})
	_ = s.CreateEndpoint(ctx, queries.CreateEndpointParams{
		ID: epID, Url: "https://example.com/hook", Topics: []string{"test.topic"},
		SigningSecret: "whsec_test", Enabled: true,
	})

	// Record a failed attempt with retry
	retryAt := time.Now().Add(-1 * time.Minute)
	_ = s.CreateDeliveryAttempt(ctx, queries.CreateDeliveryAttemptParams{
		ID:            id.NewDeliveryAttempt(),
		EventID:       eventID,
		EndpointID:    epID,
		AttemptNumber: 1,
		Status:        "failed",
		HttpStatus:    pgtype.Int4{Int32: 500, Valid: true},
		DurationMs:    pgtype.Int4{Int32: 150, Valid: true},
		NextRetryAt:   pgtype.Timestamptz{Time: retryAt, Valid: true},
	})

	// Get attempts by event
	attempts, err := s.GetDeliveryAttemptsByEvent(ctx, eventID)
	if err != nil {
		t.Fatalf("GetDeliveryAttemptsByEvent: %v", err)
	}
	if len(attempts) != 1 {
		t.Fatalf("expected 1 attempt, got %d", len(attempts))
	}
	if attempts[0].Status != "failed" {
		t.Errorf("expected failed, got %s", attempts[0].Status)
	}

	// Get retryable attempts
	retryable, err := s.GetRetryableAttempts(ctx, time.Now(), 10)
	if err != nil {
		t.Fatalf("GetRetryableAttempts: %v", err)
	}
	if len(retryable) != 1 {
		t.Errorf("expected 1 retryable attempt, got %d", len(retryable))
	}
}

func TestIntegrationGetPendingEvents(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	// Create pending events
	for i := 0; i < 5; i++ {
		_ = s.CreateEvent(ctx, queries.CreateEventParams{
			ID: id.NewEvent(), Topic: "test.topic", Payload: []byte(`{}`), Status: "pending",
		})
	}

	pending, err := s.GetPendingEvents(ctx, 3)
	if err != nil {
		t.Fatalf("GetPendingEvents: %v", err)
	}
	if len(pending) != 3 {
		t.Errorf("expected 3 pending events (limit), got %d", len(pending))
	}
}

func TestIntegrationCountEventsByStatus(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	_ = s.CreateEvent(ctx, queries.CreateEventParams{ID: id.NewEvent(), Topic: "t", Payload: []byte(`{}`), Status: "pending"})
	_ = s.CreateEvent(ctx, queries.CreateEventParams{ID: id.NewEvent(), Topic: "t", Payload: []byte(`{}`), Status: "pending"})
	_ = s.CreateEvent(ctx, queries.CreateEventParams{ID: id.NewEvent(), Topic: "t", Payload: []byte(`{}`), Status: "delivered"})

	counts, err := s.CountEventsByStatus(ctx)
	if err != nil {
		t.Fatalf("CountEventsByStatus: %v", err)
	}

	statusMap := make(map[string]int64)
	for _, c := range counts {
		statusMap[c.Status] = c.Count
	}
	if statusMap["pending"] != 2 {
		t.Errorf("expected 2 pending, got %d", statusMap["pending"])
	}
	if statusMap["delivered"] != 1 {
		t.Errorf("expected 1 delivered, got %d", statusMap["delivered"])
	}
}
