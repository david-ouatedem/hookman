package store

import (
	"context"
	"time"
)

// Event represents a webhook event.
type Event struct {
	ID             string
	Topic          string
	Payload        []byte // raw JSON
	IdempotencyKey *string
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Endpoint represents a subscriber endpoint.
type Endpoint struct {
	ID            string
	URL           string
	Topics        []string
	Description   *string
	SigningSecret  string
	Enabled       bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// DeliveryAttempt represents a single delivery attempt for an event to an endpoint.
type DeliveryAttempt struct {
	ID            string
	EventID       string
	EndpointID    string
	AttemptNumber int
	HTTPStatus    *int
	ResponseBody  *string
	DurationMs    *int
	Status        string
	AttemptedAt   time.Time
	NextRetryAt   *time.Time
}

// EventStore defines operations on events.
type EventStore interface {
	CreateEvent(ctx context.Context, event *Event) error
	GetEvent(ctx context.Context, id string) (*Event, error)
	ListEvents(ctx context.Context, opts ListEventsOpts) ([]Event, error)
	UpdateEventStatus(ctx context.Context, id string, status string) error
	GetPendingEvents(ctx context.Context, limit int) ([]Event, error)
}

// EndpointStore defines operations on subscriber endpoints.
type EndpointStore interface {
	CreateEndpoint(ctx context.Context, endpoint *Endpoint) error
	GetEndpoint(ctx context.Context, id string) (*Endpoint, error)
	ListEndpoints(ctx context.Context) ([]Endpoint, error)
	UpdateEndpoint(ctx context.Context, id string, updates EndpointUpdates) error
	DeleteEndpoint(ctx context.Context, id string) error
	GetEndpointsByTopic(ctx context.Context, topic string) ([]Endpoint, error)
}

// DeliveryStore defines operations on delivery attempts.
type DeliveryStore interface {
	CreateDeliveryAttempt(ctx context.Context, attempt *DeliveryAttempt) error
	GetDeliveryAttemptsByEvent(ctx context.Context, eventID string) ([]DeliveryAttempt, error)
	GetDeliveryAttemptsByEndpoint(ctx context.Context, endpointID string, limit int) ([]DeliveryAttempt, error)
	GetRetryableAttempts(ctx context.Context, before time.Time, limit int) ([]DeliveryAttempt, error)
}

// ListEventsOpts contains optional filters for listing events.
type ListEventsOpts struct {
	Topic  *string
	Status *string
	Limit  int
	Before *string // cursor (event ID)
}

// EndpointUpdates contains fields that can be updated on an endpoint.
type EndpointUpdates struct {
	URL         *string
	Topics      []string
	Description *string
	Enabled     *bool
}
