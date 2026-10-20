package main

import (
	"context"
	"fmt"
	"time"
)

type Endpoint struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Store) RegisterEndpoint(ctx context.Context, ep Endpoint) error {
	err := s.saveEndpoint(ctx, ep)
	if err != nil {
		return err
	}

	err = s.redis.SAdd(ctx, allEndpointsKey(), ep.ID).Err()
	return err
}

func (s *Store) GetEndpoint(ctx context.Context, id string) (*Endpoint, error) {
	data, err := s.redis.HGetAll(ctx, endpointKey(id)).Result()
	if err != nil {
		return nil, fmt.Errorf("get endpoint %q: %w", id, err)
	}
	if len(data) == 0 {
		return nil, nil
	}

	createdAt, err := time.Parse(time.RFC3339, data["created_at"])
	if err != nil {
		return nil, fmt.Errorf("parse created_at for endpoint %q: %w", id, err)
	}

	ep := &Endpoint{
		ID:        data["id"],
		URL:       data["url"],
		CreatedAt: createdAt,
	}
	return ep, nil
}

func (s *Store) ListEndpoints(ctx context.Context) ([]Endpoint, error) {
	ids, err := s.redis.SMembers(ctx, allEndpointsKey()).Result()
	if err != nil {
		return nil, fmt.Errorf("list endpoint IDs: %w", err)
	}

	endpoints := make([]Endpoint, 0, len(ids))
	for _, id := range ids {
		ep, err := s.GetEndpoint(ctx, id)
		if err != nil {
			return nil, err
		}
		if ep != nil {
			endpoints = append(endpoints, *ep)
		}
	}

	return endpoints, nil
}

func (s *Store) DeleteEndpoint(ctx context.Context, id string) error {
	eventTypes, err := s.redis.SMembers(ctx, subscriptionsKey(id)).Result()
	if err != nil {
		return fmt.Errorf("read subscriptions for endpoint %q: %w", id, err)
	}

	pipe := s.redis.Pipeline()
	pipe.Del(ctx, endpointKey(id))
	pipe.Del(ctx, subscriptionsKey(id))
	pipe.SRem(ctx, allEndpointsKey(), id)
	for _, et := range eventTypes {
		pipe.SRem(ctx, eventTypeKey(et), id)
	}

	_, err = pipe.Exec(ctx)
	return err
}

func (s *Store) Subscribe(ctx context.Context, endpointID, eventType string) error {
	pipe := s.redis.Pipeline()
	pipe.SAdd(ctx, subscriptionsKey(endpointID), eventType)
	pipe.SAdd(ctx, eventTypeKey(eventType), endpointID)

	_, err := pipe.Exec(ctx)
	return err
}

func (s *Store) Unsubscribe(ctx context.Context, endpointID, eventType string) error {
	pipe := s.redis.Pipeline()
	pipe.SRem(ctx, subscriptionsKey(endpointID), eventType)
	pipe.SRem(ctx, eventTypeKey(eventType), endpointID)

	_, err := pipe.Exec(ctx)
	return err
}

func (s *Store) EndpointsForEventType(ctx context.Context, eventType string) ([]string, error) {
	ids, err := s.redis.SMembers(ctx, eventTypeKey(eventType)).Result()
	if err != nil {
		return nil, fmt.Errorf("endpoints for event type %q: %w", eventType, err)
	}
	return ids, nil
}

func (s *Store) saveEndpoint(ctx context.Context, ep Endpoint) error {
	key := endpointKey(ep.ID)
	err := s.redis.HSet(ctx, key,
		"id", ep.ID,
		"url", ep.URL,
		"created_at", ep.CreatedAt.Format(time.RFC3339),
	).Err()
	return err
}
