package dashboard

import (
	"bytes"
	"embed"
	"encoding/json"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/david-ouatedem/hookman/internal/id"
	"github.com/david-ouatedem/hookman/internal/store"
	"github.com/david-ouatedem/hookman/internal/store/queries"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

//go:embed templates/*.html templates/partials/*.html
var templateFS embed.FS

// Dashboard handles all dashboard HTTP routes.
type Dashboard struct {
	store store.Store
	tmpl  *template.Template
}

// NewDashboard creates a new Dashboard with parsed templates.
func NewDashboard(s store.Store) *Dashboard {
	funcMap := template.FuncMap{
		"formatTime": func(t pgtype.Timestamptz) string {
			if !t.Valid {
				return "—"
			}
			return t.Time.Format("2006-01-02 15:04:05")
		},
		"statusClass": func(status string) string {
			switch status {
			case "delivered", "success":
				return "status-success"
			case "pending", "processing":
				return "status-pending"
			case "dead":
				return "status-dead"
			default:
				return "status-failed"
			}
		},
		"formatJSON": func(data []byte) string {
			var buf bytes.Buffer
			if err := json.Indent(&buf, data, "", "  "); err != nil {
				return string(data)
			}
			return buf.String()
		},
		"joinTopics": func(topics []string) string {
			return strings.Join(topics, ", ")
		},
		"derefInt32": func(v pgtype.Int4) string {
			if !v.Valid {
				return "—"
			}
			return strconv.Itoa(int(v.Int32))
		},
		"derefText": func(v pgtype.Text) string {
			if !v.Valid {
				return ""
			}
			return v.String
		},
		"now": func() string {
			return time.Now().Format("2006-01-02 15:04:05")
		},
	}

	tmpl := template.Must(template.New("").Funcs(funcMap).ParseFS(templateFS,
		"templates/*.html",
		"templates/partials/*.html",
	))

	return &Dashboard{store: s, tmpl: tmpl}
}

// RegisterRoutes mounts all dashboard routes on the given router.
func (d *Dashboard) RegisterRoutes(r chi.Router) {
	r.Route("/dashboard", func(r chi.Router) {
		r.Get("/", d.handleStats)
		r.Get("/events", d.handleEvents)
		r.Get("/events/{id}", d.handleEventDetail)
		r.Post("/events/{id}/replay", d.handleReplayEvent)
		r.Post("/events/dead/replay", d.handleBulkReplay)
		r.Get("/endpoints", d.handleEndpoints)
		r.Post("/endpoints", d.handleCreateEndpoint)
		r.Post("/endpoints/{id}/toggle", d.handleToggleEndpoint)
		r.Delete("/endpoints/{id}", d.handleDeleteEndpoint)

		// HTMX partials
		r.Get("/partials/events", d.handleEventRows)
		r.Get("/partials/endpoints", d.handleEndpointRows)
	})
}

func (d *Dashboard) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := d.tmpl.ExecuteTemplate(w, name, data); err != nil {
		slog.Error("template render error", "template", name, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// --- Page Handlers ---

func (d *Dashboard) handleStats(w http.ResponseWriter, r *http.Request) {
	rows, err := d.store.CountEventsByStatus(r.Context())
	if err != nil {
		slog.Error("dashboard: failed to get stats", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	stats := map[string]int64{
		"pending": 0, "processing": 0, "delivered": 0, "failed": 0, "dead": 0,
	}
	var total int64
	for _, row := range rows {
		stats[row.Status] = row.Count
		total += row.Count
	}
	stats["total"] = total

	d.render(w, "stats.html", stats)
}

func (d *Dashboard) handleEvents(w http.ResponseWriter, r *http.Request) {
	events, params := d.fetchEvents(r)
	d.render(w, "events.html", map[string]any{
		"Events": events,
		"Topic":  params.Topic.String,
		"Status": params.Status.String,
	})
}

func (d *Dashboard) handleEventRows(w http.ResponseWriter, r *http.Request) {
	events, _ := d.fetchEvents(r)
	d.render(w, "event_rows.html", map[string]any{"Events": events})
}

func (d *Dashboard) fetchEvents(r *http.Request) ([]queries.Event, queries.ListEventsParams) {
	params := queries.ListEventsParams{LimitCount: 50}
	if topic := r.URL.Query().Get("topic"); topic != "" {
		params.Topic = pgtype.Text{String: topic, Valid: true}
	}
	if status := r.URL.Query().Get("status"); status != "" {
		params.Status = pgtype.Text{String: status, Valid: true}
	}
	if before := r.URL.Query().Get("before"); before != "" {
		params.BeforeID = pgtype.Text{String: before, Valid: true}
	}

	events, err := d.store.ListEvents(r.Context(), params)
	if err != nil {
		slog.Error("dashboard: failed to list events", "error", err)
		return nil, params
	}
	return events, params
}

func (d *Dashboard) handleEventDetail(w http.ResponseWriter, r *http.Request) {
	eventID := chi.URLParam(r, "id")

	event, err := d.store.GetEvent(r.Context(), eventID)
	if err != nil {
		http.Error(w, "event not found", http.StatusNotFound)
		return
	}

	attempts, err := d.store.GetDeliveryAttemptsByEvent(r.Context(), eventID)
	if err != nil {
		slog.Error("dashboard: failed to get delivery attempts", "error", err)
		attempts = nil
	}

	d.render(w, "event_detail.html", map[string]any{
		"Event":    event,
		"Attempts": attempts,
	})
}

func (d *Dashboard) handleReplayEvent(w http.ResponseWriter, r *http.Request) {
	eventID := chi.URLParam(r, "id")
	if err := d.store.UpdateEventStatus(r.Context(), store.UpdateStatusParams(eventID, "pending")); err != nil {
		slog.Error("dashboard: failed to replay event", "error", err)
		d.render(w, "toast.html", map[string]any{"Error": "Failed to replay event"})
		return
	}
	d.render(w, "toast.html", map[string]any{"Success": "Event queued for replay"})
}

func (d *Dashboard) handleBulkReplay(w http.ResponseWriter, r *http.Request) {
	count, err := d.store.BulkReplayDeadEvents(r.Context())
	if err != nil {
		slog.Error("dashboard: failed to bulk replay", "error", err)
		d.render(w, "toast.html", map[string]any{"Error": "Failed to replay dead events"})
		return
	}
	d.render(w, "toast.html", map[string]any{
		"Success": strconv.FormatInt(count, 10) + " event(s) queued for replay",
	})
}

// --- Endpoint Handlers ---

func (d *Dashboard) handleEndpoints(w http.ResponseWriter, r *http.Request) {
	endpoints, err := d.store.ListEndpoints(r.Context())
	if err != nil {
		slog.Error("dashboard: failed to list endpoints", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	d.render(w, "endpoints.html", map[string]any{"Endpoints": endpoints})
}

func (d *Dashboard) handleEndpointRows(w http.ResponseWriter, r *http.Request) {
	endpoints, err := d.store.ListEndpoints(r.Context())
	if err != nil {
		slog.Error("dashboard: failed to list endpoints", "error", err)
		return
	}
	d.render(w, "endpoint_rows.html", map[string]any{"Endpoints": endpoints})
}

func (d *Dashboard) handleCreateEndpoint(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		d.render(w, "toast.html", map[string]any{"Error": "Invalid form data"})
		return
	}

	url := strings.TrimSpace(r.FormValue("url"))
	topicsRaw := strings.TrimSpace(r.FormValue("topics"))
	description := strings.TrimSpace(r.FormValue("description"))

	if url == "" || topicsRaw == "" {
		d.render(w, "toast.html", map[string]any{"Error": "URL and topics are required"})
		return
	}

	topics := strings.Split(topicsRaw, ",")
	for i := range topics {
		topics[i] = strings.TrimSpace(topics[i])
	}

	signingSecret := generateSigningSecret()
	params := queries.CreateEndpointParams{
		ID:            id.NewEndpoint(),
		Url:           url,
		Topics:        topics,
		SigningSecret: signingSecret,
		Enabled:       true,
	}
	if description != "" {
		params.Description = pgtype.Text{String: description, Valid: true}
	}

	if err := d.store.CreateEndpoint(r.Context(), params); err != nil {
		slog.Error("dashboard: failed to create endpoint", "error", err)
		d.render(w, "toast.html", map[string]any{"Error": "Failed to create endpoint"})
		return
	}

	// Return updated endpoint table
	w.Header().Set("HX-Trigger", "showToast")
	d.handleEndpointRows(w, r)
}

func (d *Dashboard) handleToggleEndpoint(w http.ResponseWriter, r *http.Request) {
	endpointID := chi.URLParam(r, "id")

	ep, err := d.store.GetEndpoint(r.Context(), endpointID)
	if err != nil {
		d.render(w, "toast.html", map[string]any{"Error": "Endpoint not found"})
		return
	}

	newEnabled := !ep.Enabled
	if err := d.store.UpdateEndpoint(r.Context(), queries.UpdateEndpointParams{
		ID:      endpointID,
		Enabled: pgtype.Bool{Bool: newEnabled, Valid: true},
	}); err != nil {
		slog.Error("dashboard: failed to toggle endpoint", "error", err)
		d.render(w, "toast.html", map[string]any{"Error": "Failed to toggle endpoint"})
		return
	}

	d.handleEndpointRows(w, r)
}

func (d *Dashboard) handleDeleteEndpoint(w http.ResponseWriter, r *http.Request) {
	endpointID := chi.URLParam(r, "id")

	if err := d.store.DeleteEndpoint(r.Context(), endpointID); err != nil {
		slog.Error("dashboard: failed to delete endpoint", "error", err)
		d.render(w, "toast.html", map[string]any{"Error": "Failed to delete endpoint"})
		return
	}

	d.handleEndpointRows(w, r)
}

func generateSigningSecret() string {
	b := make([]byte, 32)
	_, _ = bytes.NewReader(make([]byte, 32)).Read(b)
	return "whsec_" + id.NewEndpoint()[3:] // reuse ULID entropy
}
