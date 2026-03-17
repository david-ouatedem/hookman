package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/david-ouatedem/hookman/internal/id"
	"github.com/david-ouatedem/hookman/internal/store/queries"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type createEndpointRequest struct {
	URL         string   `json:"url"`
	Topics      []string `json:"topics"`
	Description *string  `json:"description,omitempty"`
}

type endpointResponse struct {
	ID            string   `json:"id"`
	URL           string   `json:"url"`
	Topics        []string `json:"topics"`
	Description   *string  `json:"description,omitempty"`
	SigningSecret string   `json:"signingSecret,omitempty"`
	Enabled       bool     `json:"enabled"`
	CreatedAt     string   `json:"createdAt"`
}

type updateEndpointRequest struct {
	URL         *string  `json:"url,omitempty"`
	Topics      []string `json:"topics,omitempty"`
	Description *string  `json:"description,omitempty"`
	Enabled     *bool    `json:"enabled,omitempty"`
}

func (s *Server) handleCreateEndpoint(w http.ResponseWriter, r *http.Request) {
	var req createEndpointRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if req.URL == "" {
		writeError(w, http.StatusBadRequest, "url is required")
		return
	}
	if _, err := url.ParseRequestURI(req.URL); err != nil {
		writeError(w, http.StatusBadRequest, "invalid url")
		return
	}
	if len(req.Topics) == 0 {
		writeError(w, http.StatusBadRequest, "at least one topic is required")
		return
	}

	signingSecret := generateSigningSecret()
	endpointID := id.NewEndpoint()

	params := queries.CreateEndpointParams{
		ID:            endpointID,
		Url:           req.URL,
		Topics:        req.Topics,
		SigningSecret: signingSecret,
		Enabled:       true,
	}
	if req.Description != nil {
		params.Description = pgtype.Text{String: *req.Description, Valid: true}
	}

	if err := s.store.CreateEndpoint(r.Context(), params); err != nil {
		slog.Error("failed to create endpoint", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create endpoint")
		return
	}

	writeJSON(w, http.StatusCreated, endpointResponse{
		ID:            endpointID,
		URL:           req.URL,
		Topics:        req.Topics,
		SigningSecret: signingSecret,
		Enabled:       true,
	})
}

func (s *Server) handleListEndpoints(w http.ResponseWriter, r *http.Request) {
	endpoints, err := s.store.ListEndpoints(r.Context())
	if err != nil {
		slog.Error("failed to list endpoints", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list endpoints")
		return
	}

	resp := make([]endpointResponse, len(endpoints))
	for i, ep := range endpoints {
		resp[i] = endpointResponse{
			ID:      ep.ID,
			URL:     ep.Url,
			Topics:  ep.Topics,
			Enabled: ep.Enabled,
			CreatedAt: ep.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
		}
		if ep.Description.Valid {
			desc := ep.Description.String
			resp[i].Description = &desc
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleUpdateEndpoint(w http.ResponseWriter, r *http.Request) {
	endpointID := chi.URLParam(r, "id")

	_, err := s.store.GetEndpoint(r.Context(), endpointID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "endpoint not found")
			return
		}
		slog.Error("failed to get endpoint", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	var req updateEndpointRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	params := queries.UpdateEndpointParams{
		ID: endpointID,
	}
	if req.URL != nil {
		params.Url = pgtype.Text{String: *req.URL, Valid: true}
	}
	if req.Topics != nil {
		params.Topics = req.Topics
	}
	if req.Description != nil {
		params.Description = pgtype.Text{String: *req.Description, Valid: true}
	}
	if req.Enabled != nil {
		params.Enabled = pgtype.Bool{Bool: *req.Enabled, Valid: true}
	}

	if err := s.store.UpdateEndpoint(r.Context(), params); err != nil {
		slog.Error("failed to update endpoint", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update endpoint")
		return
	}

	// Return the updated endpoint
	updated, err := s.store.GetEndpoint(r.Context(), endpointID)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]string{"id": endpointID, "status": "updated"})
		return
	}

	resp := endpointResponse{
		ID:      updated.ID,
		URL:     updated.Url,
		Topics:  updated.Topics,
		Enabled: updated.Enabled,
		CreatedAt: updated.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
	}
	if updated.Description.Valid {
		desc := updated.Description.String
		resp.Description = &desc
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleDeleteEndpoint(w http.ResponseWriter, r *http.Request) {
	endpointID := chi.URLParam(r, "id")

	_, err := s.store.GetEndpoint(r.Context(), endpointID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "endpoint not found")
			return
		}
		slog.Error("failed to get endpoint", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if err := s.store.DeleteEndpoint(r.Context(), endpointID); err != nil {
		slog.Error("failed to delete endpoint", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete endpoint")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleEndpointDeliveries(w http.ResponseWriter, r *http.Request) {
	endpointID := chi.URLParam(r, "id")

	_, err := s.store.GetEndpoint(r.Context(), endpointID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "endpoint not found")
			return
		}
		slog.Error("failed to get endpoint", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	attempts, err := s.store.GetDeliveryAttemptsByEndpoint(r.Context(), queries.GetDeliveryAttemptsByEndpointParams{
		EndpointID: endpointID,
		Limit:      50,
	})
	if err != nil {
		slog.Error("failed to get endpoint deliveries", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	resp := make([]deliveryAttemptResponse, len(attempts))
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
		if a.DurationMs.Valid {
			dur := a.DurationMs.Int32
			dar.DurationMs = &dur
		}
		resp[i] = dar
	}

	writeJSON(w, http.StatusOK, resp)
}

func generateSigningSecret() string {
	b := make([]byte, 32)
	rand.Read(b)
	return "whsec_" + hex.EncodeToString(b)
}
