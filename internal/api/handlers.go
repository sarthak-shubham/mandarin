package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/sarthak-shubham/mandarin/internal/store"
)

func registerEndpointHandler(s *store.Store) http.HandlerFunc {
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

		ep := store.Endpoint{
			ID:        uuid.NewString(),
			URL:       body.URL,
			CreatedAt: time.Now().UTC(),
		}

		err = s.RegisterEndpoint(r.Context(), ep)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not save endpoint")
			return
		}

		writeJSON(w, http.StatusCreated, ep)
	}
}

func listEndpointsHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		endpoints, err := s.ListEndpoints(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not list endpoints")
			return
		}

		writeJSON(w, http.StatusOK, endpoints)
	}
}

func deleteEndpointHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		_, ok := requireEndpoint(r.Context(), s, w, id)
		if !ok {
			return
		}

		err := s.DeleteEndpoint(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not delete endpoint")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func subscribeHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		_, ok := requireEndpoint(r.Context(), s, w, id)
		if !ok {
			return
		}

		var body struct {
			EventType string `json:"event_type"`
		}

		err := json.NewDecoder(r.Body).Decode(&body)
		if err != nil {
			writeError(w, http.StatusBadRequest, "event_type is required")
			return
		}
		if body.EventType == "" {
			writeError(w, http.StatusBadRequest, "event_type is required")
			return
		}

		err = s.Subscribe(r.Context(), id, body.EventType)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not save subscription")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func unsubscribeHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		eventType := r.PathValue("event_type")

		_, ok := requireEndpoint(r.Context(), s, w, id)
		if !ok {
			return
		}

		err := s.Unsubscribe(r.Context(), id, eventType)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not remove subscription")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func publishEventHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			EventType string          `json:"event_type"`
			Payload   json.RawMessage `json:"payload"`
		}

		err := json.NewDecoder(r.Body).Decode(&body)
		if err != nil {
			writeError(w, http.StatusBadRequest, "request body must be valid JSON")
			return
		}
		if body.EventType == "" {
			writeError(w, http.StatusBadRequest, "event_type is required")
			return
		}
		if len(body.Payload) == 0 || !json.Valid(body.Payload) {
			writeError(w, http.StatusBadRequest, "payload must be valid JSON")
			return
		}

		ev := store.Event{
			ID:          uuid.NewString(),
			EventType:   body.EventType,
			PublishedAt: time.Now().UTC(),
			Payload:     body.Payload,
		}

		err = s.SaveEvent(r.Context(), ev)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not save event")
			return
		}

		endpointIDs, err := s.EndpointsForEventType(r.Context(), ev.EventType)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not look up subscribers")
			return
		}

		err = scheduleDeliveries(r.Context(), s, ev.ID, endpointIDs)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not schedule deliveries")
			return
		}

		writeJSON(w, http.StatusAccepted, map[string]string{"id": ev.ID})
	}
}

func scheduleDeliveries(ctx context.Context, s *store.Store, eventID string, endpointIDs []string) error {
	err := s.ScheduleDeliveries(ctx, eventID, endpointIDs)
	return err
}

func getEventHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		ev, err := s.GetEvent(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not look up event")
			return
		}
		if ev == nil {
			writeError(w, http.StatusNotFound, "event not found")
			return
		}

		deliveries, err := s.GetDeliveriesForEvent(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not look up deliveries")
			return
		}

		pending, success, failed := countDeliveries(deliveries)

		writeJSON(w, http.StatusOK, map[string]any{
			"event": ev,
			"summary": map[string]int{
				"pending": pending,
				"success": success,
				"failed":  failed,
			},
			"deliveries": deliveries,
		})
	}
}

func countDeliveries(deliveries []store.Delivery) (pending, success, failed int) {
	for _, d := range deliveries {
		switch d.State {
		case store.StatePending:
			pending++
		case store.StateSuccess:
			success++
		case store.StateFailed:
			failed++
		}
	}
	return pending, success, failed
}
