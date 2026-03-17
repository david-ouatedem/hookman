package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/david-ouatedem/hookman/internal/config"
	"github.com/david-ouatedem/hookman/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Server holds the HTTP server and its dependencies.
type Server struct {
	cfg    *config.Config
	store  store.Store
	router chi.Router
	http   *http.Server
}

// NewServer creates a new API server.
func NewServer(cfg *config.Config, s store.Store) *Server {
	srv := &Server{
		cfg:   cfg,
		store: s,
	}
	srv.router = srv.buildRouter()
	srv.http = &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: srv.router,
	}
	return srv
}

func (s *Server) buildRouter() chi.Router {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	// Health check (no auth)
	r.Get("/health", s.handleHealth)

	// API routes (auth required)
	r.Route("/api", func(r chi.Router) {
		r.Use(s.authMiddleware)

		// Events
		r.Post("/events", s.handleCreateEvent)
		r.Get("/events", s.handleListEvents)
		r.Get("/events/{id}", s.handleGetEvent)
		r.Post("/events/{id}/replay", s.handleReplayEvent)

		// Endpoints
		r.Post("/endpoints", s.handleCreateEndpoint)
		r.Get("/endpoints", s.handleListEndpoints)
		r.Patch("/endpoints/{id}", s.handleUpdateEndpoint)
		r.Delete("/endpoints/{id}", s.handleDeleteEndpoint)
		r.Get("/endpoints/{id}/deliveries", s.handleEndpointDeliveries)
	})

	return r
}

// Start begins listening for HTTP requests.
func (s *Server) Start() error {
	slog.Info("HTTP server starting", "addr", s.http.Addr)
	return s.http.ListenAndServe()
}

// Shutdown gracefully shuts down the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}
