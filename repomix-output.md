This file is a merged representation of the entire codebase, combined into a single document by Repomix.

# File Summary

## Purpose
This file contains a packed representation of the entire repository's contents.
It is designed to be easily consumable by AI systems for analysis, code review,
or other automated processes.

## File Format
The content is organized as follows:
1. This summary section
2. Repository information
3. Directory structure
4. Repository files (if enabled)
5. Multiple file entries, each consisting of:
  a. A header with the file path (## File: path/to/file)
  b. The full contents of the file in a code block

## Usage Guidelines
- This file should be treated as read-only. Any changes should be made to the
  original repository files, not this packed version.
- When processing this file, use the file path to distinguish
  between different files in the repository.
- Be aware that this file may contain sensitive information. Handle it with
  the same level of security as you would the original repository.

## Notes
- Some files may have been excluded based on .gitignore rules and Repomix's configuration
- Binary files are not included in this packed representation. Please refer to the Repository Structure section for a complete list of file paths, including binary files
- Files matching patterns in .gitignore are excluded
- Files matching default ignore patterns are excluded
- Files are sorted by Git change count (files with more changes are at the bottom)

# Directory Structure
```
config.go
endpoint_store.go
endpoints.go
event_store.go
events.go
go.mod
handlers.go
main.go
mochi.exe
store.go
worker.go
```

# Files

## File: config.go
```go
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
```

## File: endpoint_store.go
```go
package main

import (
	"context"
	"fmt"
	"time"
)

type Endpoint struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Store) RegisterEndpoint(ctx context.Context, ep Endpoint) error {
	if err := s.saveEndpoint(ctx, ep); err != nil {
		return err
	}
	return s.redis.SAdd(ctx, allEndpointsKey(), ep.ID).Err()
}

func (s *Store) GetEndpoint(ctx context.Context, id string) (*Endpoint, error) {
	data, err := s.redis.HGetAll(ctx, endpointKey(id)).Result()
	if err != nil {
		return nil, fmt.Errorf("get endpoint %q: %w", id, err)
	}
	if len(data) == 0 {
		return nil, nil
	}

	createdAt, err := time.Parse(time.RFC3339, data["created_at"])
	if err != nil {
		return nil, fmt.Errorf("parse created_at for endpoint %q: %w", id, err)
	}

	return &Endpoint{
		ID:        data["id"],
		URL:       data["url"],
		CreatedAt: createdAt,
	}, nil
}

func (s *Store) ListEndpoints(ctx context.Context) ([]Endpoint, error) {
	ids, err := s.redis.SMembers(ctx, allEndpointsKey()).Result()
	if err != nil {
		return nil, fmt.Errorf("list endpoint IDs: %w", err)
	}

	endpoints := make([]Endpoint, 0, len(ids))
	for _, id := range ids {
		ep, err := s.GetEndpoint(ctx, id)
		if err != nil {
			return nil, err
		}
		if ep != nil {
			endpoints = append(endpoints, *ep)
		}
	}

	return endpoints, nil
}

func (s *Store) DeleteEndpoint(ctx context.Context, id string) error {
	eventTypes, err := s.redis.SMembers(ctx, subscriptionsKey(id)).Result()
	if err != nil {
		return fmt.Errorf("read subscriptions for endpoint %q: %w", id, err)
	}

	pipe := s.redis.Pipeline()
	pipe.Del(ctx, endpointKey(id))
	pipe.Del(ctx, subscriptionsKey(id))
	pipe.SRem(ctx, allEndpointsKey(), id)
	for _, et := range eventTypes {
		pipe.SRem(ctx, eventTypeKey(et), id)
	}
	_, err = pipe.Exec(ctx)
	return err
}

func (s *Store) Subscribe(ctx context.Context, endpointID, eventType string) error {
	pipe := s.redis.Pipeline()
	pipe.SAdd(ctx, subscriptionsKey(endpointID), eventType)
	pipe.SAdd(ctx, eventTypeKey(eventType), endpointID)
	_, err := pipe.Exec(ctx)
	return err
}

func (s *Store) Unsubscribe(ctx context.Context, endpointID, eventType string) error {
	pipe := s.redis.Pipeline()
	pipe.SRem(ctx, subscriptionsKey(endpointID), eventType)
	pipe.SRem(ctx, eventTypeKey(eventType), endpointID)
	_, err := pipe.Exec(ctx)
	return err
}

func (s *Store) EndpointsForEventType(ctx context.Context, eventType string) ([]string, error) {
	ids, err := s.redis.SMembers(ctx, eventTypeKey(eventType)).Result()
	if err != nil {
		return nil, fmt.Errorf("endpoints for event type %q: %w", eventType, err)
	}
	return ids, nil
}

func (s *Store) saveEndpoint(ctx context.Context, ep Endpoint) error {
	return s.redis.HSet(ctx, endpointKey(ep.ID),
		"id", ep.ID,
		"url", ep.URL,
		"created_at", ep.CreatedAt.Format(time.RFC3339),
	).Err()
}
```

## File: endpoints.go
```go
package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
)

func registerEndpointHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			URL string `json:"url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "request body must be valid JSON")
			return
		}
		if !isValidURL(body.URL) {
			writeError(w, http.StatusBadRequest, "url is missing or invalid")
			return
		}

		ep := Endpoint{
			ID:        uuid.NewString(),
			URL:       body.URL,
			CreatedAt: time.Now().UTC(),
		}
		if err := store.RegisterEndpoint(r.Context(), ep); err != nil {
			writeError(w, http.StatusInternalServerError, "could not save endpoint")
			return
		}

		writeJSON(w, http.StatusCreated, ep)
	}
}

func listEndpointsHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		endpoints, err := store.ListEndpoints(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not list endpoints")
			return
		}

		writeJSON(w, http.StatusOK, endpoints)
	}
}

func deleteEndpointHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		ep, err := store.GetEndpoint(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not look up endpoint")
			return
		}
		if ep == nil {
			writeError(w, http.StatusNotFound, "endpoint not found")
			return
		}

		if err := store.DeleteEndpoint(r.Context(), id); err != nil {
			writeError(w, http.StatusInternalServerError, "could not delete endpoint")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func subscribeHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		ep, err := store.GetEndpoint(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not look up endpoint")
			return
		}
		if ep == nil {
			writeError(w, http.StatusNotFound, "endpoint not found")
			return
		}

		var body struct {
			EventType string `json:"event_type"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.EventType == "" {
			writeError(w, http.StatusBadRequest, "event_type is required")
			return
		}

		if err := store.Subscribe(r.Context(), id, body.EventType); err != nil {
			writeError(w, http.StatusInternalServerError, "could not save subscription")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func unsubscribeHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		eventType := r.PathValue("event_type")

		ep, err := store.GetEndpoint(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not look up endpoint")
			return
		}
		if ep == nil {
			writeError(w, http.StatusNotFound, "endpoint not found")
			return
		}

		if err := store.Unsubscribe(r.Context(), id, eventType); err != nil {
			writeError(w, http.StatusInternalServerError, "could not remove subscription")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
```

## File: event_store.go
```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	StatePending = "pending"
	StateSuccess = "success"
	StateFailed  = "failed"
)

type Event struct {
	ID          string          `json:"id"`
	EventType   string          `json:"event_type"`
	PublishedAt time.Time       `json:"published_at"`
	Payload     json.RawMessage `json:"payload"`
}

type Delivery struct {
	EventID       string `json:"event_id"`
	EndpointID    string `json:"endpoint_id"`
	State         string `json:"state"`
	HTTPStatus    int    `json:"http_status,omitempty"`
	FailureReason string `json:"failure_reason,omitempty"`
	DurationMS    int64  `json:"duration_ms,omitempty"`
}

func (s *Store) SaveEvent(ctx context.Context, ev Event) error {
	return s.redis.HSet(ctx, eventKey(ev.ID),
		"id", ev.ID,
		"event_type", ev.EventType,
		"published_at", ev.PublishedAt.Format(time.RFC3339),
		"payload", string(ev.Payload),
	).Err()
}

func (s *Store) GetEvent(ctx context.Context, id string) (*Event, error) {
	data, err := s.redis.HGetAll(ctx, eventKey(id)).Result()
	if err != nil {
		return nil, fmt.Errorf("get event %q: %w", id, err)
	}
	if len(data) == 0 {
		return nil, nil
	}

	publishedAt, err := time.Parse(time.RFC3339, data["published_at"])
	if err != nil {
		return nil, fmt.Errorf("parse published_at for event %q: %w", id, err)
	}

	return &Event{
		ID:          data["id"],
		EventType:   data["event_type"],
		PublishedAt: publishedAt,
		Payload:     json.RawMessage(data["payload"]),
	}, nil
}

func (s *Store) SaveDelivery(ctx context.Context, d Delivery) error {
	pipe := s.redis.Pipeline()
	pipe.HSet(ctx, deliveryKey(d.EventID, d.EndpointID),
		"event_id", d.EventID,
		"endpoint_id", d.EndpointID,
		"state", d.State,
		"http_status", d.HTTPStatus,
		"failure_reason", d.FailureReason,
		"duration_ms", d.DurationMS,
	)
	pipe.SAdd(ctx, eventDeliveriesKey(d.EventID), d.EndpointID)
	_, err := pipe.Exec(ctx)
	return err
}

func (s *Store) GetDelivery(ctx context.Context, eventID, endpointID string) (*Delivery, error) {
	data, err := s.redis.HGetAll(ctx, deliveryKey(eventID, endpointID)).Result()
	if err != nil {
		return nil, fmt.Errorf("get delivery %q/%q: %w", eventID, endpointID, err)
	}
	if len(data) == 0 {
		return nil, nil
	}

	var httpStatus int
	fmt.Sscanf(data["http_status"], "%d", &httpStatus)

	var durationMS int64
	fmt.Sscanf(data["duration_ms"], "%d", &durationMS)

	return &Delivery{
		EventID:       data["event_id"],
		EndpointID:    data["endpoint_id"],
		State:         data["state"],
		HTTPStatus:    httpStatus,
		FailureReason: data["failure_reason"],
		DurationMS:    durationMS,
	}, nil
}

func (s *Store) GetDeliveriesForEvent(ctx context.Context, eventID string) ([]Delivery, error) {
	endpointIDs, err := s.redis.SMembers(ctx, eventDeliveriesKey(eventID)).Result()
	if err != nil {
		return nil, fmt.Errorf("list deliveries for event %q: %w", eventID, err)
	}

	deliveries := make([]Delivery, 0, len(endpointIDs))
	for _, epID := range endpointIDs {
		d, err := s.GetDelivery(ctx, eventID, epID)
		if err != nil {
			return nil, err
		}
		if d != nil {
			deliveries = append(deliveries, *d)
		}
	}

	return deliveries, nil
}

func (s *Store) EnqueueDelivery(ctx context.Context, eventID, endpointID string) error {
	return s.redis.XAdd(ctx, &redis.XAddArgs{
		Stream: deliveryStreamKey(),
		Values: map[string]any{
			"event_id":    eventID,
			"endpoint_id": endpointID,
		},
	}).Err()
}
```

## File: events.go
```go
package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
)

func publishEventHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			EventType string          `json:"event_type"`
			Payload   json.RawMessage `json:"payload"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "request body must be valid JSON")
			return
		}
		if body.EventType == "" {
			writeError(w, http.StatusBadRequest, "event_type is required")
			return
		}
		if len(body.Payload) == 0 || !json.Valid(body.Payload) {
			writeError(w, http.StatusBadRequest, "payload must be valid JSON")
			return
		}

		ev := Event{
			ID:          uuid.NewString(),
			EventType:   body.EventType,
			PublishedAt: time.Now().UTC(),
			Payload:     body.Payload,
		}

		if err := store.SaveEvent(r.Context(), ev); err != nil {
			writeError(w, http.StatusInternalServerError, "could not save event")
			return
		}

		endpointIDs, err := store.EndpointsForEventType(r.Context(), ev.EventType)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not look up subscribers")
			return
		}

		for _, endpointID := range endpointIDs {
			delivery := Delivery{
				EventID:    ev.ID,
				EndpointID: endpointID,
				State:      StatePending,
			}
			if err := store.SaveDelivery(r.Context(), delivery); err != nil {
				writeError(w, http.StatusInternalServerError, "could not save delivery")
				return
			}
			if err := store.EnqueueDelivery(r.Context(), ev.ID, endpointID); err != nil {
				writeError(w, http.StatusInternalServerError, "could not enqueue delivery")
				return
			}
		}

		writeJSON(w, http.StatusAccepted, map[string]string{"id": ev.ID})
	}
}

func getEventHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		ev, err := store.GetEvent(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not look up event")
			return
		}
		if ev == nil {
			writeError(w, http.StatusNotFound, "event not found")
			return
		}

		deliveries, err := store.GetDeliveriesForEvent(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not look up deliveries")
			return
		}

		pending, success, failed := countDeliveries(deliveries)

		writeJSON(w, http.StatusOK, map[string]any{
			"event": ev,
			"summary": map[string]int{
				"pending": pending,
				"success": success,
				"failed":  failed,
			},
			"deliveries": deliveries,
		})
	}
}

func countDeliveries(deliveries []Delivery) (pending, success, failed int) {
	for _, d := range deliveries {
		switch d.State {
		case StatePending:
			pending++
		case StateSuccess:
			success++
		case StateFailed:
			failed++
		}
	}
	return
}
```

## File: go.mod
```
module mochi

go 1.27.1

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/redis/go-redis/v9 v9.23.0 // indirect
	go.uber.org/atomic v1.12.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
```

## File: handlers.go
```go
package main

import (
	"encoding/json"
	"net/http"
	"net/url"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func isValidURL(raw string) bool {
	if raw == "" {
		return false
	}
	u, err := url.ParseRequestURI(raw)
	if err != nil {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}
```

## File: main.go
```go
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
)

func main() {
	configPath := flag.String("config", "", "path to mochi.yaml (optional)")
	flag.Parse()

	cfg, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mochi: invalid configuration: %s\n", err)
		os.Exit(1)
	}

	redisClient, err := connectRedis(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mochi: cannot connect to Redis: %s\n", err)
		os.Exit(1)
	}
	defer redisClient.Close()

	store := NewStore(redisClient)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := StartWorkers(ctx, store, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "mochi: cannot start workers: %s\n", err)
		os.Exit(1)
	}

	server := buildServer(cfg, store)

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("mochi: listening on port %d", cfg.Server.Port)
		serverErr <- server.ListenAndServe()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		log.Fatalf("mochi: server error: %s", err)
	case sig := <-quit:
		log.Printf("mochi: received %s, shutting down", sig)
	}

	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("mochi: shutdown error: %s", err)
	}

	log.Println("mochi: stopped")
}

