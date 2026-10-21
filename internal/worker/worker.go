package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"tamarind/internal/config"
	"tamarind/internal/store"
)

const consumerGroupName = "tamarind-workers"

func StartWorkers(ctx context.Context, s *store.Store, cfg config.Config) error {
	err := s.Redis.XGroupCreateMkStream(ctx, store.DeliveryStreamKey(), consumerGroupName, "$").Err()
	if err != nil && !isGroupExistsError(err) {
		return fmt.Errorf("create consumer group: %w", err)
	}

	httpClient := &http.Client{Timeout: cfg.DeliveryTimeout}

	for i := 0; i < cfg.Workers; i++ {
		workerID := fmt.Sprintf("worker-%d", i)
		go runWorker(ctx, s, httpClient, workerID)
	}

	return nil
}

func runWorker(ctx context.Context, s *store.Store, client *http.Client, workerID string) {
	for ctx.Err() == nil {
		msgs, err := s.Redis.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    consumerGroupName,
			Consumer: workerID,
			Streams:  []string{store.DeliveryStreamKey(), ">"},
			Count:    1,
			Block:    2 * time.Second,
		}).Result()

		if err == redis.Nil {
			continue
		}
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("worker %s: read error: %s", workerID, err)
			continue
		}

		for _, stream := range msgs {
			for _, msg := range stream.Messages {
				processMessage(ctx, s, client, workerID, msg)
			}
		}
	}
}

func processMessage(ctx context.Context, s *store.Store, client *http.Client, workerID string, msg redis.XMessage) {
	eventID, _ := msg.Values["event_id"].(string)
	endpointID, _ := msg.Values["endpoint_id"].(string)

	deliver(ctx, s, client, workerID, eventID, endpointID)
	ack(ctx, s, msg.ID)
}

func deliver(ctx context.Context, s *store.Store, client *http.Client, workerID, eventID, endpointID string) {
	ev, err := s.GetEvent(ctx, eventID)
	if err != nil || ev == nil {
		markFailed(ctx, s, eventID, endpointID, "event not found")
		return
	}

	ep, err := s.GetEndpoint(ctx, endpointID)
	if err != nil || ep == nil {
		markFailed(ctx, s, eventID, endpointID, "endpoint not found")
		return
	}

	result := sendWebhook(client, ep.URL, ev)
	result.EventID = eventID
	result.EndpointID = endpointID

	err = s.SaveDelivery(ctx, result)
	if err != nil {
		log.Printf("worker %s: save delivery %s/%s: %s", workerID, eventID, endpointID, err)
	}
}

func sendWebhook(client *http.Client, targetURL string, ev *store.Event) store.Delivery {
	body, err := json.Marshal(map[string]any{
		"id":         ev.ID,
		"event_type": ev.EventType,
		"timestamp":  ev.PublishedAt,
		"payload":    ev.Payload,
	})
	if err != nil {
		res := store.Delivery{State: store.StateFailed, FailureReason: "could not marshal payload"}
		return res
	}

	start := time.Now()
	resp, err := client.Post(targetURL, "application/json", bytes.NewReader(body))
	elapsed := time.Since(start).Milliseconds()

	if err != nil {
		res := store.Delivery{
			State:         store.StateFailed,
			FailureReason: err.Error(),
			DurationMS:    elapsed,
		}
		return res
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		res := store.Delivery{
			State:      store.StateSuccess,
			HTTPStatus: resp.StatusCode,
			DurationMS: elapsed,
		}
		return res
	}

	res := store.Delivery{
		State:         store.StateFailed,
		HTTPStatus:    resp.StatusCode,
		FailureReason: fmt.Sprintf("non-2xx response: %d", resp.StatusCode),
		DurationMS:    elapsed,
	}
	return res
}

func markFailed(ctx context.Context, s *store.Store, eventID, endpointID, reason string) {
	d := store.Delivery{
		EventID:       eventID,
		EndpointID:    endpointID,
		State:         store.StateFailed,
		FailureReason: reason,
	}

	err := s.SaveDelivery(ctx, d)
	if err != nil {
		log.Printf("markFailed %s/%s: %s", eventID, endpointID, err)
	}
}

func ack(ctx context.Context, s *store.Store, msgID string) {
	s.Redis.XAck(ctx, store.DeliveryStreamKey(), consumerGroupName, msgID)
}

func isGroupExistsError(err error) bool {
	exists := strings.Contains(err.Error(), "BUSYGROUP")
	return exists
}

