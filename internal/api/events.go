package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/david-ouatedem/hookman/internal/id"
	"github.com/david-ouatedem/hookman/internal/metrics"
	"github.com/david-ouatedem/hookman/internal/store"
	"github.com/david-ouatedem/hookman/internal/store/queries"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type createEventRequest struct {
	Topic          string          `json:"topic"`
	Payload        json.RawMessage `json:"payload"`
	IdempotencyKey *string         `json:"idempotencyKey,omitempty"`
}

type eventResponse struct {
	ID        string          `json:"id"`
	Topic     string          `json:"topic"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	Status    string          `json:"status"`
	CreatedAt string          `json:"createdAt"`
}

type eventDetailResponse struct {
	eventResponse
	DeliveryAttempts []deliveryAttemptResponse `json:"deliveryAttempts"`
}

type deliveryAttemptResponse struct {
	ID            string  `json:"id"`
	EndpointID    string  `json:"endpointId"`
	AttemptNumber int32   `json:"attemptNumber"`
	HTTPStatus    *int32  `json:"httpStatus"`
	ResponseBody  *string `json:"responseBody,omitempty"`
	DurationMs    *int32  `json:"durationMs"`
	Status        string  `json:"status"`
	AttemptedAt   string  `json:"attemptedAt"`
	NextRetryAt   *string `json:"nextRetryAt,omitempty"`
}

func (s *Server) handleCreateEvent(w http.ResponseWriter, r *http.Request) {
	var req createEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if req.Topic == "" {
		writeError(w, http.StatusBadRequest, "topic is required")
		return
	}
	if len(req.Payload) == 0 {
		writeError(w, http.StatusBadRequest, "payload is required")
		return
	}

	// Check idempotency
	if req.IdempotencyKey != nil && *req.IdempotencyKey != "" {
		existing, err := s.store.GetEventByIdempotencyKey(r.Context(), pgtype.Text{String: *req.IdempotencyKey, Valid: true})
		if err == nil {
			writeJSON(w, http.StatusOK, eventResponse{
				ID:        existing.ID,
				Topic:     existing.Topic,
				Status:    existing.Status,
				CreatedAt: existing.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
			})
			return
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			slog.Error("failed to check idempotency key", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}

	eventID := id.NewEvent()
	params := queries.CreateEventParams{
		ID:      eventID,
		Topic:   req.Topic,
		Payload: req.Payload,
		Status:  "pending",
	}
	if req.IdempotencyKey != nil && *req.IdempotencyKey != "" {
		params.IdempotencyKey = pgtype.Text{String: *req.IdempotencyKey, Valid: true}
	}

	if err := s.store.CreateEvent(r.Context(), params); err != nil {
		slog.Error("failed to create event", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create event")
		return
	}

	metrics.RecordEventCreated(req.Topic)

	writeJSON(w, http.StatusCreated, eventResponse{
		ID:     eventID,
		Topic:  req.Topic,
		Status: "pending",
	})
}

func (s *Server) handleListEvents(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}

	params := queries.ListEventsParams{
		LimitCount: int32(limit),
	}

	if topic := r.URL.Query().Get("topic"); topic != "" {
		params.Topic = pgtype.Text{String: topic, Valid: true}
	}
	if status := r.URL.Query().Get("status"); status != "" {
		params.Status = pgtype.Text{String: status, Valid: true}
	}
	if before := r.URL.Query().Get("before"); before != "" {
		params.BeforeID = pgtype.Text{String: before, Valid: true}
	}

	events, err := s.store.ListEvents(r.Context(), params)
	if err != nil {
		slog.Error("failed to list events", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list events")
		return
	}

	resp := make([]eventResponse, len(events))
	for i, e := range events {
		resp[i] = eventResponse{
			ID:        e.ID,
			Topic:     e.Topic,
			Payload:   e.Payload,
			Status:    e.Status,
			CreatedAt: e.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleGetEvent(w http.ResponseWriter, r *http.Request) {
	eventID := chi.URLParam(r, "id")

	event, err := s.store.GetEvent(r.Context(), eventID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "event not found")
			return
		}
		slog.Error("failed to get event", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	attempts, err := s.store.GetDeliveryAttemptsByEvent(r.Context(), eventID)
	if err != nil {
		slog.Error("failed to get delivery attempts", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	resp := eventDetailResponse{
		eventResponse: eventResponse{
			ID:        event.ID,
			Topic:     event.Topic,
			Payload:   event.Payload,
			Status:    event.Status,
			CreatedAt: event.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
		},
		DeliveryAttempts: make([]deliveryAttemptResponse, len(attempts)),
	}

	for i, a := range attempts {
		dar := deliveryAttemptResponse{
			ID:            a.ID,
			EndpointID:    a.EndpointID,
			AttemptNumber: a.AttemptNumber,
			Status:        a.Status,
			AttemptedAt:   a.AttemptedAt.Time.Format("2006-01-02T15:04:05Z"),
		}
		if a.HttpStatus.Valid {
			status := a.HttpStatus.Int32
			dar.HTTPStatus = &status
		}
		if a.ResponseBody.Valid {
			body := a.ResponseBody.String
			dar.ResponseBody = &body
		}
		if a.DurationMs.Valid {
			dur := a.DurationMs.Int32
			dar.DurationMs = &dur
		}
		if a.NextRetryAt.Valid {
			t := a.NextRetryAt.Time.Format("2006-01-02T15:04:05Z")
			dar.NextRetryAt = &t
		}
		resp.DeliveryAttempts[i] = dar
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleReplayEvent(w http.ResponseWriter, r *http.Request) {
	eventID := chi.URLParam(r, "id")

	_, err := s.store.GetEvent(r.Context(), eventID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "event not found")
			return
		}
		slog.Error("failed to get event for replay", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if err := s.store.UpdateEventStatus(r.Context(), store.UpdateStatusParams(eventID, "pending")); err != nil {
		slog.Error("failed to reset event status for replay", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to replay event")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"id":     eventID,
		"status": "pending",
	})
}