func buildServer(cfg Config, store *Store) *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", healthHandler)

	mux.HandleFunc("POST /endpoints", registerEndpointHandler(store))
	mux.HandleFunc("GET /endpoints", listEndpointsHandler(store))
	mux.HandleFunc("DELETE /endpoints/{id}", deleteEndpointHandler(store))

	mux.HandleFunc("POST /endpoints/{id}/subscriptions", subscribeHandler(store))
	mux.HandleFunc("DELETE /endpoints/{id}/subscriptions/{event_type}", unsubscribeHandler(store))

	mux.HandleFunc("POST /events", publishEventHandler(store))
	mux.HandleFunc("GET /events/{id}", getEventHandler(store))

	return &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: mux,
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func connectRedis(cfg Config) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Address,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("PING failed (%s): %w", cfg.Redis.Address, err)
	}

	return client, nil
}
```

## File: store.go
```go
package main

import "github.com/redis/go-redis/v9"

type Store struct {
	redis *redis.Client
}

func NewStore(client *redis.Client) *Store {
	return &Store{redis: client}
}

func endpointKey(id string) string                  { return "endpoint:" + id }
func allEndpointsKey() string                       { return "endpoints:all" }
func subscriptionsKey(id string) string             { return "subscriptions:" + id }
func eventTypeKey(eventType string) string          { return "event_type:" + eventType }
func eventKey(id string) string                     { return "event:" + id }
func deliveryKey(eventID, endpointID string) string { return "delivery:" + eventID + ":" + endpointID }
func eventDeliveriesKey(eventID string) string      { return "event_deliveries:" + eventID }
func deliveryStreamKey() string                     { return "mochi:deliveries" }
```

## File: worker.go
```go
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const consumerGroupName = "mochi-workers"

