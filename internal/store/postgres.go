package store

import (
	"context"
	"time"

	"github.com/david-ouatedem/hookman/internal/store/queries"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore implements Store using Postgres via sqlc-generated queries.
type PostgresStore struct {
	pool *pgxpool.Pool
	q    *queries.Queries
}

// NewPostgresStore creates a new PostgresStore backed by the given connection pool.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{
		pool: pool,
		q:    queries.New(pool),
	}
}

// --- EventStore ---

func (s *PostgresStore) CreateEvent(ctx context.Context, params queries.CreateEventParams) error {
	return s.q.CreateEvent(ctx, params)
}

func (s *PostgresStore) GetEvent(ctx context.Context, id string) (queries.Event, error) {
	return s.q.GetEvent(ctx, id)
}

func (s *PostgresStore) GetEventByIdempotencyKey(ctx context.Context, key pgtype.Text) (queries.Event, error) {
	return s.q.GetEventByIdempotencyKey(ctx, key)
}

func (s *PostgresStore) ListEvents(ctx context.Context, params queries.ListEventsParams) ([]queries.Event, error) {
	return s.q.ListEvents(ctx, params)
}

func (s *PostgresStore) UpdateEventStatus(ctx context.Context, params queries.UpdateEventStatusParams) error {
	return s.q.UpdateEventStatus(ctx, params)
}

func (s *PostgresStore) GetPendingEvents(ctx context.Context, limit int32) ([]queries.Event, error) {
	return s.q.GetPendingEvents(ctx, limit)
}

func (s *PostgresStore) GetDeadEvents(ctx context.Context, params queries.GetDeadEventsParams) ([]queries.Event, error) {
	return s.q.GetDeadEvents(ctx, params)
}

func (s *PostgresStore) CountEventsByStatus(ctx context.Context) ([]queries.CountEventsByStatusRow, error) {
	return s.q.CountEventsByStatus(ctx)
}

func (s *PostgresStore) BulkReplayDeadEvents(ctx context.Context) (int64, error) {
	return s.q.BulkReplayDeadEvents(ctx)
}

func (s *PostgresStore) PurgeDeadEvents(ctx context.Context) (int64, error) {
	return s.q.PurgeDeadEvents(ctx)
}

// --- EndpointStore ---

func (s *PostgresStore) CreateEndpoint(ctx context.Context, params queries.CreateEndpointParams) error {
	return s.q.CreateEndpoint(ctx, params)
}

func (s *PostgresStore) GetEndpoint(ctx context.Context, id string) (queries.Endpoint, error) {
	return s.q.GetEndpoint(ctx, id)
}

func (s *PostgresStore) ListEndpoints(ctx context.Context) ([]queries.Endpoint, error) {
	return s.q.ListEndpoints(ctx)
}

func (s *PostgresStore) UpdateEndpoint(ctx context.Context, params queries.UpdateEndpointParams) error {
	return s.q.UpdateEndpoint(ctx, params)
}

func (s *PostgresStore) DeleteEndpoint(ctx context.Context, id string) error {
	return s.q.DeleteEndpoint(ctx, id)
}

func (s *PostgresStore) GetEndpointsByTopic(ctx context.Context, topic string) ([]queries.Endpoint, error) {
	return s.q.GetEndpointsByTopic(ctx, []string{topic})
}

// --- DeliveryStore ---

func (s *PostgresStore) CreateDeliveryAttempt(ctx context.Context, params queries.CreateDeliveryAttemptParams) error {
	return s.q.CreateDeliveryAttempt(ctx, params)
}

func (s *PostgresStore) GetDeliveryAttemptsByEvent(ctx context.Context, eventID string) ([]queries.DeliveryAttempt, error) {
	return s.q.GetDeliveryAttemptsByEvent(ctx, eventID)
}

func (s *PostgresStore) GetDeliveryAttemptsByEndpoint(ctx context.Context, params queries.GetDeliveryAttemptsByEndpointParams) ([]queries.DeliveryAttempt, error) {
	return s.q.GetDeliveryAttemptsByEndpoint(ctx, params)
}

func (s *PostgresStore) GetRetryableAttempts(ctx context.Context, before time.Time, limit int32) ([]queries.DeliveryAttempt, error) {
	return s.q.GetRetryableAttempts(ctx, queries.GetRetryableAttemptsParams{
		NextRetryAt: pgtype.Timestamptz{Time: before, Valid: true},
		Limit:       limit,
	})
}
