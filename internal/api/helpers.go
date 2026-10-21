package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"tamarind/internal/store"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func isValidURL(raw string) bool {
	if raw == "" {
		return false
	}
	u, err := url.ParseRequestURI(raw)
	if err != nil {
		return false
	}
	valid := u.Scheme == "http" || u.Scheme == "https"
	return valid
}

func requireEndpoint(ctx context.Context, s *store.Store, w http.ResponseWriter, id string) (*store.Endpoint, bool) {
	ep, err := s.GetEndpoint(ctx, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not look up endpoint")
		return nil, false
	}
	if ep == nil {
		writeError(w, http.StatusNotFound, "endpoint not found")
		return nil, false
	}
	return ep, true
}
