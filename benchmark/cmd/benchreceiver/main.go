package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/sarthak-shubham/mandarin/benchmark/internal/config"
	"github.com/sarthak-shubham/mandarin/benchmark/internal/receiver"
)

func main() {
	configPath := flag.String("config", "benchmark/benchmark.yaml", "path to benchmark.yaml")
	portFlag := flag.Int("port", 0, "port to listen on (overrides config)")
	delayFlag := flag.Duration("delay", 0, "artificial delay before responding (overrides config)")
	failFlag := flag.Float64("failure-rate", -1.0, "failure rate 0.0-1.0 (overrides config)")
	flag.Parse()

	port := 9000
	var delay time.Duration
	var failureRate float64

	cfg, err := config.LoadBenchmarkConfig(*configPath)
	if err == nil {
		port = cfg.Receiver.Port
		delay = cfg.Receiver.Delay
		failureRate = cfg.Receiver.FailureRate
	}

	if *portFlag > 0 {
		port = *portFlag
	}
	if *delayFlag > 0 {
		delay = *delayFlag
	}
	if *failFlag >= 0.0 {
		failureRate = *failFlag
	}

	srv := receiver.NewReceiverServer(port, delay, failureRate)

	fmt.Println("==========================================================")
	fmt.Printf("   Tamarind Mock Webhook Receiver listening on :%d\n", port)
	if delay > 0 {
		fmt.Printf("   Simulated network latency: %s\n", delay)
	}
	if failureRate > 0 {
		fmt.Printf("   Simulated failure rate:    %.1f%%\n", failureRate*100)
	}
	fmt.Println("==========================================================")

	err = srv.ListenAndServe()
	if err != nil {
		log.Fatalf("receiver error: %s", err)
	}
}
