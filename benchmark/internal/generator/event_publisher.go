package generator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type PublishResult struct {
	EventID         string        `json:"event_id"`
	EnqueueDuration time.Duration `json:"enqueue_duration"`
	TotalDuration   time.Duration `json:"total_duration"`
	PendingCount    int           `json:"pending_count"`
	SuccessCount    int           `json:"success_count"`
	FailedCount     int           `json:"failed_count"`
}

type EventPublisher struct {
	client      *http.Client
	tamarindURL string
}

func NewEventPublisher(tamarindURL string) *EventPublisher {
	pub := &EventPublisher{
		client:      &http.Client{Timeout: 10 * time.Second},
		tamarindURL: tamarindURL,
	}
	return pub
}

func (p *EventPublisher) PublishBroadcast(ctx context.Context, eventType string, payload any) (string, time.Duration, error) {
	body, err := json.Marshal(map[string]any{
		"event_type": eventType,
		"payload":    payload,
	})
	if err != nil {
		return "", 0, fmt.Errorf("marshal event: %w", err)
	}

	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.tamarindURL+"/events", bytes.NewReader(body))
	if err != nil {
		return "", 0, fmt.Errorf("create event request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("post event to tamarind: %w", err)
	}
	defer resp.Body.Close()

	enqueueTime := time.Since(start)
	if resp.StatusCode != http.StatusAccepted {
		return "", enqueueTime, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var result struct {
		ID string `json:"id"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return "", enqueueTime, fmt.Errorf("decode publish response: %w", err)
	}

	return result.ID, enqueueTime, nil
}

func (p *EventPublisher) WaitForCompletion(ctx context.Context, eventID string, totalExpected int) (*PublishResult, error) {
	start := time.Now()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			res, finished, err := p.checkEventStatus(ctx, eventID, totalExpected)
			if err != nil {
				return nil, err
			}
			if finished {
				res.TotalDuration = time.Since(start)
				return res, nil
			}
		}
	}
}

func (p *EventPublisher) checkEventStatus(ctx context.Context, id string, total int) (*PublishResult, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.tamarindURL+"/events/"+id, nil)
	if err != nil {
		return nil, false, err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()

	var data struct {
		Summary map[string]int `json:"summary"`
	}
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return nil, false, err
	}

	pending := data.Summary["pending"]
	success := data.Summary["success"]
	failed := data.Summary["failed"]

	res := &PublishResult{
		EventID:      id,
		PendingCount: pending,
		SuccessCount: success,
		FailedCount:  failed,
	}

	completed := (success+failed) >= total || (total > 0 && pending == 0 && (success+failed) > 0)
	return res, completed, nil
}
