package api

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/david-ouatedem/hookman/internal/store/queries"
	"github.com/jackc/pgx/v5/pgtype"
)

func (s *Server) handleListDeadEvents(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}

	params := queries.GetDeadEventsParams{
		LimitCount: int32(limit),
	}
	if before := r.URL.Query().Get("before"); before != "" {
		params.BeforeID = pgtype.Text{String: before, Valid: true}
	}

	events, err := s.store.GetDeadEvents(r.Context(), params)
	if err != nil {
		slog.Error("failed to list dead events", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list dead events")
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

func (s *Server) handleBulkReplayDeadEvents(w http.ResponseWriter, r *http.Request) {
	count, err := s.store.BulkReplayDeadEvents(r.Context())
	if err != nil {
		slog.Error("failed to bulk replay dead events", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to replay dead events")
		return
	}

	slog.Info("bulk replayed dead events", "count", count)
	writeJSON(w, http.StatusOK, map[string]any{
		"replayed": count,
		"status":   "pending",
	})
}

func (s *Server) handlePurgeDeadEvents(w http.ResponseWriter, r *http.Request) {
	count, err := s.store.PurgeDeadEvents(r.Context())
	if err != nil {
		slog.Error("failed to purge dead events", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to purge dead events")
		return
	}

	slog.Info("purged dead events", "count", count)
	writeJSON(w, http.StatusOK, map[string]any{
		"purged": count,
	})
}
