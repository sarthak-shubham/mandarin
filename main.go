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