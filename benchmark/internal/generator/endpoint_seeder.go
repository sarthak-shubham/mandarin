package generator

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type EndpointSeeder struct {
	client *redis.Client
}

func NewEndpointSeeder(client *redis.Client) *EndpointSeeder {
	seeder := &EndpointSeeder{client: client}
	return seeder
}

func (s *EndpointSeeder) TeardownBenchmarkData(ctx context.Context, count int, eventType, eventID string) error {
	batchSize := 1000
	for i := 0; i < count; i += batchSize {
		limit := i + batchSize
		if limit > count {
			limit = count
		}

		pipe := s.client.Pipeline()
		for j := i; j < limit; j++ {
			id := fmt.Sprintf("bench-sub-%05d", j)
			pipe.Unlink(ctx, "endpoint:"+id)
			pipe.Unlink(ctx, "subscriptions:"+id)
			if eventID != "" {
				pipe.Unlink(ctx, fmt.Sprintf("delivery:%s:%s", eventID, id))
			}
		}

		_, err := pipe.Exec(ctx)
		if err != nil {
			return fmt.Errorf("teardown batch %d-%d: %w", i, limit, err)
		}
	}

	metaPipe := s.client.Pipeline()
	metaPipe.Unlink(ctx, "endpoints:all")
	if eventType != "" {
		metaPipe.Unlink(ctx, "event_type:"+eventType)
	}
	if eventID != "" {
		metaPipe.Unlink(ctx, "event:"+eventID)
		metaPipe.Unlink(ctx, "event_deliveries:"+eventID)
	}
	metaPipe.Unlink(ctx, "mandarin:deliveries")

	_, err := metaPipe.Exec(ctx)
	return err
}

func (s *EndpointSeeder) SeedSubscribers(ctx context.Context, count int, baseURL, eventType string) error {
	batchSize := 1000
	createdAt := time.Now().UTC().Format(time.RFC3339)

	for i := 0; i < count; i += batchSize {
		limit := i + batchSize
		if limit > count {
			limit = count
		}

		err := s.seedBatch(ctx, i, limit, baseURL, eventType, createdAt)
		if err != nil {
			return fmt.Errorf("seed batch %d-%d: %w", i, limit, err)
		}
	}

	return nil
}

func (s *EndpointSeeder) seedBatch(ctx context.Context, start, end int, baseURL, eventType, createdAt string) error {
	pipe := s.client.Pipeline()

	for j := start; j < end; j++ {
		id := fmt.Sprintf("bench-sub-%05d", j)
		targetURL := fmt.Sprintf("%s/webhook/%s", baseURL, id)

		endpointData := map[string]any{
			"id":         id,
			"url":        targetURL,
			"created_at": createdAt,
		}

		pipe.HSet(ctx, "endpoint:"+id, endpointData)
		pipe.SAdd(ctx, "endpoints:all", id)
		pipe.SAdd(ctx, "event_type:"+eventType, id)
		pipe.SAdd(ctx, "subscriptions:"+id, eventType)
	}

	_, err := pipe.Exec(ctx)
	return err
}
