package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
)

func registerEndpointHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			URL string `json:"url"`
		}

		err := json.NewDecoder(r.Body).Decode(&body)
		if err != nil {
			writeError(w, http.StatusBadRequest, "request body must be valid JSON")
			return
		}

		if !isValidURL(body.URL) {
			writeError(w, http.StatusBadRequest, "url is missing or invalid")
			return
		}

		ep := Endpoint{
			ID:        uuid.NewString(),
			URL:       body.URL,
			CreatedAt: time.Now().UTC(),
		}

		err = store.RegisterEndpoint(r.Context(), ep)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not save endpoint")
			return
		}

		writeJSON(w, http.StatusCreated, ep)
	}
}

func listEndpointsHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		endpoints, err := store.ListEndpoints(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not list endpoints")
			return
		}

		writeJSON(w, http.StatusOK, endpoints)
	}
}

func deleteEndpointHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		ep, err := store.GetEndpoint(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not look up endpoint")
			return
		}
		if ep == nil {
			writeError(w, http.StatusNotFound, "endpoint not found")
			return
		}

		err = store.DeleteEndpoint(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not delete endpoint")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func subscribeHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		ep, err := store.GetEndpoint(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not look up endpoint")
			return
		}
		if ep == nil {
			writeError(w, http.StatusNotFound, "endpoint not found")
			return
		}

		var body struct {
			EventType string `json:"event_type"`
		}

		err = json.NewDecoder(r.Body).Decode(&body)
		if err != nil {
			writeError(w, http.StatusBadRequest, "event_type is required")
			return
		}
		if body.EventType == "" {
			writeError(w, http.StatusBadRequest, "event_type is required")
			return
		}

		err = store.Subscribe(r.Context(), id, body.EventType)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not save subscription")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func unsubscribeHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		eventType := r.PathValue("event_type")

		ep, err := store.GetEndpoint(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not look up endpoint")
			return
		}
		if ep == nil {
			writeError(w, http.StatusNotFound, "endpoint not found")
			return
		}

		err = store.Unsubscribe(r.Context(), id, eventType)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not remove subscription")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
