.PHONY: help run test build docker-build up down logs clean

help:
	@echo "Tamarind - Lightweight Webhook Engine"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  run           Run Tamarind locally using tamarind.yaml"
	@echo "  test          Run the full automated test suite"
	@echo "  build         Compile binary to bin/tamarind"
	@echo "  docker-build  Build the Docker image (tamarind:latest)"
	@echo "  up            Start Redis and Tamarind via Docker Compose"
	@echo "  down          Stop Docker Compose services"
	@echo "  logs          Tail Docker logs from Tamarind server"
	@echo "  clean         Remove local build binaries"

run:
	go run ./cmd/tamarind -config tamarind.yaml

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
