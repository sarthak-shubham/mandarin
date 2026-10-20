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

func TestWebhookDelivery_FanOut(t *testing.T) {
	_, s := setupTestRedis(t)
	receiverA, receivedA := newMockReceiver(t, http.StatusOK)
	receiverB, receivedB := newMockReceiver(t, http.StatusOK)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	cfg := config.Config{
		Workers:         3,
		DeliveryTimeout: 2 * time.Second,
	}

	err := worker.StartWorkers(ctx, s, cfg)
	if err != nil {
		t.Fatalf("start workers failed: %s", err)
	}

	epA := store.Endpoint{ID: "ep-fan-a", URL: receiverA.URL, CreatedAt: time.Now().UTC()}
	epB := store.Endpoint{ID: "ep-fan-b", URL: receiverB.URL, CreatedAt: time.Now().UTC()}

	err = s.RegisterEndpoint(ctx, epA)
	if err != nil {
		t.Fatalf("register epA failed: %s", err)
	}
	err = s.RegisterEndpoint(ctx, epB)
	if err != nil {
		t.Fatalf("register epB failed: %s", err)
	}

	err = s.Subscribe(ctx, epA.ID, "order.placed")
	if err != nil {
		t.Fatalf("subscribe epA failed: %s", err)
	}
	err = s.Subscribe(ctx, epB.ID, "order.placed")
	if err != nil {
		t.Fatalf("subscribe epB failed: %s", err)
	}

	ev := store.Event{
		ID:          "ev-fan-1",
		EventType:   "order.placed",
		PublishedAt: time.Now().UTC(),
		Payload:     json.RawMessage(`{"order_id": 42}`),
	}

	err = s.SaveEvent(ctx, ev)
	if err != nil {
		t.Fatalf("save event failed: %s", err)
	}

	subscribers, err := s.EndpointsForEventType(ctx, ev.EventType)
	if err != nil {
		t.Fatalf("endpoints for event type failed: %s", err)
	}

	for _, subID := range subscribers {
		pending := store.Delivery{EventID: ev.ID, EndpointID: subID, State: store.StatePending}
		err = s.SaveDelivery(ctx, pending)
		if err != nil {
			t.Fatalf("save delivery failed: %s", err)
		}
		err = s.EnqueueDelivery(ctx, ev.ID, subID)
		if err != nil {
			t.Fatalf("enqueue delivery failed: %s", err)
		}
	}

	deliveries := waitForDeliveries(t, s, ev.ID, 2, 3*time.Second)
	if len(deliveries) != 2 {
		t.Fatalf("expected 2 deliveries, got %d", len(deliveries))
	}

	for _, d := range deliveries {
		if d.State != store.StateSuccess {
			t.Errorf("delivery for endpoint %s failed: %s (status: %d)", d.EndpointID, d.FailureReason, d.HTTPStatus)
		}
	}

	if len(*receivedA) != 1 {
		t.Errorf("expected receiver A to get 1 webhook, got %d", len(*receivedA))
	}
	if len(*receivedB) != 1 {
		t.Errorf("expected receiver B to get 1 webhook, got %d", len(*receivedB))
	}
}
