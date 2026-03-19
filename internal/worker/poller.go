package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/david-ouatedem/hookman/internal/delivery"
	"github.com/david-ouatedem/hookman/internal/metrics"
	"github.com/david-ouatedem/hookman/internal/store"
)

// Poller periodically checks the database for pending events and retryable
// delivery attempts, then feeds them into the worker pool's job channel.
type Poller struct {
	store        store.Store
	interval     time.Duration
	maxRetries   int
	jobs         chan<- delivery.DeliveryJob
	batchSize    int32
}

// NewPoller creates a new poller.
func NewPoller(store store.Store, interval time.Duration, maxRetries int, jobs chan<- delivery.DeliveryJob) *Poller {
	return &Poller{
		store:      store,
		interval:   interval,
		maxRetries: maxRetries,
		jobs:       jobs,
		batchSize:  50,
	}
}

// Start runs the polling loop until ctx is cancelled.
func (p *Poller) Start(ctx context.Context) {
	slog.Info("poller started", "interval", p.interval)

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("poller stopped")
			return
		case <-ticker.C:
			p.pollPendingEvents(ctx)
			p.pollRetryableAttempts(ctx)
		}
	}
}

func (p *Poller) pollPendingEvents(ctx context.Context) {
	events, err := p.store.GetPendingEvents(ctx, p.batchSize)
	if err != nil {
		slog.Error("poller: failed to get pending events", "error", err)
		return
	}

	metrics.RecordQueueDepth("pending", len(events))

	for _, event := range events {
		endpoints, err := p.store.GetEndpointsByTopic(ctx, event.Topic)
		if err != nil {
			slog.Error("poller: failed to get endpoints for topic",
				"topic", event.Topic,
				"error", err,
			)
			continue
		}

		if len(endpoints) == 0 {
			slog.Debug("poller: no endpoints for topic, marking delivered",
				"event_id", event.ID,
				"topic", event.Topic,
			)
			continue
		}

		// Mark event as processing
		if err := p.store.UpdateEventStatus(ctx, store.UpdateStatusParams(event.ID, "processing")); err != nil {
			slog.Error("poller: failed to update event status", "event_id", event.ID, "error", err)
			continue
		}

		for _, ep := range endpoints {
			job := delivery.DeliveryJob{
				EventID:       event.ID,
				EndpointID:    ep.ID,
				EndpointURL:   ep.Url,
				SigningSecret: ep.SigningSecret,
				Payload:       event.Payload,
				AttemptNumber: 1,
				MaxRetries:    p.maxRetries,
			}

			select {
			case p.jobs <- job:
			case <-ctx.Done():
				return
			}
		}
	}
}

func (p *Poller) pollRetryableAttempts(ctx context.Context) {
	attempts, err := p.store.GetRetryableAttempts(ctx, time.Now(), p.batchSize)
	if err != nil {
		slog.Error("poller: failed to get retryable attempts", "error", err)
		return
	}

	for _, attempt := range attempts {
		event, err := p.store.GetEvent(ctx, attempt.EventID)
		if err != nil {
			slog.Error("poller: failed to get event for retry",
				"event_id", attempt.EventID,
				"error", err,
			)
			continue
		}

		endpoint, err := p.store.GetEndpoint(ctx, attempt.EndpointID)
		if err != nil {
			slog.Error("poller: failed to get endpoint for retry",
				"endpoint_id", attempt.EndpointID,
				"error", err,
			)
			continue
		}

		job := delivery.DeliveryJob{
			EventID:       event.ID,
			EndpointID:    endpoint.ID,
			EndpointURL:   endpoint.Url,
			SigningSecret: endpoint.SigningSecret,
			Payload:       event.Payload,
			AttemptNumber: int(attempt.AttemptNumber) + 1,
			MaxRetries:    p.maxRetries,
		}

		select {
		case p.jobs <- job:
		case <-ctx.Done():
			return
		}
	}
}
