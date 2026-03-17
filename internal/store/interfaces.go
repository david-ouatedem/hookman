package store

import (
	"context"
	"time"

	"github.com/david-ouatedem/hookman/internal/store/queries"
	"github.com/jackc/pgx/v5/pgtype"
)

// UpdateStatusParams is a convenience constructor for queries.UpdateEventStatusParams.
func UpdateStatusParams(id, status string) queries.UpdateEventStatusParams {
	return queries.UpdateEventStatusParams{ID: id, Status: status}
}

// Store combines all data access operations.
type Store interface {
	EventStore
	EndpointStore
	DeliveryStore
}

// EventStore defines operations on events.
type EventStore interface {
	CreateEvent(ctx context.Context, params queries.CreateEventParams) error
	GetEvent(ctx context.Context, id string) (queries.Event, error)
	GetEventByIdempotencyKey(ctx context.Context, key pgtype.Text) (queries.Event, error)
	ListEvents(ctx context.Context, params queries.ListEventsParams) ([]queries.Event, error)
	UpdateEventStatus(ctx context.Context, params queries.UpdateEventStatusParams) error
	GetPendingEvents(ctx context.Context, limit int32) ([]queries.Event, error)
}

// EndpointStore defines operations on subscriber endpoints.
type EndpointStore interface {
	CreateEndpoint(ctx context.Context, params queries.CreateEndpointParams) error
	GetEndpoint(ctx context.Context, id string) (queries.Endpoint, error)
	ListEndpoints(ctx context.Context) ([]queries.Endpoint, error)
	UpdateEndpoint(ctx context.Context, params queries.UpdateEndpointParams) error
	DeleteEndpoint(ctx context.Context, id string) error
	GetEndpointsByTopic(ctx context.Context, topic string) ([]queries.Endpoint, error)
}

// DeliveryStore defines operations on delivery attempts.
type DeliveryStore interface {
	CreateDeliveryAttempt(ctx context.Context, params queries.CreateDeliveryAttemptParams) error
	GetDeliveryAttemptsByEvent(ctx context.Context, eventID string) ([]queries.DeliveryAttempt, error)
	GetDeliveryAttemptsByEndpoint(ctx context.Context, params queries.GetDeliveryAttemptsByEndpointParams) ([]queries.DeliveryAttempt, error)
	GetRetryableAttempts(ctx context.Context, before time.Time, limit int32) ([]queries.DeliveryAttempt, error)
}
