package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/redis/go-redis/v9"

	"tamarind/benchmark/internal/config"
	"tamarind/benchmark/internal/generator"
	"tamarind/benchmark/internal/report"
)

func main() {
	configPath := flag.String("config", "benchmark/benchmark.yaml", "path to benchmark.yaml")
	subsFlag := flag.Int("subscribers", 0, "override number of subscribers")
	recvURLFlag := flag.String("receiver-url", "", "override receiver url")
	keepData := flag.Bool("keep-data", false, "skip post-run redis flush")
	flag.Parse()

	cfg, err := config.LoadBenchmarkConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "benchmark config error: %s\n", err)
		os.Exit(1)
	}

	if *subsFlag > 0 {
		cfg.Benchmark.Subscribers = *subsFlag
	}
	if *recvURLFlag != "" {
		cfg.Receiver.URL = *recvURLFlag
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Address,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer redisClient.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	err = redisClient.Ping(ctx).Err()
	cancel()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot connect to Redis at %s (DB %d): %s\nPlease run 'make up' or start Redis first.\n", cfg.Redis.Address, cfg.Redis.DB, err)
		os.Exit(1)
	}

	runBenchmark(cfg, redisClient, *keepData)
}

func runBenchmark(cfg config.BenchmarkConfig, client *redis.Client, keepData bool) {
	seeder := generator.NewEndpointSeeder(client)
	publisher := generator.NewEventPublisher(cfg.Tamarind.Address)

	fmt.Printf("1. Seeding %d subscribers to %q...\n", cfg.Benchmark.Subscribers, cfg.Benchmark.EventType)
	seedStart := time.Now()
	err := seeder.SeedSubscribers(context.Background(), cfg.Benchmark.Subscribers, cfg.Receiver.URL, cfg.Benchmark.EventType)
	if err != nil {
		log.Fatalf("seeding failed: %s", err)
	}
	fmt.Printf("   Seeded %d subscribers in %s\n", cfg.Benchmark.Subscribers, time.Since(seedStart))

	fmt.Println("2. Publishing broadcast event to Tamarind...")
	payload := map[string]any{"version": "2.0", "effective_date": "2026-11-01"}
	eventID, enqueueDuration, err := publisher.PublishBroadcast(context.Background(), cfg.Benchmark.EventType, payload)
	if err != nil {
		log.Fatalf("publish broadcast failed: %s\nIs Tamarind running at %s?", err, cfg.Tamarind.Address)
	}
	fmt.Printf("   Event accepted: %s (enqueue latency: %s)\n", eventID, enqueueDuration)

	fmt.Println("3. Delivering webhooks to receiver (waiting for workers)...")
	waitCtx, waitCancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer waitCancel()

	result, err := publisher.WaitForCompletion(waitCtx, eventID, cfg.Benchmark.Subscribers)
	if err != nil {
		log.Fatalf("delivery wait error: %s", err)
	}

	total := result.SuccessCount + result.FailedCount
	var rps float64
	if result.TotalDuration.Seconds() > 0 && total > 0 {
		rps = float64(total) / result.TotalDuration.Seconds()
	}

	var rate float64
	if total > 0 {
		rate = (float64(result.SuccessCount) / float64(total)) * 100.0
	}

	rep := report.BenchmarkReport{
		Timestamp:        time.Now().UTC(),
		Subscribers:      cfg.Benchmark.Subscribers,
		EventType:        cfg.Benchmark.EventType,
		EnqueueDuration:  enqueueDuration,
		DeliveryDuration: result.TotalDuration,
		ThroughputRPS:    rps,
		SuccessCount:     result.SuccessCount,
		FailedCount:      result.FailedCount,
		SuccessRate:      rate,
	}

	report.PrintSummary(rep)

	err = report.SaveJSON(rep, "benchmark/report.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not save report.json: %s\n", err)
	} else {
		fmt.Println("Report successfully saved to benchmark/report.json")
	}

	if !keepData {
		fmt.Println("4. Post-cleaning Redis keys (zero-footprint teardown)...")
		err = seeder.TeardownBenchmarkData(context.Background(), cfg.Benchmark.Subscribers, cfg.Benchmark.EventType, eventID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: teardown error: %s\n", err)
		} else {
			fmt.Println("   Teardown complete: all benchmark keys and stream unlinked.")
		}
	}
}
