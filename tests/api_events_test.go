package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/sarthak-shubham/mandarin/internal/store"
)

func TestAPI_PublishEvent_Validation(t *testing.T) {
	_, s := setupTestRedis(t)
	ts := newTestAPIServer(t, s)

	badJSONResp, err := http.Post(ts.URL+"/events", "application/json", bytes.NewReader([]byte("{bad-json}")))
	if err != nil {
		t.Fatalf("post bad json failed: %s", err)
	}
	defer badJSONResp.Body.Close()
	if badJSONResp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for bad json, got %d", badJSONResp.StatusCode)
	}

	noTypePayload := `{"event_type":"","payload":{"key":"val"}}`
	noTypeResp, err := http.Post(ts.URL+"/events", "application/json", bytes.NewReader([]byte(noTypePayload)))
	if err != nil {
		t.Fatalf("post no event_type failed: %s", err)
	}
	defer noTypeResp.Body.Close()
	if noTypeResp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for empty event_type, got %d", noTypeResp.StatusCode)
	}

	noDataPayload := `{"event_type":"user.login"}`
	noDataResp, err := http.Post(ts.URL+"/events", "application/json", bytes.NewReader([]byte(noDataPayload)))
	if err != nil {
		t.Fatalf("post no payload failed: %s", err)
	}
	defer noDataResp.Body.Close()
	if noDataResp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for missing payload, got %d", noDataResp.StatusCode)
	}
}

func TestAPI_PublishAndGetEvent_Success(t *testing.T) {
	ctx := t.Context()
	_, s := setupTestRedis(t)
	ts := newTestAPIServer(t, s)

	ep := store.Endpoint{
		ID:        "ep-api-test-1",
		URL:       "https://api.example.com/dest",
		CreatedAt: time.Now().UTC(),
	}
	err := s.RegisterEndpoint(ctx, ep)
	if err != nil {
		t.Fatalf("register endpoint failed: %s", err)
	}

	err = s.Subscribe(ctx, ep.ID, "order.completed")
	if err != nil {
		t.Fatalf("subscribe endpoint failed: %s", err)
	}

	validEvent := `{"event_type":"order.completed","payload":{"item":"laptop","qty":1}}`
	pubResp, err := http.Post(ts.URL+"/events", "application/json", bytes.NewReader([]byte(validEvent)))
	if err != nil {
		t.Fatalf("post valid event failed: %s", err)
	}
	defer pubResp.Body.Close()
	if pubResp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202 Accepted, got %d", pubResp.StatusCode)
	}

	var pubResult map[string]string
	err = json.NewDecoder(pubResp.Body).Decode(&pubResult)
	if err != nil {
		t.Fatalf("decode publish response failed: %s", err)
	}

	eventID := pubResult["id"]
	if eventID == "" {
		t.Fatal("expected non-empty event id in publish response")
	}

	getResp, err := http.Get(ts.URL + "/events/" + eventID)
	if err != nil {
		t.Fatalf("get event failed: %s", err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for get event, got %d", getResp.StatusCode)
	}

	var eventDetails struct {
		Event   store.Event    `json:"event"`
		Summary map[string]int `json:"summary"`
	}
	err = json.NewDecoder(getResp.Body).Decode(&eventDetails)
	if err != nil {
		t.Fatalf("decode event details failed: %s", err)
	}

	if eventDetails.Event.EventType != "order.completed" {
		t.Errorf("expected event_type order.completed, got %s", eventDetails.Event.EventType)
	}
	if eventDetails.Summary["pending"] != 1 {
		t.Errorf("expected 1 pending delivery, got %d", eventDetails.Summary["pending"])
	}

	notFoundResp, err := http.Get(ts.URL + "/events/non-existent-uuid")
	if err != nil {
		t.Fatalf("get non-existent event failed: %s", err)
	}
	defer notFoundResp.Body.Close()
	if notFoundResp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for non-existent event, got %d", notFoundResp.StatusCode)
	}
}
