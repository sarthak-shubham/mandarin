package store

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

func chunkSlice(items []string, chunkSize int) [][]string {
	if chunkSize <= 0 {
		chunkSize = 1000
	}

	var chunks [][]string
	for i := 0; i < len(items); i += chunkSize {
		end := i + chunkSize
		if end > len(items) {
			end = len(items)
		}
		chunks = append(chunks, items[i:end])
	}
	return chunks
}

func (s *Store) ScheduleDeliveries(ctx context.Context, eventID string, endpointIDs []string) error {
	chunks := chunkSlice(endpointIDs, s.BatchSize)

	for _, chunk := range chunks {
		pipe := s.Redis.Pipeline()

		for _, epID := range chunk {
			delKey := deliveryKey(eventID, epID)
			pipe.HSet(ctx, delKey,
				"event_id", eventID,
				"endpoint_id", epID,
				"state", StatePending,
			)
			pipe.SAdd(ctx, eventDeliveriesKey(eventID), epID)
			pipe.XAdd(ctx, &redis.XAddArgs{
				Stream: deliveryStreamKey(),
				Values: map[string]any{
					"event_id":    eventID,
					"endpoint_id": epID,
				},
			})
		}

		_, err := pipe.Exec(ctx)
		if err != nil {
			return fmt.Errorf("schedule deliveries batch: %w", err)
		}
	}

	return nil
}

func (s *Store) GetDeliveriesForEvent(ctx context.Context, eventID string) ([]Delivery, error) {
	endpointIDs, err := s.Redis.SMembers(ctx, eventDeliveriesKey(eventID)).Result()
	if err != nil {
		return nil, fmt.Errorf("list deliveries for event %q: %w", eventID, err)
	}

	chunks := chunkSlice(endpointIDs, s.BatchSize)
	var deliveries []Delivery

	for _, chunk := range chunks {
		pipe := s.Redis.Pipeline()
		var cmds []*redis.MapStringStringCmd

		for _, epID := range chunk {
			cmd := pipe.HGetAll(ctx, deliveryKey(eventID, epID))
			cmds = append(cmds, cmd)
		}

		_, err = pipe.Exec(ctx)
		if err != nil {
			return nil, fmt.Errorf("get deliveries pipeline: %w", err)
		}

		for _, cmd := range cmds {
			d := parseDelivery(cmd.Val())
			if d != nil {
				deliveries = append(deliveries, *d)
			}
		}
	}

	return deliveries, nil
}
