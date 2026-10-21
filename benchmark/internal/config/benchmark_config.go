package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type BenchmarkConfig struct {
	Tamarind struct {
		Address string `yaml:"address"`
	} `yaml:"tamarind"`

	Redis struct {
		Address  string `yaml:"address"`
		Password string `yaml:"password"`
		DB       int    `yaml:"db"`
	} `yaml:"redis"`

	Receiver struct {
		Port        int           `yaml:"port"`
		URL         string        `yaml:"url"`
		Delay       time.Duration `yaml:"delay"`
		FailureRate float64       `yaml:"failure_rate"`
	} `yaml:"receiver"`

	Benchmark struct {
		Subscribers int    `yaml:"subscribers"`
		EventType   string `yaml:"event_type"`
		BatchSize   int    `yaml:"batch_size"`
	} `yaml:"benchmark"`
}

func defaultBenchmarkConfig() BenchmarkConfig {
	var cfg BenchmarkConfig
	cfg.Tamarind.Address = "http://localhost:8080"
	cfg.Receiver.Port = 9000
	cfg.Receiver.URL = "http://localhost:9000"
	cfg.Benchmark.Subscribers = 10000
	cfg.Benchmark.EventType = "privacy_policy.updated"
	cfg.Benchmark.BatchSize = 1000
	cfg.Redis.DB = 14
	return cfg
}

func LoadBenchmarkConfig(path string) (BenchmarkConfig, error) {
	cfg := defaultBenchmarkConfig()

	if path == "" {
		path = "benchmark/benchmark.yaml"
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return BenchmarkConfig{}, fmt.Errorf("read benchmark config %q: %w", path, err)
	}

	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return BenchmarkConfig{}, fmt.Errorf("parse benchmark config %q: %w", path, err)
	}

	err = validateBenchmarkConfig(cfg)
	if err != nil {
		return BenchmarkConfig{}, err
	}

	return cfg, nil
}

func validateBenchmarkConfig(cfg BenchmarkConfig) error {
	if cfg.Redis.Address == "" {
		return fmt.Errorf("redis.address is required (set it in benchmark.yaml)")
	}

	if cfg.Tamarind.Address == "" {
		return fmt.Errorf("tamarind.address is required")
	}

	if cfg.Receiver.URL == "" {
		return fmt.Errorf("receiver.url is required")
	}

	if cfg.Benchmark.Subscribers < 1 {
		return fmt.Errorf("benchmark.subscribers must be at least 1, got %d", cfg.Benchmark.Subscribers)
	}

	if cfg.Benchmark.EventType == "" {
		return fmt.Errorf("benchmark.event_type is required")
	}

	return nil
}
