# Setup and development

Mandarin needs a reachable Redis server. Redis can run locally, in a container, or on a separate host. Mandarin connects to Redis; it does not install or manage Redis.

## Requirements

- Go compatible with the `go 1.27.1` directive in `go.mod` to run from source.
- A reachable Redis server compatible with Mandarin's current standalone Redis client configuration.
- Git.
- Docker Engine and the Compose plugin, if you want to use the included container setup.

The current configuration accepts a Redis address, password, and logical database number. It connects directly to a Redis server; Redis Sentinel discovery and Redis Cluster are not supported by the current configuration. A managed Redis service can be used if it provides a reachable, compatible standalone endpoint.

---

## Run Mandarin

The root `docker-compose.yml` defines two services:

- `redis` runs `redis:7-alpine` and mounts the `redis_data` named volume at `/data`.
- `mandarin` builds the application image from the repository's `Dockerfile` and connects to `redis:6379` on the Compose network.

From the repository root, start both services:

```sh
docker compose up --build -d
docker compose ps
```

Check `http://localhost:8080/health`, then stop the services with:

```sh
docker compose down
```

Docker Compose manages `redis_data`, not Mandarin. `docker compose down` keeps the named volume; `docker compose down -v` removes it. Whether Redis data survives container replacement also depends on Redis persistence being configured to write data under `/data`.

### Use an existing Redis server

The Compose file is an example stack, not a requirement to run both services in Docker. If Redis is already running, configure `MANDARIN_REDIS_ADDRESS` with an address reachable from Mandarin, and provide `MANDARIN_REDIS_PASSWORD` and `MANDARIN_REDIS_DB` if needed. You can remove the `redis` service and `redis_data` volume from the Compose file if you do not want Compose to manage a Redis container. When Mandarin runs inside a container, `localhost` refers to that container, not your host or another container.

To run Mandarin from source while using the Compose Redis service, start Redis only and run the Go application:

```sh
docker compose up -d redis
go run ./cmd/mandarin -config mandarin.yaml
```

The root `mandarin.yaml` points to `localhost:6379`, which works when Redis is published on the host by Compose. For a Redis server at another address, change `redis.address` in the YAML file or override it with `MANDARIN_REDIS_ADDRESS`.

You can also run both services without Docker: install and start Redis separately, then run Mandarin from source and point its configuration at that Redis server. The address must be reachable from the machine or container running Mandarin.

### Run the published container image

A standalone Mandarin container does not start Redis. Replace `your-dockerhub-username` with the account that publishes the image, and use the desired published tag:

```sh
docker pull your-dockerhub-username/mandarin:latest
docker run --rm --name mandarin -p 8080:8080 \
  -e MANDARIN_REDIS_ADDRESS="redis.example.net:6379" \
  -e MANDARIN_REDIS_DB="0" \
  your-dockerhub-username/mandarin:latest
```

Replace `redis.example.net:6379` with the Redis address reachable from inside the container. Add `MANDARIN_REDIS_PASSWORD` if the server requires a password. The image's bundled YAML file uses `localhost:6379`, so set `MANDARIN_REDIS_ADDRESS` when Redis is elsewhere. The example uses POSIX shell syntax; in PowerShell, set environment variables with `$env:NAME = "value"` or pass them through Docker's `-e NAME=value` option as shown.

To build the image from this checkout instead, run `docker build -t mandarin:local .` and replace the published image name in the `docker run` command with `mandarin:local`.

### Quick reference (Makefile)

If you have `make` installed, common operational commands are available as targets:

| Target | Description | Equivalent Command |
| --- | --- | --- |
| `make up` | Start Redis and Mandarin in background | `docker compose up -d` |
| `make down` | Stop Compose services | `docker compose down` |
| `make run` | Run Mandarin locally against `mandarin.yaml` | `go run ./cmd/mandarin -config mandarin.yaml` |
| `make test` | Run all automated tests | `go test -v ./tests/...` |
| `make build` | Compile native binary to `bin/mandarin` | `go build -o bin/mandarin ./cmd/mandarin` |
| `make logs` | Tail logs from Mandarin container | `docker compose logs -f mandarin` |
| `make clean` | Remove local compiled binaries | `rm -rf bin/ mandarin mandarin.exe` |

---

## Configuration

Mandarin applies built-in defaults, then values from the YAML file passed with `-config`, then environment variables. Environment variables override matching YAML values. Without `-config`, Mandarin uses built-in defaults and environment variables. It does not load `.env` files automatically; set variables in the process environment or container configuration.

