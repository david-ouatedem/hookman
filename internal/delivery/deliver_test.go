package delivery

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDeliver_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Hookman-Signature") == "" {
			t.Error("missing signature header")
		}
		if r.Header.Get("X-Hookman-Event-Id") != "evt_123" {
			t.Errorf("unexpected event ID header: %s", r.Header.Get("X-Hookman-Event-Id"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("unexpected content type: %s", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"received":true}`))
	}))
	defer server.Close()

	job := DeliveryJob{
		EventID:       "evt_123",
		EndpointID:    "ep_456",
		EndpointURL:   server.URL,
		SigningSecret: "test-secret",
		Payload:       []byte(`{"amount":5000}`),
		AttemptNumber: 1,
		MaxRetries:    5,
	}

	result := Deliver(context.Background(), server.Client(), job)

	if !result.Success {
		t.Error("expected successful delivery")
	}
	if result.HTTPStatus == nil || *result.HTTPStatus != 200 {
		t.Errorf("expected HTTP 200, got %v", result.HTTPStatus)
	}
	if result.NextRetryAt != nil {
		t.Error("successful delivery should not have next retry")
	}
	if result.DurationMs < 0 {
		t.Error("duration should be non-negative")
	}
}

func TestDeliver_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("Service Unavailable"))
	}))
	defer server.Close()

	job := DeliveryJob{
		EventID:       "evt_123",
		EndpointID:    "ep_456",
		EndpointURL:   server.URL,
		SigningSecret: "test-secret",
		Payload:       []byte(`{"test":true}`),
		AttemptNumber: 1,
		MaxRetries:    5,
	}

	result := Deliver(context.Background(), server.Client(), job)

	if result.Success {
		t.Error("expected failed delivery")
	}
	if result.HTTPStatus == nil || *result.HTTPStatus != 503 {
		t.Errorf("expected HTTP 503, got %v", result.HTTPStatus)
	}
	if result.NextRetryAt == nil {
		t.Error("failed delivery should have next retry time")
	}
}

func TestDeliver_ConnectionRefused(t *testing.T) {
	job := DeliveryJob{
		EventID:       "evt_123",
		EndpointID:    "ep_456",
		EndpointURL:   "http://localhost:19999",
		SigningSecret: "test-secret",
		Payload:       []byte(`{"test":true}`),
		AttemptNumber: 1,
		MaxRetries:    5,
	}

	result := Deliver(context.Background(), &http.Client{}, job)

	if result.Success {
		t.Error("expected failed delivery on connection refused")
	}
	if result.HTTPStatus != nil {
		t.Error("connection refused should have nil HTTP status")
	}
	if result.NextRetryAt == nil {
		t.Error("connection refused should schedule retry")
	}
}

func TestDeliver_LastAttemptExhausted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	job := DeliveryJob{
		EventID:       "evt_123",
		EndpointID:    "ep_456",
		EndpointURL:   server.URL,
		SigningSecret: "test-secret",
		Payload:       []byte(`{"test":true}`),
		AttemptNumber: 5,
		MaxRetries:    5,
	}

	result := Deliver(context.Background(), server.Client(), job)

	if result.Success {
		t.Error("expected failed delivery")
	}
	if result.NextRetryAt != nil {
		t.Error("exhausted retries should have nil next retry (dead letter)")
	}
}
