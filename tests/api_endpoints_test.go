package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sarthak-shubham/mandarin/internal/api"
	"github.com/sarthak-shubham/mandarin/internal/config"
	"github.com/sarthak-shubham/mandarin/internal/store"
)

func newTestAPIServer(t *testing.T, s *store.Store) *httptest.Server {
	t.Helper()

	cfg := config.Config{}
	cfg.Server.Port = 8080

	server := api.NewServer(cfg, s)
	ts := httptest.NewServer(server.Handler)

	t.Cleanup(func() {
		ts.Close()
	})

	return ts
}

func TestAPI_HealthCheck(t *testing.T) {
	_, s := setupTestRedis(t)
	ts := newTestAPIServer(t, s)

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("expected GET /health to succeed, got error: %s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var body map[string]string
	err = json.NewDecoder(resp.Body).Decode(&body)
	if err != nil {
		t.Fatalf("decode health response failed: %s", err)
	}

	if body["status"] != "ok" {
		t.Errorf("expected status ok, got %s", body["status"])
	}
}

func TestAPI_Endpoints_Lifecycle(t *testing.T) {
	_, s := setupTestRedis(t)
	ts := newTestAPIServer(t, s)

	badReq, err := http.Post(ts.URL+"/endpoints", "application/json", bytes.NewReader([]byte("{invalid-json}")))
	if err != nil {
		t.Fatalf("post bad json failed: %s", err)
	}
	defer badReq.Body.Close()
	if badReq.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for bad json, got %d", badReq.StatusCode)
	}

	badURLReq, err := http.Post(ts.URL+"/endpoints", "application/json", bytes.NewReader([]byte(`{"url":"not-a-url"}`)))
	if err != nil {
		t.Fatalf("post bad url failed: %s", err)
	}
	defer badURLReq.Body.Close()
	if badURLReq.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for bad url, got %d", badURLReq.StatusCode)
	}

	createPayload := `{"url":"https://api.example.com/webhooks"}`
	createResp, err := http.Post(ts.URL+"/endpoints", "application/json", bytes.NewReader([]byte(createPayload)))
	if err != nil {
		t.Fatalf("post valid endpoint failed: %s", err)
	}
	defer createResp.Body.Close()
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 for valid endpoint, got %d", createResp.StatusCode)
	}

	var created store.Endpoint
	err = json.NewDecoder(createResp.Body).Decode(&created)
	if err != nil {
		t.Fatalf("decode created endpoint failed: %s", err)
	}
	if created.ID == "" {
		t.Error("expected generated id, got empty")
	}

	listResp, err := http.Get(ts.URL + "/endpoints")
	if err != nil {
		t.Fatalf("get endpoints failed: %s", err)
	}
	defer listResp.Body.Close()

	var endpoints []store.Endpoint
	err = json.NewDecoder(listResp.Body).Decode(&endpoints)
	if err != nil {
		t.Fatalf("decode endpoints failed: %s", err)
	}
	if len(endpoints) != 1 {
		t.Fatalf("expected 1 endpoint in list, got %d", len(endpoints))
	}

	delReq, err := http.NewRequest(http.MethodDelete, ts.URL+"/endpoints/"+created.ID, nil)
	if err != nil {
		t.Fatalf("create delete request failed: %s", err)
	}
	delResp, err := http.DefaultClient.Do(delReq)
	if err != nil {
		t.Fatalf("execute delete request failed: %s", err)
	}
	defer delResp.Body.Close()
	if delResp.StatusCode != http.StatusNoContent {
		t.Errorf("expected 204 for delete, got %d", delResp.StatusCode)
	}
}
