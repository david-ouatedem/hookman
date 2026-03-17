package delivery

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DeliveryJob represents a webhook delivery to execute.
type DeliveryJob struct {
	EventID       string
	EndpointID    string
	EndpointURL   string
	SigningSecret string
	Payload       []byte // raw JSON
	AttemptNumber int
	MaxRetries    int
}

// DeliveryResult holds the outcome of a delivery attempt.
type DeliveryResult struct {
	HTTPStatus   *int
	ResponseBody string
	DurationMs   int
	Success      bool
	NextRetryAt  *time.Time
}

// Deliver executes an HTTP POST to the endpoint URL with the signed payload.
func Deliver(ctx context.Context, client *http.Client, job DeliveryJob) DeliveryResult {
	start := time.Now()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, job.EndpointURL, bytes.NewReader(job.Payload))
	if err != nil {
		durationMs := int(time.Since(start).Milliseconds())
		return DeliveryResult{
			DurationMs:   durationMs,
			Success:      false,
			ResponseBody: fmt.Sprintf("failed to create request: %v", err),
			NextRetryAt:  NextRetryAt(job.AttemptNumber, job.MaxRetries),
		}
	}

	signature := Sign(job.Payload, job.SigningSecret)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hookman-Signature", signature)
	req.Header.Set("X-Hookman-Event-Id", job.EventID)
	req.Header.Set("User-Agent", "Hookman/0.1")

	resp, err := client.Do(req)
	durationMs := int(time.Since(start).Milliseconds())

	if err != nil {
		return DeliveryResult{
			DurationMs:   durationMs,
			Success:      false,
			ResponseBody: fmt.Sprintf("request failed: %v", err),
			NextRetryAt:  NextRetryAt(job.AttemptNumber, job.MaxRetries),
		}
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	status := resp.StatusCode

	success := status >= 200 && status < 300

	var nextRetryAt *time.Time
	if !success {
		nextRetryAt = NextRetryAt(job.AttemptNumber, job.MaxRetries)
	}

	return DeliveryResult{
		HTTPStatus:   &status,
		ResponseBody: string(body),
		DurationMs:   durationMs,
		Success:      success,
		NextRetryAt:  nextRetryAt,
	}
}
