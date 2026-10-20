package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
)

func publishEventHandler(store *Store) http.HandlerFunc {
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

		ev := Event{
			ID:          uuid.NewString(),
			EventType:   body.EventType,
			PublishedAt: time.Now().UTC(),
			Payload:     body.Payload,
		}

		err = store.SaveEvent(r.Context(), ev)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not save event")
			return
		}

		endpointIDs, err := store.EndpointsForEventType(r.Context(), ev.EventType)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not look up subscribers")
			return
		}

		for _, endpointID := range endpointIDs {
			delivery := Delivery{
				EventID:    ev.ID,
				EndpointID: endpointID,
				State:      StatePending,
			}

			err = store.SaveDelivery(r.Context(), delivery)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "could not save delivery")
				return
			}

			err = store.EnqueueDelivery(r.Context(), ev.ID, endpointID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "could not enqueue delivery")
				return
			}
		}

		writeJSON(w, http.StatusAccepted, map[string]string{"id": ev.ID})
	}
}

func getEventHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		ev, err := store.GetEvent(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not look up event")
			return
		}
		if ev == nil {
			writeError(w, http.StatusNotFound, "event not found")
			return
		}

		deliveries, err := store.GetDeliveriesForEvent(r.Context(), id)
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

func countDeliveries(deliveries []Delivery) (pending, success, failed int) {
	for _, d := range deliveries {
		switch d.State {
		case StatePending:
			pending++
		case StateSuccess:
			success++
		case StateFailed:
			failed++
		}
	}
	return
}
