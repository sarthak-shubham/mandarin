package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"tamarind/internal/store"
)

func TestAPI_Subscriptions_Lifecycle(t *testing.T) {
	_, s := setupTestRedis(t)
	ts := newTestAPIServer(t, s)

	nonExistentSub, err := http.Post(ts.URL+"/endpoints/no-such-id/subscriptions", "application/json", bytes.NewReader([]byte(`{"event_type":"user.created"}`)))
	if err != nil {
		t.Fatalf("post sub to non existent failed: %s", err)
	}
	defer nonExistentSub.Body.Close()
	if nonExistentSub.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for non existent endpoint, got %d", nonExistentSub.StatusCode)
	}

	createPayload := `{"url":"https://api.example.com/subs"}`
	createResp, err := http.Post(ts.URL+"/endpoints", "application/json", bytes.NewReader([]byte(createPayload)))
	if err != nil {
		t.Fatalf("create endpoint failed: %s", err)
	}
	defer createResp.Body.Close()

	var created store.Endpoint
	err = json.NewDecoder(createResp.Body).Decode(&created)
	if err != nil {
		t.Fatalf("decode endpoint failed: %s", err)
	}

	subEmpty, err := http.Post(ts.URL+"/endpoints/"+created.ID+"/subscriptions", "application/json", bytes.NewReader([]byte(`{"event_type":""}`)))
	if err != nil {
		t.Fatalf("post empty sub failed: %s", err)
	}
	defer subEmpty.Body.Close()
	if subEmpty.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for empty event_type, got %d", subEmpty.StatusCode)
	}

	subValid, err := http.Post(ts.URL+"/endpoints/"+created.ID+"/subscriptions", "application/json", bytes.NewReader([]byte(`{"event_type":"order.paid"}`)))
	if err != nil {
		t.Fatalf("post valid sub failed: %s", err)
	}
	defer subValid.Body.Close()
	if subValid.StatusCode != http.StatusNoContent {
		t.Errorf("expected 204 for valid subscription, got %d", subValid.StatusCode)
	}

	unsubReq, err := http.NewRequest(http.MethodDelete, ts.URL+"/endpoints/"+created.ID+"/subscriptions/order.paid", nil)
	if err != nil {
		t.Fatalf("create unsub request failed: %s", err)
	}
	unsubResp, err := http.DefaultClient.Do(unsubReq)
	if err != nil {
		t.Fatalf("execute unsub request failed: %s", err)
	}
	defer unsubResp.Body.Close()
	if unsubResp.StatusCode != http.StatusNoContent {
		t.Errorf("expected 204 for unsubscribe, got %d", unsubResp.StatusCode)
	}
}
