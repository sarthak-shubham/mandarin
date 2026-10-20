package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"mochi/internal/config"
	"mochi/internal/store"
	"mochi/internal/worker"
)

func TestWebhookDelivery_Success(t *testing.T) {
	_, s := setupTestRedis(t)
	receiver, received := newMockReceiver(t, http.StatusOK)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	cfg := config.Config{
		Workers:         2,
		DeliveryTimeout: 2 * time.Second,
	}

	err := worker.StartWorkers(ctx, s, cfg)
	if err != nil {
		t.Fatalf("start workers failed: %s", err)
	}

	ep := store.Endpoint{
		ID:        "ep-test-1",
		URL:       receiver.URL,
		CreatedAt: time.Now().UTC(),
	}

	err = s.RegisterEndpoint(ctx, ep)
	if err != nil {
		t.Fatalf("register endpoint failed: %s", err)
	}

	err = s.Subscribe(ctx, ep.ID, "payment.success")
	if err != nil {
		t.Fatalf("subscribe failed: %s", err)
	}

	ev := store.Event{
		ID:          "ev-test-1",
		EventType:   "payment.success",
		PublishedAt: time.Now().UTC(),
		Payload:     json.RawMessage(`{"amount": 100}`),
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
	if delivery.State != store.StateSuccess {
		t.Errorf("expected state %s, got %s (reason: %s)", store.StateSuccess, delivery.State, delivery.FailureReason)
	}
	if delivery.HTTPStatus != 200 {
		t.Errorf("expected HTTP status 200, got %d", delivery.HTTPStatus)
	}

	if len(*received) != 1 {
		t.Fatalf("expected receiver to get 1 webhook, got %d", len(*received))
	}
}
