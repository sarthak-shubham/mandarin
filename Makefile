.PHONY: help run test build docker-build up down logs clean bench-receiver bench-run

help:
	@echo "Tamarind - Lightweight Webhook Engine"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  run             Run Tamarind locally using tamarind.yaml"
	@echo "  test            Run the full automated test suite"
	@echo "  bench-receiver  Start mock webhook receiver (port 9000)"
	@echo "  bench-run       Run 10k fan-out benchmark using benchmark/benchmark.yaml"
	@echo "  build           Compile binary to bin/tamarind"
	@echo "  docker-build    Build the Docker image (tamarind:latest)"
	@echo "  up              Start Redis and Tamarind via Docker Compose"
	@echo "  down            Stop Docker Compose services"
	@echo "  logs            Tail Docker logs from Tamarind server"
	@echo "  clean           Remove local build binaries"

run:
	go run ./cmd/tamarind -config tamarind.yaml

bench-receiver:
	go run ./benchmark/cmd/benchreceiver -config benchmark/benchmark.yaml

bench-run:
	go run ./benchmark/cmd/benchrunner -config benchmark/benchmark.yaml

test:
	go test -v ./tests/...

build:
	go build -o bin/tamarind ./cmd/tamarind

docker-build:
	docker build -t tamarind:latest .

up:
	docker compose up -d

down:
	docker compose down

logs:
	docker compose logs -f tamarind

clean:
	rm -rf bin/ tamarind tamarind.exe