func StartWorkers(ctx context.Context, store *Store, cfg Config) error {
	err := store.redis.XGroupCreateMkStream(ctx, deliveryStreamKey(), consumerGroupName, "$").Err()
	if err != nil && !isGroupExistsError(err) {
		return fmt.Errorf("create consumer group: %w", err)
	}

	httpClient := &http.Client{Timeout: cfg.DeliveryTimeout}

	for i := 0; i < cfg.Workers; i++ {
		workerID := fmt.Sprintf("worker-%d", i)
		go runWorker(ctx, store, httpClient, workerID)
	}

	return nil
}

func runWorker(ctx context.Context, store *Store, client *http.Client, workerID string) {
	for ctx.Err() == nil {
		msgs, err := store.redis.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    consumerGroupName,
			Consumer: workerID,
			Streams:  []string{deliveryStreamKey(), ">"},
			Count:    1,
			Block:    2 * time.Second,
		}).Result()

		if err == redis.Nil {
			continue
		}
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("worker %s: read error: %s", workerID, err)
			continue
		}

		for _, stream := range msgs {
			for _, msg := range stream.Messages {
				processMessage(ctx, store, client, workerID, msg)
			}
		}
	}
}

func processMessage(ctx context.Context, store *Store, client *http.Client, workerID string, msg redis.XMessage) {
	eventID, _ := msg.Values["event_id"].(string)
	endpointID, _ := msg.Values["endpoint_id"].(string)

	ev, err := store.GetEvent(ctx, eventID)
	if err != nil || ev == nil {
		markFailed(ctx, store, eventID, endpointID, "event not found")
		ack(ctx, store, msg.ID)
		return
	}

	ep, err := store.GetEndpoint(ctx, endpointID)
	if err != nil || ep == nil {
		markFailed(ctx, store, eventID, endpointID, "endpoint not found")
		ack(ctx, store, msg.ID)
		return
	}

	result := sendWebhook(client, ep.URL, ev)
	result.EventID = eventID
	result.EndpointID = endpointID

	if err := store.SaveDelivery(ctx, result); err != nil {
		log.Printf("worker %s: save delivery %s/%s: %s", workerID, eventID, endpointID, err)
	}

	ack(ctx, store, msg.ID)
}

func sendWebhook(client *http.Client, targetURL string, ev *Event) Delivery {
	body, err := json.Marshal(map[string]any{
		"id":         ev.ID,
		"event_type": ev.EventType,
		"timestamp":  ev.PublishedAt,
		"payload":    ev.Payload,
	})
	if err != nil {
		return Delivery{State: StateFailed, FailureReason: "could not marshal payload"}
	}

	start := time.Now()
	resp, err := client.Post(targetURL, "application/json", bytes.NewReader(body))
	elapsed := time.Since(start).Milliseconds()

	if err != nil {
		return Delivery{
			State:         StateFailed,
			FailureReason: err.Error(),
			DurationMS:    elapsed,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return Delivery{
			State:      StateSuccess,
			HTTPStatus: resp.StatusCode,
			DurationMS: elapsed,
		}
	}

	return Delivery{
		State:         StateFailed,
		HTTPStatus:    resp.StatusCode,
		FailureReason: fmt.Sprintf("non-2xx response: %d", resp.StatusCode),
		DurationMS:    elapsed,
	}
}

func markFailed(ctx context.Context, store *Store, eventID, endpointID, reason string) {
	d := Delivery{
		EventID:       eventID,
		EndpointID:    endpointID,
		State:         StateFailed,
		FailureReason: reason,
	}
	if err := store.SaveDelivery(ctx, d); err != nil {
		log.Printf("markFailed %s/%s: %s", eventID, endpointID, err)
	}
}

func ack(ctx context.Context, store *Store, msgID string) {
	store.redis.XAck(ctx, deliveryStreamKey(), consumerGroupName, msgID)
}

func isGroupExistsError(err error) bool {
	return strings.Contains(err.Error(), "BUSYGROUP")
}
```