| Setting | Built-in default | Root `mandarin.yaml` | Environment variable |
| --- | --- | --- | --- |
| `server.port` | `8080` | `8080` | `MANDARIN_SERVER_PORT` |
| `redis.address` | Required; no default | `localhost:6379` | `MANDARIN_REDIS_ADDRESS` |
| `redis.password` | Empty | Empty | `MANDARIN_REDIS_PASSWORD` |
| `redis.db` | `0` | `0` | `MANDARIN_REDIS_DB` |
| `workers` | `10` | `10` | `MANDARIN_WORKERS` |
| `delivery_timeout` | `5s` | `5s` | `MANDARIN_DELIVERY_TIMEOUT` |
| `batch_size` | `1000` | `1000` | `MANDARIN_BATCH_SIZE` |

The Redis address is required if no YAML file supplies it. `localhost:6379` is a value in the local YAML file, not an application fallback.

---

## Run the tests

Tests use a real Redis server. Run them from the repository root:

```sh
go test -v -count=1 ./tests/...
```

To enable the race detector and write a coverage file:

```sh
go test -v -race -coverprofile=coverage.out ./tests/...
```

The tests cover API validation, endpoint and subscription operations, configuration, event status, webhook success and failure, deleted endpoints, and fan-out to multiple subscribers.

### Test database safety

The test helper reads `MANDARIN_TEST_REDIS_ADDRESS` (default `localhost:6379`) and `MANDARIN_TEST_REDIS_DB` (default `15`). Before each test setup, it runs `FLUSHDB` on the selected database; it runs `FLUSHDB` again during cleanup. This deletes **every key in that logical database**, not just keys created by the tests.

Use a dedicated, non-production Redis instance and a database containing no data you need to keep. DB 15 is isolated from the root configuration's DB 0 only if no other application uses DB 15. If the test helper cannot connect to Redis, it skips the test; check the output for skipped tests rather than treating them as passing.

---

## Run the benchmark

The benchmark seeds subscribers directly in Redis, publishes one event through Mandarin's HTTP API, waits for webhook delivery, and reports enqueue latency, delivery duration, throughput, and success/failure counts. It measures the fan-out and delivery path; it does not measure registering each endpoint through the public API.

Use a **dedicated, disposable, non-production Redis database**. The default is DB 14 in `benchmark/benchmark.yaml`, but cleanup also removes shared subscription-index keys and the `mandarin:deliveries` stream. Do not use a database containing data owned by other applications.

Start these programs in three separate terminals from the repository root.

**Terminal 1 — mock webhook receiver**

```sh
go run ./benchmark/cmd/benchreceiver -config benchmark/benchmark.yaml
# Or: make bench-receiver
```

**Terminal 2 — Mandarin with benchmark configuration**

```sh
go run ./cmd/mandarin -config benchmark/benchmark.yaml
# Or: make bench-server
```

**Terminal 3 — benchmark runner**

```sh
go run ./benchmark/cmd/benchrunner -config benchmark/benchmark.yaml
# Or: make bench-run
```

Use the benchmark configuration for Terminal 2. The root `mandarin.yaml` uses DB 0, while the benchmark configuration uses DB 14 by default. If the runner and server use different Redis databases, the runner will not find the seeded subscribers.

### Benchmark configuration and results

Edit `benchmark/benchmark.yaml` to configure a run:

| Setting | Default | Purpose |
| --- | --- | --- |
| `mandarin.address` | `http://localhost:8080` | Mandarin API address used by the runner |
| `redis.address` | `localhost:6379` | Redis address used by the runner and Mandarin |
| `redis.db` | `14` | Redis database for benchmark data |
| `receiver.port` | `9000` | Port where the mock receiver listens |
| `receiver.url` | `http://localhost:9000` | Base URL assigned to seeded endpoints |
| `receiver.delay` | `0s` | Artificial response delay |
| `receiver.failure_rate` | `0.0` | Fraction of requests the receiver should fail |
| `benchmark.subscribers` | `10000` | Number of seeded endpoints |
| `benchmark.event_type` | `privacy_policy.updated` | Event type for the broadcast |
| `benchmark.batch_size` | `1000` | Configured batch size |

The components communicate over HTTP and Redis. They can run on different hosts if every configured address is reachable from the process that uses it: `redis.address` must be reachable by the runner and Mandarin, `mandarin.address` by the runner, and `receiver.url` by Mandarin's workers. Update these values for your network; `localhost` is appropriate only when the relevant service is reachable on the same host or through the container's network configuration.

The runner prints a summary to the terminal and writes `benchmark/report.json` when it can. After a successful run, it removes benchmark data and unlinks the delivery stream. Workers in Terminal 2 will then log `NOGROUP` because the stream and consumer group no longer exist. This is expected with the current cleanup behaviour. Press `Ctrl+C` in Terminal 2, then restart it before the next run. Stop the receiver in Terminal 1 when finished.

---

## Related documentation

- [README](../README.md) — project overview.
- [Architecture](architecture.md) — system design and event flow.
- [Design decisions and trade-offs](decision.md) — delivery behaviour and current constraints.
