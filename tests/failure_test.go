package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/sarthak-shubham/mandarin/internal/config"
	"github.com/sarthak-shubham/mandarin/internal/store"
	"github.com/sarthak-shubham/mandarin/internal/worker"
)

func TestWebhookDelivery_ReceiverReturns500(t *testing.T) {
	_, s := setupTestRedis(t)
	receiver, _ := newMockReceiver(t, http.StatusInternalServerError)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	cfg := config.Config{
		Workers:         1,
		DeliveryTimeout: 2 * time.Second,
	}

	err := worker.StartWorkers(ctx, s, cfg)
	if err != nil {
		t.Fatalf("start workers failed: %s", err)
	}

	ep := store.Endpoint{
		ID:        "ep-fail-500",
		URL:       receiver.URL,
		CreatedAt: time.Now().UTC(),
	}

	err = s.RegisterEndpoint(ctx, ep)
	if err != nil {
		t.Fatalf("register endpoint failed: %s", err)
	}

	err = s.Subscribe(ctx, ep.ID, "alert.triggered")
	if err != nil {
		t.Fatalf("subscribe failed: %s", err)
	}

	ev := store.Event{
		ID:          "ev-fail-1",
		EventType:   "alert.triggered",
		PublishedAt: time.Now().UTC(),
		Payload:     json.RawMessage(`{"alert": "cpu"}`),
	}

	err = s.SaveEvent(ctx, ev)
	if err != nil {
		t.Fatalf("save event failed: %s", err)
	}

	pending := store.Delivery{
		EventID:    ev.ID,
		EndpointID: ep.ID,
		State:      store.StatePending,
	}

	err = s.SaveDelivery(ctx, pending)
	if err != nil {
		t.Fatalf("save delivery failed: %s", err)
	}

	err = s.EnqueueDelivery(ctx, ev.ID, ep.ID)
	if err != nil {
		t.Fatalf("enqueue delivery failed: %s", err)
	}

	deliveries := waitForDeliveries(t, s, ev.ID, 1, 3*time.Second)
	if len(deliveries) != 1 {
		t.Fatalf("expected 1 delivery, got %d", len(deliveries))
	}

	delivery := deliveries[0]
	if delivery.State != store.StateFailed {
		t.Errorf("expected state %s, got %s", store.StateFailed, delivery.State)
	}
	if delivery.HTTPStatus != 500 {
		t.Errorf("expected HTTP status 500, got %d", delivery.HTTPStatus)
	}
}

func TestWebhookDelivery_DeletedEndpointCleanup(t *testing.T) {
	_, s := setupTestRedis(t)
	receiver, received := newMockReceiver(t, http.StatusOK)

	ctx := context.Background()

	ep := store.Endpoint{
		ID:        "ep-cleanup-1",
		URL:       receiver.URL,
		CreatedAt: time.Now().UTC(),
	}

	err := s.RegisterEndpoint(ctx, ep)
	if err != nil {
		t.Fatalf("register endpoint failed: %s", err)
	}

	err = s.Subscribe(ctx, ep.ID, "user.deleted")
	if err != nil {
		t.Fatalf("subscribe failed: %s", err)
	}

	err = s.DeleteEndpoint(ctx, ep.ID)
	if err != nil {
		t.Fatalf("delete endpoint failed: %s", err)
	}

	subscribers, err := s.EndpointsForEventType(ctx, "user.deleted")
	if err != nil {
		t.Fatalf("query subscribers failed: %s", err)
	}

	if len(subscribers) != 0 {
		t.Errorf("expected 0 subscribers after deletion, got %d", len(subscribers))
	}

	if len(*received) != 0 {
		t.Errorf("expected receiver to receive 0 webhooks, got %d", len(*received))
	}
}
