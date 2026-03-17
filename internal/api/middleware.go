package api

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeError(w, http.StatusUnauthorized, "missing Authorization header")
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == authHeader {
			writeError(w, http.StatusUnauthorized, "invalid Authorization format, use: Bearer <token>")
			return
		}

		if subtle.ConstantTimeCompare([]byte(token), []byte(s.cfg.APIKey)) != 1 {
			writeError(w, http.StatusUnauthorized, "invalid API key")
			return
		}

		next.ServeHTTP(w, r)
	})
}
