package worker

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/david-ouatedem/hookman/internal/delivery"
	"github.com/david-ouatedem/hookman/internal/id"
	"github.com/david-ouatedem/hookman/internal/metrics"
	"github.com/david-ouatedem/hookman/internal/store"
	"github.com/david-ouatedem/hookman/internal/store/queries"
	"github.com/jackc/pgx/v5/pgtype"
)

// Pool manages a fixed set of goroutines that process delivery jobs.
type Pool struct {
	concurrency int
	maxRetries  int
	store       store.Store
	client      *http.Client
	jobs        chan delivery.DeliveryJob
	wg          sync.WaitGroup
}

// NewPool creates a new worker pool.
func NewPool(concurrency int, maxRetries int, store store.Store, timeout time.Duration) *Pool {
	return &Pool{
		concurrency: concurrency,
		maxRetries:  maxRetries,
		store:       store,
		client:      &http.Client{Timeout: timeout},
		jobs:        make(chan delivery.DeliveryJob, concurrency*2),
	}
}

// Jobs returns the job channel for the poller to send work into.
func (p *Pool) Jobs() chan<- delivery.DeliveryJob {
	return p.jobs
}

// Start launches the worker goroutines. They run until ctx is cancelled.
func (p *Pool) Start(ctx context.Context) {
	for i := 0; i < p.concurrency; i++ {
		p.wg.Add(1)
		go p.worker(ctx, i)
	}
	slog.Info("worker pool started", "concurrency", p.concurrency)
}

// Stop waits for all workers to finish processing their current jobs.
func (p *Pool) Stop() {
	close(p.jobs)
	p.wg.Wait()
	slog.Info("worker pool stopped")
}

func (p *Pool) worker(ctx context.Context, workerID int) {
	defer p.wg.Done()

	for job := range p.jobs {
		if ctx.Err() != nil {
			return
		}
		p.processJob(ctx, workerID, job)
	}
}

func (p *Pool) processJob(ctx context.Context, workerID int, job delivery.DeliveryJob) {
	logger := slog.With(
		"worker", workerID,
		"event_id", job.EventID,
		"endpoint_id", job.EndpointID,
		"attempt", job.AttemptNumber,
	)

	logger.Debug("delivering webhook")

	result := delivery.Deliver(ctx, p.client, job)

	// Record delivery attempt
	status := "failed"
	if result.Success {
		status = "success"
	}

	attemptParams := queries.CreateDeliveryAttemptParams{
		ID:            id.NewDeliveryAttempt(),
		EventID:       job.EventID,
		EndpointID:    job.EndpointID,
		AttemptNumber: int32(job.AttemptNumber),
		Status:        status,
		DurationMs:    pgtype.Int4{Int32: int32(result.DurationMs), Valid: true},
		ResponseBody:  pgtype.Text{String: result.ResponseBody, Valid: result.ResponseBody != ""},
	}
	if result.HTTPStatus != nil {
		attemptParams.HttpStatus = pgtype.Int4{Int32: int32(*result.HTTPStatus), Valid: true}
	}
	if result.NextRetryAt != nil {
		attemptParams.NextRetryAt = pgtype.Timestamptz{Time: *result.NextRetryAt, Valid: true}
	}

	if err := p.store.CreateDeliveryAttempt(ctx, attemptParams); err != nil {
		logger.Error("failed to record delivery attempt", "error", err)
		return
	}

	metrics.RecordDeliveryAttempt(status, result.DurationMs)

	// Update event status
	if result.Success {
		if err := p.store.UpdateEventStatus(ctx, queries.UpdateEventStatusParams{
			ID:     job.EventID,
			Status: "delivered",
		}); err != nil {
			logger.Error("failed to mark event delivered", "error", err)
		}
		logger.Info("webhook delivered", "http_status", *result.HTTPStatus, "duration_ms", result.DurationMs)
	} else if result.NextRetryAt == nil {
		// Retries exhausted → dead letter
		if err := p.store.UpdateEventStatus(ctx, queries.UpdateEventStatusParams{
			ID:     job.EventID,
			Status: "dead",
		}); err != nil {
			logger.Error("failed to mark event dead", "error", err)
		}
		metrics.RecordDeadLetter()
		logger.Warn("webhook dead-lettered after max retries",
			"http_status", result.HTTPStatus,
			"duration_ms", result.DurationMs,
		)
	} else {
		logger.Info("webhook delivery failed, retry scheduled",
			"http_status", result.HTTPStatus,
			"duration_ms", result.DurationMs,
			"next_retry_at", result.NextRetryAt,
		)
	}
}
