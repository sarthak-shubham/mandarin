package tests

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"tamarind/internal/store"
)

type receivedWebhook struct {
	Headers http.Header
	Body    map[string]any
}

func setupTestRedis(t *testing.T) (*redis.Client, *store.Store) {
	t.Helper()

	addr := os.Getenv("TAMARIND_TEST_REDIS_ADDRESS")
	if addr == "" {
		addr = "localhost:6379"
	}

	db := 15
	dbStr := os.Getenv("TAMARIND_TEST_REDIS_DB")
	if dbStr != "" {
		parsedDB, err := strconv.Atoi(dbStr)
		if err == nil {
			db = parsedDB
		}
	}

	client := redis.NewClient(&redis.Options{
		Addr: addr,
		DB:   db,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := client.Ping(ctx).Err()
	if err != nil {
		client.Close()
		t.Skipf("skipping test: Redis not reachable at %s: %s", addr, err)
	}

	client.FlushDB(context.Background())

	t.Cleanup(func() {
		client.FlushDB(context.Background())
		client.Close()
	})

	s := store.NewStore(client)
	return client, s
}

func newMockReceiver(t *testing.T, statusCode int) (*httptest.Server, *[]receivedWebhook) {
	t.Helper()

	var mu sync.Mutex
	var received []receivedWebhook

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		var payload map[string]any
		err = json.Unmarshal(bodyBytes, &payload)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		mu.Lock()
		item := receivedWebhook{
			Headers: r.Header.Clone(),
			Body:    payload,
		}
		received = append(received, item)
		mu.Unlock()

		w.WriteHeader(statusCode)
	}))

	t.Cleanup(func() {
		server.Close()
	})

	return server, &received
}

func waitForDeliveries(t *testing.T, s *store.Store, eventID string, count int, timeout time.Duration) []store.Delivery {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		deliveries, err := s.GetDeliveriesForEvent(context.Background(), eventID)
		if err == nil && len(deliveries) >= count {
			finished := true
			for _, d := range deliveries {
				if d.State == store.StatePending {
					finished = false
					break
				}
			}
			if finished {
				return deliveries
			}
		}
		time.Sleep(20 * time.Millisecond)
	}

	deliveries, _ := s.GetDeliveriesForEvent(context.Background(), eventID)
	return deliveries
}
