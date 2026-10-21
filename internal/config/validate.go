package config

import "fmt"

func validate(cfg Config) error {
	if cfg.Redis.Address == "" {
		return fmt.Errorf("redis.address is required (set it in the config file or via TAMARIND_REDIS_ADDRESS)")
	}

	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		return fmt.Errorf("server.port must be between 1 and 65535, got %d", cfg.Server.Port)
	}

	if cfg.Workers < 1 {
		return fmt.Errorf("workers must be at least 1, got %d", cfg.Workers)
	}

	if cfg.DeliveryTimeout <= 0 {
		return fmt.Errorf("delivery_timeout must be greater than zero, got %s", cfg.DeliveryTimeout)
	}

	if cfg.BatchSize <= 0 {
		return fmt.Errorf("batch_size must be at least 1, got %d", cfg.BatchSize)
	}

	return nil
}
