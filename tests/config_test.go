package tests

import (
	"testing"
	"time"

	"github.com/sarthak-shubham/mandarin/internal/config"
)

func TestConfig_Defaults(t *testing.T) {
	t.Setenv("MANDARIN_REDIS_ADDRESS", "localhost:6379")

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("expected successful load, got error: %s", err)
	}

	if cfg.Server.Port != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Workers != 10 {
		t.Errorf("expected 10 workers, got %d", cfg.Workers)
	}
	if cfg.DeliveryTimeout != 5*time.Second {
		t.Errorf("expected 5s timeout, got %s", cfg.DeliveryTimeout)
	}
	if cfg.BatchSize != 1000 {
		t.Errorf("expected 1000 batch size, got %d", cfg.BatchSize)
	}
}

func TestConfig_Validation(t *testing.T) {
	_, err := config.Load("")
	if err == nil {
		t.Error("expected error for missing redis address, got nil")
	}

	t.Setenv("MANDARIN_REDIS_ADDRESS", "localhost:6379")
	t.Setenv("MANDARIN_SERVER_PORT", "99999")
	_, err = config.Load("")
	if err == nil {
		t.Error("expected error for invalid port > 65535, got nil")
	}

	t.Setenv("MANDARIN_SERVER_PORT", "8080")
	t.Setenv("MANDARIN_WORKERS", "0")
	_, err = config.Load("")
	if err == nil {
		t.Error("expected error for workers < 1, got nil")
	}
}

func TestConfig_EnvOverrides(t *testing.T) {
	t.Setenv("MANDARIN_REDIS_ADDRESS", "redis.internal:6380")
	t.Setenv("MANDARIN_SERVER_PORT", "9090")
	t.Setenv("MANDARIN_WORKERS", "25")
	t.Setenv("MANDARIN_DELIVERY_TIMEOUT", "10s")
	t.Setenv("MANDARIN_BATCH_SIZE", "500")

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("expected success, got error: %s", err)
	}

	if cfg.Redis.Address != "redis.internal:6380" {
		t.Errorf("expected address redis.internal:6380, got %s", cfg.Redis.Address)
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("expected port 9090, got %d", cfg.Server.Port)
	}
	if cfg.Workers != 25 {
		t.Errorf("expected 25 workers, got %d", cfg.Workers)
	}
	if cfg.DeliveryTimeout != 10*time.Second {
		t.Errorf("expected 10s timeout, got %s", cfg.DeliveryTimeout)
	}
	if cfg.BatchSize != 500 {
		t.Errorf("expected 500 batch size, got %d", cfg.BatchSize)
	}
}
