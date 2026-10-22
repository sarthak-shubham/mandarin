.PHONY: help run test build docker-build up down logs clean bench-receiver bench-server bench-run

help:
	@echo "Mandarin - High-Performance Webhook Fan-Out Engine"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  run             Run Mandarin locally using mandarin.yaml (DB 0)"
	@echo "  test            Run the full automated test suite (DB 15)"
	@echo "  bench-receiver  Start mock webhook receiver (port 9000)"
	@echo "  bench-server    Start Mandarin engine configured for benchmarks (DB 14)"
	@echo "  bench-run       Run 10k fan-out benchmark using benchmark/benchmark.yaml"
	@echo "  build           Compile binary to bin/mandarin"
	@echo "  docker-build    Build the Docker image (mandarin:latest)"
	@echo "  up              Start Redis and Mandarin via Docker Compose"
	@echo "  down            Stop Docker Compose services"
	@echo "  logs            Tail Docker logs from Mandarin server"
	@echo "  clean           Remove local build binaries"

run:
	go run ./cmd/mandarin -config mandarin.yaml

bench-receiver:
	go run ./benchmark/cmd/benchreceiver -config benchmark/benchmark.yaml

bench-server:
	go run ./cmd/mandarin -config benchmark/benchmark.yaml

bench-run:
	go run ./benchmark/cmd/benchrunner -config benchmark/benchmark.yaml

test:
	go test -v ./tests/...

build:
	go build -o bin/mandarin ./cmd/mandarin

docker-build:
	docker build -t mandarin:latest .

up:
	docker compose up -d

down:
	docker compose down

logs:
	docker compose logs -f mandarin

clean:
	rm -rf bin/ mandarin mandarin.exe
