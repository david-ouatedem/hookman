package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/david-ouatedem/hookman/internal/store/queries"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// mockStore implements store.Store for dashboard testing.
type mockStore struct {
	events    map[string]queries.Event
	endpoints map[string]queries.Endpoint
}

func newMockStore() *mockStore {
	return &mockStore{
		events:    make(map[string]queries.Event),
		endpoints: make(map[string]queries.Endpoint),
	}
}

func (m *mockStore) CreateEvent(_ context.Context, p queries.CreateEventParams) error {
	m.events[p.ID] = queries.Event{ID: p.ID, Topic: p.Topic, Payload: p.Payload, Status: p.Status, CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}}
	return nil
}
func (m *mockStore) GetEvent(_ context.Context, id string) (queries.Event, error) {
	e, ok := m.events[id]
	if !ok {
		return queries.Event{}, pgx.ErrNoRows
	}
	return e, nil
}
func (m *mockStore) GetEventByIdempotencyKey(_ context.Context, _ pgtype.Text) (queries.Event, error) {
	return queries.Event{}, pgx.ErrNoRows
}
func (m *mockStore) ListEvents(_ context.Context, _ queries.ListEventsParams) ([]queries.Event, error) {
	r := make([]queries.Event, 0, len(m.events))
	for _, e := range m.events {
		r = append(r, e)
	}
	return r, nil
}
func (m *mockStore) UpdateEventStatus(_ context.Context, p queries.UpdateEventStatusParams) error {
	e := m.events[p.ID]
	e.Status = p.Status
	m.events[p.ID] = e
	return nil
}
func (m *mockStore) GetPendingEvents(_ context.Context, _ int32) ([]queries.Event, error) {
	return nil, nil
}
func (m *mockStore) GetDeadEvents(_ context.Context, _ queries.GetDeadEventsParams) ([]queries.Event, error) {
	return nil, nil
}
func (m *mockStore) CountEventsByStatus(_ context.Context) ([]queries.CountEventsByStatusRow, error) {
	counts := make(map[string]int64)
	for _, e := range m.events {
		counts[e.Status]++
	}
	var r []queries.CountEventsByStatusRow
	for s, c := range counts {
		r = append(r, queries.CountEventsByStatusRow{Status: s, Count: c})
	}
	return r, nil
}
func (m *mockStore) BulkReplayDeadEvents(_ context.Context) (int64, error) { return 0, nil }
func (m *mockStore) PurgeDeadEvents(_ context.Context) (int64, error)      { return 0, nil }
func (m *mockStore) CreateEndpoint(_ context.Context, p queries.CreateEndpointParams) error {
	m.endpoints[p.ID] = queries.Endpoint{ID: p.ID, Url: p.Url, Topics: p.Topics, Enabled: p.Enabled, CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}}
	return nil
}
func (m *mockStore) GetEndpoint(_ context.Context, id string) (queries.Endpoint, error) {
	e, ok := m.endpoints[id]
	if !ok {
		return queries.Endpoint{}, pgx.ErrNoRows
	}
	return e, nil
}
func (m *mockStore) ListEndpoints(_ context.Context) ([]queries.Endpoint, error) {
	r := make([]queries.Endpoint, 0, len(m.endpoints))
	for _, e := range m.endpoints {
		r = append(r, e)
	}
	return r, nil
}
func (m *mockStore) UpdateEndpoint(_ context.Context, p queries.UpdateEndpointParams) error {
	e := m.endpoints[p.ID]
	if p.Enabled.Valid {
		e.Enabled = p.Enabled.Bool
	}
	m.endpoints[p.ID] = e
	return nil
}
func (m *mockStore) DeleteEndpoint(_ context.Context, id string) error {
	delete(m.endpoints, id)
	return nil
}
func (m *mockStore) GetEndpointsByTopic(_ context.Context, _ string) ([]queries.Endpoint, error) {
	return nil, nil
}
func (m *mockStore) CreateDeliveryAttempt(_ context.Context, _ queries.CreateDeliveryAttemptParams) error {
	return nil
}
func (m *mockStore) GetDeliveryAttemptsByEvent(_ context.Context, _ string) ([]queries.DeliveryAttempt, error) {
	return nil, nil
}
func (m *mockStore) GetDeliveryAttemptsByEndpoint(_ context.Context, _ queries.GetDeliveryAttemptsByEndpointParams) ([]queries.DeliveryAttempt, error) {
	return nil, nil
}
func (m *mockStore) GetRetryableAttempts(_ context.Context, _ time.Time, _ int32) ([]queries.DeliveryAttempt, error) {
	return nil, nil
}

func testDashboard() (*Dashboard, chi.Router) {
	ms := newMockStore()
	ms.events["evt_1"] = queries.Event{
		ID: "evt_1", Topic: "test.topic", Payload: []byte(`{"key":"value"}`),
		Status: "delivered", CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
		UpdatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	ms.events["evt_2"] = queries.Event{
		ID: "evt_2", Topic: "test.topic", Payload: []byte(`{"key":"value2"}`),
		Status: "dead", CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
		UpdatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}

	d := NewDashboard(ms)
	r := chi.NewRouter()
	d.RegisterRoutes(r)
	return d, r
}

func TestDashboardStats(t *testing.T) {
	_, r := testDashboard()
	req := httptest.NewRequest("GET", "/dashboard/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("expected text/html, got %s", ct)
	}
}

func TestDashboardEvents(t *testing.T) {
	_, r := testDashboard()
	req := httptest.NewRequest("GET", "/dashboard/events", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if len(body) == 0 {
		t.Error("expected non-empty response body")
	}
}

func TestDashboardEventDetail(t *testing.T) {
	_, r := testDashboard()
	req := httptest.NewRequest("GET", "/dashboard/events/evt_1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestDashboardEventDetail_NotFound(t *testing.T) {
	_, r := testDashboard()
	req := httptest.NewRequest("GET", "/dashboard/events/evt_missing", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestDashboardEndpoints(t *testing.T) {
	_, r := testDashboard()
	req := httptest.NewRequest("GET", "/dashboard/endpoints", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestDashboardPartialEvents(t *testing.T) {
	_, r := testDashboard()
	req := httptest.NewRequest("GET", "/dashboard/partials/events", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}
