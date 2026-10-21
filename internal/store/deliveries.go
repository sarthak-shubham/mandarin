package store

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

const (
	StatePending = "pending"
	StateSuccess = "success"
	StateFailed  = "failed"
)

type Delivery struct {
	EventID       string `json:"event_id"`
	EndpointID    string `json:"endpoint_id"`
	State         string `json:"state"`
	HTTPStatus    int    `json:"http_status,omitempty"`
	FailureReason string `json:"failure_reason,omitempty"`
	DurationMS    int64  `json:"duration_ms,omitempty"`
}

func (s *Store) SaveDelivery(ctx context.Context, d Delivery) error {
	pipe := s.Redis.Pipeline()
	pipe.HSet(ctx, deliveryKey(d.EventID, d.EndpointID),
		"event_id", d.EventID,
		"endpoint_id", d.EndpointID,
		"state", d.State,
		"http_status", d.HTTPStatus,
		"failure_reason", d.FailureReason,
		"duration_ms", d.DurationMS,
	)
	pipe.SAdd(ctx, eventDeliveriesKey(d.EventID), d.EndpointID)

	_, err := pipe.Exec(ctx)
	return err
}

func (s *Store) GetDelivery(ctx context.Context, eventID, endpointID string) (*Delivery, error) {
	data, err := s.Redis.HGetAll(ctx, deliveryKey(eventID, endpointID)).Result()
	if err != nil {
		return nil, fmt.Errorf("get delivery %q/%q: %w", eventID, endpointID, err)
	}
	if len(data) == 0 {
		return nil, nil
	}

	d := parseDelivery(data)
	return d, nil
}

func (s *Store) EnqueueDelivery(ctx context.Context, eventID, endpointID string) error {
	args := &redis.XAddArgs{
		Stream: deliveryStreamKey(),
		Values: map[string]any{
			"event_id":    eventID,
			"endpoint_id": endpointID,
		},
	}

	err := s.Redis.XAdd(ctx, args).Err()
	return err
}

func parseDelivery(data map[string]string) *Delivery {
	if len(data) == 0 {
		return nil
	}

	var httpStatus int
	fmt.Sscanf(data["http_status"], "%d", &httpStatus)

	var durationMS int64
	fmt.Sscanf(data["duration_ms"], "%d", &durationMS)

	d := &Delivery{
		EventID:       data["event_id"],
		EndpointID:    data["endpoint_id"],
		State:         data["state"],
		HTTPStatus:    httpStatus,
		FailureReason: data["failure_reason"],
		DurationMS:    durationMS,
	}
	return d
}
