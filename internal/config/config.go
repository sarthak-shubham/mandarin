package config

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

func Load(path string) (Config, error) {
	cfg := defaultConfig()

	if path != "" {
		err := mergeYAML(&cfg, path)
		if err != nil {
			return Config{}, err
		}
	}

	mergeEnv(&cfg)

	err := validate(cfg)
	if err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func mergeYAML(cfg *Config, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("cannot read config file %q: %w", path, err)
	}

	err = yaml.Unmarshal(data, cfg)
	if err != nil {
		return fmt.Errorf("cannot parse config file %q: %w", path, err)
	}

	return nil
}

func mergeEnv(cfg *Config) {
	portStr := os.Getenv("MOCHI_SERVER_PORT")
	if portStr != "" {
		n, err := strconv.Atoi(portStr)
		if err == nil {
			cfg.Server.Port = n
		}
	}

	redisAddr := os.Getenv("MOCHI_REDIS_ADDRESS")
	if redisAddr != "" {
		cfg.Redis.Address = redisAddr
	}

	redisPass := os.Getenv("MOCHI_REDIS_PASSWORD")
	if redisPass != "" {
		cfg.Redis.Password = redisPass
	}

	redisDB := os.Getenv("MOCHI_REDIS_DB")
	if redisDB != "" {
		n, err := strconv.Atoi(redisDB)
		if err == nil {
			cfg.Redis.DB = n
		}
	}

	workersStr := os.Getenv("MOCHI_WORKERS")
	if workersStr != "" {
		n, err := strconv.Atoi(workersStr)
		if err == nil {
			cfg.Workers = n
		}
	}

	timeoutStr := os.Getenv("MOCHI_DELIVERY_TIMEOUT")
	if timeoutStr != "" {
		d, err := time.ParseDuration(timeoutStr)
		if err == nil {
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

