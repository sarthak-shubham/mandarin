package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"mochi/internal/api"
	"mochi/internal/config"
	"mochi/internal/store"
	"mochi/internal/worker"
)

func main() {
	configPath := flag.String("config", "", "path to mochi.yaml (optional)")
	flag.Parse()

	cfg, err := config.Load(*configPath)
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

	s := store.NewStore(redisClient)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = worker.StartWorkers(ctx, s, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mochi: cannot start workers: %s\n", err)
		os.Exit(1)
	}

	server := api.NewServer(cfg, s)

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

	err = server.Shutdown(shutdownCtx)
	if err != nil {
		log.Fatalf("mochi: shutdown error: %s", err)
	}

	log.Println("mochi: stopped")
}

func connectRedis(cfg config.Config) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Address,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.Ping(ctx).Err()
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("PING failed (%s): %w", cfg.Redis.Address, err)
	}

	return client, nil
}

