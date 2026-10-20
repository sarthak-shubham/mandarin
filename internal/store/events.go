package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	StatePending = "pending"
	StateSuccess = "success"
	StateFailed  = "failed"
)

type Event struct {
	ID          string          `json:"id"`
	EventType   string          `json:"event_type"`
	PublishedAt time.Time       `json:"published_at"`
	Payload     json.RawMessage `json:"payload"`
}

type Delivery struct {
	EventID       string `json:"event_id"`
	EndpointID    string `json:"endpoint_id"`
	State         string `json:"state"`
	HTTPStatus    int    `json:"http_status,omitempty"`
	FailureReason string `json:"failure_reason,omitempty"`
	DurationMS    int64  `json:"duration_ms,omitempty"`
}

func (s *Store) SaveEvent(ctx context.Context, ev Event) error {
	key := eventKey(ev.ID)
	err := s.Redis.HSet(ctx, key,
		"id", ev.ID,
		"event_type", ev.EventType,
		"published_at", ev.PublishedAt.Format(time.RFC3339),
		"payload", string(ev.Payload),
	).Err()
	return err
}

func (s *Store) GetEvent(ctx context.Context, id string) (*Event, error) {
	data, err := s.Redis.HGetAll(ctx, eventKey(id)).Result()
	if err != nil {
		return nil, fmt.Errorf("get event %q: %w", id, err)
	}
	if len(data) == 0 {
		return nil, nil
	}

	publishedAt, err := time.Parse(time.RFC3339, data["published_at"])
	if err != nil {
		return nil, fmt.Errorf("parse published_at for event %q: %w", id, err)
	}

	ev := &Event{
		ID:          data["id"],
		EventType:   data["event_type"],
		PublishedAt: publishedAt,
		Payload:     json.RawMessage(data["payload"]),
	}
	return ev, nil
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
	return d, nil
}

func (s *Store) GetDeliveriesForEvent(ctx context.Context, eventID string) ([]Delivery, error) {
	endpointIDs, err := s.Redis.SMembers(ctx, eventDeliveriesKey(eventID)).Result()
	if err != nil {
		return nil, fmt.Errorf("list deliveries for event %q: %w", eventID, err)
	}

	deliveries := make([]Delivery, 0, len(endpointIDs))
	for _, epID := range endpointIDs {
		d, err := s.GetDelivery(ctx, eventID, epID)
		if err != nil {
			return nil, err
		}
		if d != nil {
			deliveries = append(deliveries, *d)
		}
	}

	return deliveries, nil
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
