package report

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type BenchmarkReport struct {
	Timestamp        time.Time     `json:"timestamp"`
	Subscribers      int           `json:"subscribers"`
	EventType        string        `json:"event_type"`
	EnqueueDuration  time.Duration `json:"enqueue_duration"`
	DeliveryDuration time.Duration `json:"delivery_duration"`
	ThroughputRPS    float64       `json:"throughput_rps"`
	SuccessCount     int           `json:"success_count"`
	FailedCount      int           `json:"failed_count"`
	SuccessRate      float64       `json:"success_rate_percent"`
}

func PrintSummary(r BenchmarkReport) {
	fmt.Println()
	fmt.Println("==========================================================")
	fmt.Println("               TAMARIND 10K FAN-OUT BENCHMARK             ")
	fmt.Println("==========================================================")
	fmt.Printf("Timestamp:          %s\n", r.Timestamp.Format(time.RFC3339))
	fmt.Printf("Subscribers:        %d\n", r.Subscribers)
	fmt.Printf("Event Broadcasted:  %s\n", r.EventType)
	fmt.Println("----------------------------------------------------------")
	fmt.Printf("Enqueue Latency:    %s\n", r.EnqueueDuration)
	fmt.Printf("Delivery Duration:  %s\n", r.DeliveryDuration)
	fmt.Printf("Throughput:         %.2f webhooks/sec\n", r.ThroughputRPS)
	fmt.Println("----------------------------------------------------------")
	fmt.Printf("Deliveries Succeeded: %d\n", r.SuccessCount)
	fmt.Printf("Deliveries Failed:    %d\n", r.FailedCount)
	fmt.Printf("Success Rate:         %.2f%%\n", r.SuccessRate)
	fmt.Println("==========================================================")
	fmt.Println()
}

func SaveJSON(r BenchmarkReport, path string) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal report json: %w", err)
	}

	err = os.WriteFile(path, data, 0644)
	if err != nil {
		return fmt.Errorf("write report to %q: %w", path, err)
	}

	return nil
}
