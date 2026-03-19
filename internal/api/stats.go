package api

import (
	"log/slog"
	"net/http"
)

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	rows, err := s.store.CountEventsByStatus(r.Context())
	if err != nil {
		slog.Error("failed to count events by status", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get stats")
		return
	}

	stats := map[string]int64{
		"pending":    0,
		"processing": 0,
		"delivered":  0,
		"failed":     0,
		"dead":       0,
	}

	var total int64
	for _, row := range rows {
		stats[row.Status] = row.Count
		total += row.Count
	}
	stats["total"] = total

	writeJSON(w, http.StatusOK, stats)
}
