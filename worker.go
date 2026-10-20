package main

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
)

const consumerGroupName = "mochi-workers"

func StartWorkers(ctx context.Context, store *Store, cfg Config) error {
	err := store.redis.XGroupCreateMkStream(ctx, deliveryStreamKey(), consumerGroupName, "$").Err()
	if err != nil && !isGroupExistsError(err) {
		return fmt.Errorf("create consumer group: %w", err)
	}

	httpClient := &http.Client{Timeout: cfg.DeliveryTimeout}

	for i := 0; i < cfg.Workers; i++ {
		workerID := fmt.Sprintf("worker-%d", i)
		go runWorker(ctx, store, httpClient, workerID)
	}

	return nil
}

func runWorker(ctx context.Context, store *Store, client *http.Client, workerID string) {
	for ctx.Err() == nil {
		msgs, err := store.redis.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    consumerGroupName,
			Consumer: workerID,
			Streams:  []string{deliveryStreamKey(), ">"},
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
				processMessage(ctx, store, client, workerID, msg)
			}
		}
	}
}

func processMessage(ctx context.Context, store *Store, client *http.Client, workerID string, msg redis.XMessage) {
	eventID, _ := msg.Values["event_id"].(string)
	endpointID, _ := msg.Values["endpoint_id"].(string)

	ev, err := store.GetEvent(ctx, eventID)
	if err != nil || ev == nil {
		markFailed(ctx, store, eventID, endpointID, "event not found")
		ack(ctx, store, msg.ID)
		return
	}

	ep, err := store.GetEndpoint(ctx, endpointID)
	if err != nil || ep == nil {
		markFailed(ctx, store, eventID, endpointID, "endpoint not found")
		ack(ctx, store, msg.ID)
		return
	}

	result := sendWebhook(client, ep.URL, ev)
	result.EventID = eventID
	result.EndpointID = endpointID

	err = store.SaveDelivery(ctx, result)
	if err != nil {
		log.Printf("worker %s: save delivery %s/%s: %s", workerID, eventID, endpointID, err)
	}

	ack(ctx, store, msg.ID)
}

func sendWebhook(client *http.Client, targetURL string, ev *Event) Delivery {
	body, err := json.Marshal(map[string]any{
		"id":         ev.ID,
		"event_type": ev.EventType,
		"timestamp":  ev.PublishedAt,
		"payload":    ev.Payload,
	})
	if err != nil {
		return Delivery{State: StateFailed, FailureReason: "could not marshal payload"}
	}

	start := time.Now()
	resp, err := client.Post(targetURL, "application/json", bytes.NewReader(body))
	elapsed := time.Since(start).Milliseconds()

	if err != nil {
		return Delivery{
			State:         StateFailed,
			FailureReason: err.Error(),
			DurationMS:    elapsed,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return Delivery{
			State:      StateSuccess,
			HTTPStatus: resp.StatusCode,
			DurationMS: elapsed,
		}
	}

	return Delivery{
		State:         StateFailed,
		HTTPStatus:    resp.StatusCode,
		FailureReason: fmt.Sprintf("non-2xx response: %d", resp.StatusCode),
		DurationMS:    elapsed,
	}
}

func markFailed(ctx context.Context, store *Store, eventID, endpointID, reason string) {
	d := Delivery{
		EventID:       eventID,
		EndpointID:    endpointID,
		State:         StateFailed,
		FailureReason: reason,
	}

	err := store.SaveDelivery(ctx, d)
	if err != nil {
		log.Printf("markFailed %s/%s: %s", eventID, endpointID, err)
	}
}

func ack(ctx context.Context, store *Store, msgID string) {
	store.redis.XAck(ctx, deliveryStreamKey(), consumerGroupName, msgID)
}

func isGroupExistsError(err error) bool {
	return strings.Contains(err.Error(), "BUSYGROUP")
}
