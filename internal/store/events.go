package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type Event struct {
	ID          string          `json:"id"`
	EventType   string          `json:"event_type"`
	PublishedAt time.Time       `json:"published_at"`
	Payload     json.RawMessage `json:"payload"`
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
