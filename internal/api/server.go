package api

import (
	"fmt"
	"net/http"

	"github.com/sarthak-shubham/mandarin/internal/config"
	"github.com/sarthak-shubham/mandarin/internal/store"
)

func NewServer(cfg config.Config, s *store.Store) *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", healthHandler)

	mux.HandleFunc("POST /endpoints", registerEndpointHandler(s))
	mux.HandleFunc("GET /endpoints", listEndpointsHandler(s))
	mux.HandleFunc("DELETE /endpoints/{id}", deleteEndpointHandler(s))

	mux.HandleFunc("POST /endpoints/{id}/subscriptions", subscribeHandler(s))
	mux.HandleFunc("DELETE /endpoints/{id}/subscriptions/{event_type}", unsubscribeHandler(s))

	mux.HandleFunc("POST /events", publishEventHandler(s))
	mux.HandleFunc("GET /events/{id}", getEventHandler(s))

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: mux,
	}
	return server
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
