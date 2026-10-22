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
	BatchSize       int           `yaml:"batch_size"`
}

func defaultConfig() Config {
	var c Config
	c.Server.Port = 8080
	c.Workers = 10
	c.DeliveryTimeout = 5 * time.Second
	c.BatchSize = 1000
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
	portStr := os.Getenv("MANDARIN_SERVER_PORT")
	if portStr != "" {
		n, err := strconv.Atoi(portStr)
		if err == nil {
			cfg.Server.Port = n
		}
	}

	redisAddr := os.Getenv("MANDARIN_REDIS_ADDRESS")
	if redisAddr != "" {
		cfg.Redis.Address = redisAddr
	}

	redisPass := os.Getenv("MANDARIN_REDIS_PASSWORD")
	if redisPass != "" {
		cfg.Redis.Password = redisPass
	}

	redisDB := os.Getenv("MANDARIN_REDIS_DB")
	if redisDB != "" {
		n, err := strconv.Atoi(redisDB)
		if err == nil {
			cfg.Redis.DB = n
		}
	}

	workersStr := os.Getenv("MANDARIN_WORKERS")
	if workersStr != "" {
		n, err := strconv.Atoi(workersStr)
		if err == nil {
			cfg.Workers = n
		}
	}

	timeoutStr := os.Getenv("MANDARIN_DELIVERY_TIMEOUT")
	if timeoutStr != "" {
		d, err := time.ParseDuration(timeoutStr)
		if err == nil {
			cfg.DeliveryTimeout = d
		}
	}

	batchStr := os.Getenv("MANDARIN_BATCH_SIZE")
	if batchStr != "" {
		n, err := strconv.Atoi(batchStr)
		if err == nil {
			cfg.BatchSize = n
		}
	}
}
