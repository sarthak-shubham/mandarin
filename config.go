package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server struct {
		Port int `yaml:"port"`
	} `yaml:"server"`

	Redis struct {
		Address  string `yaml:"address"`
		Password string `yaml:"password"`
		DB       int    `yaml:"db"`
	} `yaml:"redis"`

	Workers         int           `yaml:"workers"`
	DeliveryTimeout time.Duration `yaml:"delivery_timeout"`
}

func defaultConfig() Config {
	var c Config
	c.Server.Port = 8080
	c.Workers = 10
	c.DeliveryTimeout = 5 * time.Second
	return c
}

func loadConfig(path string) (Config, error) {
	cfg := defaultConfig()

	if path != "" {
		if err := mergeYAML(&cfg, path); err != nil {
			return Config{}, err
		}
	}

	mergeEnv(&cfg)

	if err := validate(cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func mergeYAML(cfg *Config, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("cannot read config file %q: %w", path, err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("cannot parse config file %q: %w", path, err)
	}

	return nil
}

func mergeEnv(cfg *Config) {
	if v := os.Getenv("MOCHI_SERVER_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = n
		}
	}

	if v := os.Getenv("MOCHI_REDIS_ADDRESS"); v != "" {
		cfg.Redis.Address = v
	}

	if v := os.Getenv("MOCHI_REDIS_PASSWORD"); v != "" {
		cfg.Redis.Password = v
	}

	if v := os.Getenv("MOCHI_REDIS_DB"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Redis.DB = n
		}
	}

	if v := os.Getenv("MOCHI_WORKERS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Workers = n
		}
	}

	if v := os.Getenv("MOCHI_DELIVERY_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.DeliveryTimeout = d
		}
	}
}

func validate(cfg Config) error {
	if cfg.Redis.Address == "" {
		return fmt.Errorf("redis.address is required (set it in the config file or via MOCHI_REDIS_ADDRESS)")
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

	return nil
}
