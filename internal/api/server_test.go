package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/david-ouatedem/hookman/internal/config"
	"github.com/david-ouatedem/hookman/internal/store/queries"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// mockStore implements store.Store for testing.
type mockStore struct {
	events    map[string]queries.Event
	endpoints map[string]queries.Endpoint
	attempts  []queries.DeliveryAttempt
}

func newMockStore() *mockStore {
	return &mockStore{
		events:    make(map[string]queries.Event),
		endpoints: make(map[string]queries.Endpoint),
	}
}

func (m *mockStore) CreateEvent(_ context.Context, params queries.CreateEventParams) error {
	m.events[params.ID] = queries.Event{
		ID:             params.ID,
		Topic:          params.Topic,
		Payload:        params.Payload,
		IdempotencyKey: params.IdempotencyKey,
		Status:         params.Status,
		CreatedAt:      pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	return nil
}

func (m *mockStore) GetEvent(_ context.Context, id string) (queries.Event, error) {
	evt, ok := m.events[id]
	if !ok {
		return queries.Event{}, pgx.ErrNoRows
	}
	return evt, nil
}

func (m *mockStore) GetEventByIdempotencyKey(_ context.Context, key pgtype.Text) (queries.Event, error) {
	for _, evt := range m.events {
		if evt.IdempotencyKey.Valid && evt.IdempotencyKey.String == key.String {
			return evt, nil
		}
	}
	return queries.Event{}, pgx.ErrNoRows
}

func (m *mockStore) ListEvents(_ context.Context, _ queries.ListEventsParams) ([]queries.Event, error) {
	result := make([]queries.Event, 0, len(m.events))
	for _, evt := range m.events {
		result = append(result, evt)
	}
	return result, nil
}

func (m *mockStore) UpdateEventStatus(_ context.Context, params queries.UpdateEventStatusParams) error {
	evt, ok := m.events[params.ID]
	if !ok {
		return pgx.ErrNoRows
	}
	evt.Status = params.Status
	m.events[params.ID] = evt
	return nil
}

func (m *mockStore) GetPendingEvents(_ context.Context, _ int32) ([]queries.Event, error) {
	return nil, nil
}

func (m *mockStore) CreateEndpoint(_ context.Context, params queries.CreateEndpointParams) error {
	m.endpoints[params.ID] = queries.Endpoint{
		ID:            params.ID,
		Url:           params.Url,
		Topics:        params.Topics,
		Description:   params.Description,
		SigningSecret: params.SigningSecret,
		Enabled:       params.Enabled,
		CreatedAt:     pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	return nil
}

func (m *mockStore) GetEndpoint(_ context.Context, id string) (queries.Endpoint, error) {
	ep, ok := m.endpoints[id]
	if !ok {
		return queries.Endpoint{}, pgx.ErrNoRows
	}
	return ep, nil
}

func (m *mockStore) ListEndpoints(_ context.Context) ([]queries.Endpoint, error) {
	result := make([]queries.Endpoint, 0, len(m.endpoints))
	for _, ep := range m.endpoints {
		result = append(result, ep)
	}
	return result, nil
}

func (m *mockStore) UpdateEndpoint(_ context.Context, params queries.UpdateEndpointParams) error {
	ep, ok := m.endpoints[params.ID]
	if !ok {
		return pgx.ErrNoRows
	}
	if params.Url.Valid {
		ep.Url = params.Url.String
	}
	if params.Topics != nil {
		ep.Topics = params.Topics
	}
	if params.Description.Valid {
		ep.Description = params.Description
	}
	if params.Enabled.Valid {
		ep.Enabled = params.Enabled.Bool
	}
	m.endpoints[params.ID] = ep
	return nil
}

func (m *mockStore) DeleteEndpoint(_ context.Context, id string) error {
	delete(m.endpoints, id)
	return nil
}

func (m *mockStore) GetEndpointsByTopic(_ context.Context, topic string) ([]queries.Endpoint, error) {
	var result []queries.Endpoint
	for _, ep := range m.endpoints {
		for _, t := range ep.Topics {
			if t == topic {
				result = append(result, ep)
				break
			}
		}
	}
	return result, nil
}

func (m *mockStore) CreateDeliveryAttempt(_ context.Context, params queries.CreateDeliveryAttemptParams) error {
	m.attempts = append(m.attempts, queries.DeliveryAttempt{
		ID:            params.ID,
		EventID:       params.EventID,
		EndpointID:    params.EndpointID,
		AttemptNumber: params.AttemptNumber,
		HttpStatus:    params.HttpStatus,
		Status:        params.Status,
	})
	return nil
}

func (m *mockStore) GetDeliveryAttemptsByEvent(_ context.Context, eventID string) ([]queries.DeliveryAttempt, error) {
	var result []queries.DeliveryAttempt
	for _, a := range m.attempts {
		if a.EventID == eventID {
			result = append(result, a)
		}
	}
	return result, nil
}

func (m *mockStore) GetDeliveryAttemptsByEndpoint(_ context.Context, params queries.GetDeliveryAttemptsByEndpointParams) ([]queries.DeliveryAttempt, error) {
	var result []queries.DeliveryAttempt
	for _, a := range m.attempts {
		if a.EndpointID == params.EndpointID {
			result = append(result, a)
		}
	}
	return result, nil
}

func (m *mockStore) GetRetryableAttempts(_ context.Context, _ time.Time, _ int32) ([]queries.DeliveryAttempt, error) {
	return nil, nil
}

func (m *mockStore) GetDeadEvents(_ context.Context, params queries.GetDeadEventsParams) ([]queries.Event, error) {
	var result []queries.Event
	for _, evt := range m.events {
		if evt.Status == "dead" {
			result = append(result, evt)
		}
	}
	return result, nil
}

func (m *mockStore) CountEventsByStatus(_ context.Context) ([]queries.CountEventsByStatusRow, error) {
	counts := make(map[string]int64)
	for _, evt := range m.events {
		counts[evt.Status]++
	}
	var result []queries.CountEventsByStatusRow
	for status, count := range counts {
		result = append(result, queries.CountEventsByStatusRow{Status: status, Count: count})
	}
	return result, nil
}

func (m *mockStore) BulkReplayDeadEvents(_ context.Context) (int64, error) {
	var count int64
	for id, evt := range m.events {
		if evt.Status == "dead" {
			evt.Status = "pending"
			m.events[id] = evt
			count++
		}
	}
	return count, nil
}

func (m *mockStore) PurgeDeadEvents(_ context.Context) (int64, error) {
	var count int64
	for id, evt := range m.events {
		if evt.Status == "dead" {
			delete(m.events, id)
			count++
		}
	}
	return count, nil
}

func testServer() (*Server, *mockStore) {
	ms := newMockStore()
	cfg := &config.Config{
		APIKey:        "test-api-key",
		SigningSecret: "test-secret",
		Port:          4000,
	}
	srv := NewServer(cfg, ms)
	return srv, ms
}

func doRequest(srv *Server, method, path string, body any) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Authorization", "Bearer test-api-key")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)
	return w
}

// --- Tests ---

func TestHealthEndpoint(t *testing.T) {
	srv, _ := testServer()
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	srv, _ := testServer()
	req := httptest.NewRequest("GET", "/api/events", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_InvalidKey(t *testing.T) {
	srv, _ := testServer()
	req := httptest.NewRequest("GET", "/api/events", nil)
	req.Header.Set("Authorization", "Bearer wrong-key")
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestCreateEvent(t *testing.T) {
	srv, ms := testServer()
	w := doRequest(srv, "POST", "/api/events", map[string]any{
		"topic":   "payment.completed",
		"payload": map[string]any{"amount": 5000},
	})

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp eventResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Topic != "payment.completed" {
		t.Errorf("expected topic 'payment.completed', got %s", resp.Topic)
	}
	if resp.Status != "pending" {
		t.Errorf("expected status 'pending', got %s", resp.Status)
	}
	if len(ms.events) != 1 {
		t.Errorf("expected 1 event in store, got %d", len(ms.events))
	}
}

func TestCreateEvent_MissingTopic(t *testing.T) {
	srv, _ := testServer()
	w := doRequest(srv, "POST", "/api/events", map[string]any{
		"payload": map[string]any{"amount": 5000},
	})

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCreateEvent_Idempotency(t *testing.T) {
	srv, _ := testServer()
	key := "idem-123"
	body := map[string]any{
		"topic":          "test.event",
		"payload":        map[string]any{"data": true},
		"idempotencyKey": key,
	}

	w1 := doRequest(srv, "POST", "/api/events", body)
	if w1.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w1.Code)
	}

	var resp1 eventResponse
	json.NewDecoder(w1.Body).Decode(&resp1)

	// Second request with same key should return existing event
	w2 := doRequest(srv, "POST", "/api/events", body)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 for idempotent request, got %d", w2.Code)
	}

	var resp2 eventResponse
	json.NewDecoder(w2.Body).Decode(&resp2)

	if resp1.ID != resp2.ID {
		t.Errorf("idempotent requests should return same ID: %s vs %s", resp1.ID, resp2.ID)
	}
}

func TestCreateEndpoint(t *testing.T) {
	srv, ms := testServer()
	w := doRequest(srv, "POST", "/api/endpoints", map[string]any{
		"url":    "https://example.com/webhooks",
		"topics": []string{"payment.completed"},
	})

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp endpointResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.URL != "https://example.com/webhooks" {
		t.Errorf("unexpected URL: %s", resp.URL)
	}
	if resp.SigningSecret == "" {
		t.Error("signing secret should be generated")
	}
	if len(ms.endpoints) != 1 {
		t.Errorf("expected 1 endpoint in store, got %d", len(ms.endpoints))
	}
}

func TestCreateEndpoint_MissingURL(t *testing.T) {
	srv, _ := testServer()
	w := doRequest(srv, "POST", "/api/endpoints", map[string]any{
		"topics": []string{"payment.completed"},
	})

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestDeleteEndpoint(t *testing.T) {
	srv, ms := testServer()

	// Create an endpoint first
	w := doRequest(srv, "POST", "/api/endpoints", map[string]any{
		"url":    "https://example.com/webhooks",
		"topics": []string{"test"},
	})
	var resp endpointResponse
	json.NewDecoder(w.Body).Decode(&resp)

	// Delete it
	w2 := doRequest(srv, "DELETE", "/api/endpoints/"+resp.ID, nil)
	if w2.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w2.Code)
	}

	if len(ms.endpoints) != 0 {
		t.Errorf("expected 0 endpoints after delete, got %d", len(ms.endpoints))
	}
}

func TestDeleteEndpoint_NotFound(t *testing.T) {
	srv, _ := testServer()
	w := doRequest(srv, "DELETE", "/api/endpoints/ep_nonexistent", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestGetEvent_NotFound(t *testing.T) {
	srv, _ := testServer()
	w := doRequest(srv, "GET", "/api/events/evt_nonexistent", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestReplayEvent(t *testing.T) {
	srv, ms := testServer()

	// Create an event and mark it delivered
	w := doRequest(srv, "POST", "/api/events", map[string]any{
		"topic":   "test.event",
		"payload": map[string]any{"data": true},
	})
	var resp eventResponse
	json.NewDecoder(w.Body).Decode(&resp)

	// Mark it as delivered
	evt := ms.events[resp.ID]
	evt.Status = "delivered"
	ms.events[resp.ID] = evt

	// Replay it
	w2 := doRequest(srv, "POST", "/api/events/"+resp.ID+"/replay", nil)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w2.Code, w2.Body.String())
	}

	// Check it's back to pending
	if ms.events[resp.ID].Status != "pending" {
		t.Errorf("expected status 'pending' after replay, got %s", ms.events[resp.ID].Status)
	}
}

// --- Dead Letter Tests ---

func TestListDeadEvents(t *testing.T) {
	srv, ms := testServer()

	// Create events and mark some as dead
	for _, topic := range []string{"a", "b", "c"} {
		w := doRequest(srv, "POST", "/api/events", map[string]any{
			"topic":   topic,
			"payload": map[string]any{"x": 1},
		})
		var resp eventResponse
		json.NewDecoder(w.Body).Decode(&resp)
		if topic != "c" {
			evt := ms.events[resp.ID]
			evt.Status = "dead"
			ms.events[resp.ID] = evt
		}
	}

	w := doRequest(srv, "GET", "/api/events/dead", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var events []eventResponse
	json.NewDecoder(w.Body).Decode(&events)
	if len(events) != 2 {
		t.Errorf("expected 2 dead events, got %d", len(events))
	}
}

func TestBulkReplayDeadEvents(t *testing.T) {
	srv, ms := testServer()

	// Create 2 dead events
	for i := 0; i < 2; i++ {
		w := doRequest(srv, "POST", "/api/events", map[string]any{
			"topic":   "test",
			"payload": map[string]any{"i": i},
		})
		var resp eventResponse
		json.NewDecoder(w.Body).Decode(&resp)
		evt := ms.events[resp.ID]
		evt.Status = "dead"
		ms.events[resp.ID] = evt
	}

	w := doRequest(srv, "POST", "/api/events/dead/replay", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// All should be pending now
	for _, evt := range ms.events {
		if evt.Status != "pending" {
			t.Errorf("expected all events pending after bulk replay, got %s", evt.Status)
		}
	}
}

func TestPurgeDeadEvents(t *testing.T) {
	srv, ms := testServer()

	// Create 3 events: 2 dead, 1 pending
	for i := 0; i < 3; i++ {
		w := doRequest(srv, "POST", "/api/events", map[string]any{
			"topic":   "test",
			"payload": map[string]any{"i": i},
		})
		var resp eventResponse
		json.NewDecoder(w.Body).Decode(&resp)
		if i < 2 {
			evt := ms.events[resp.ID]
			evt.Status = "dead"
			ms.events[resp.ID] = evt
		}
	}

	w := doRequest(srv, "DELETE", "/api/events/dead", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	if len(ms.events) != 1 {
		t.Errorf("expected 1 event remaining after purge, got %d", len(ms.events))
	}
}

// --- Stats Tests ---

func TestStats(t *testing.T) {
	srv, ms := testServer()

	// Create events with different statuses
	for _, s := range []struct{ topic, status string }{
		{"a", "pending"},
		{"b", "delivered"},
		{"c", "delivered"},
		{"d", "dead"},
	} {
		w := doRequest(srv, "POST", "/api/events", map[string]any{
			"topic":   s.topic,
			"payload": map[string]any{"x": 1},
		})
		var resp eventResponse
		json.NewDecoder(w.Body).Decode(&resp)
		evt := ms.events[resp.ID]
		evt.Status = s.status
		ms.events[resp.ID] = evt
	}

	w := doRequest(srv, "GET", "/api/stats", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var stats map[string]int64
	json.NewDecoder(w.Body).Decode(&stats)

	if stats["pending"] != 1 {
		t.Errorf("expected 1 pending, got %d", stats["pending"])
	}
	if stats["delivered"] != 2 {
		t.Errorf("expected 2 delivered, got %d", stats["delivered"])
	}
	if stats["dead"] != 1 {
		t.Errorf("expected 1 dead, got %d", stats["dead"])
	}
	if stats["total"] != 4 {
		t.Errorf("expected total 4, got %d", stats["total"])
	}
}

// --- Ready Endpoint Tests ---

func TestReadyEndpoint(t *testing.T) {
	srv, _ := testServer()
	req := httptest.NewRequest("GET", "/ready", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHealthDuringShutdown(t *testing.T) {
	srv, _ := testServer()
	srv.NotifyShutdown()

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "shutting_down" {
		t.Errorf("expected shutting_down, got %s", resp["status"])
	}
}

func TestReadyDuringShutdown(t *testing.T) {
	srv, _ := testServer()
	srv.NotifyShutdown()

	req := httptest.NewRequest("GET", "/ready", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

// --- Rate Limiting Tests ---

func TestRateLimiting(t *testing.T) {
	ms := newMockStore()
	cfg := &config.Config{
		APIKey:        "test-api-key",
		SigningSecret: "test-secret",
		Port:          4000,
		RateLimitRPS:  5,
		RateLimitBurst: 5,
	}
	srv := NewServer(cfg, ms)

	got429 := false
	for i := 0; i < 20; i++ {
		w := doRequest(srv, "GET", "/api/events", nil)
		if w.Code == http.StatusTooManyRequests {
			got429 = true
			break
		}
	}
	if !got429 {
		t.Error("expected rate limiting to return 429")
	}
}

func TestRateLimitingDisabled(t *testing.T) {
	ms := newMockStore()
	cfg := &config.Config{
		APIKey:        "test-api-key",
		SigningSecret: "test-secret",
		Port:          4000,
		RateLimitRPS:  0,
	}
	srv := NewServer(cfg, ms)

	for i := 0; i < 20; i++ {
		w := doRequest(srv, "GET", "/api/events", nil)
		if w.Code == http.StatusTooManyRequests {
			t.Error("rate limiting should be disabled when RateLimitRPS=0")
			break
		}
	}
}
