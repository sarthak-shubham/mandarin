# Design Decisions, Trade-offs, and Boundaries

In production software engineering, no architecture is universally optimal. Every design choice is an explicit compromise: gaining performance, simplicity, or operational focus in one area invariably costs flexibility, persistence, or capabilities in another.

This document details the core design decisions made in Mandarin. It explains why each choice was made, what alternatives were considered, what was sacrificed, and the current operational boundaries of the system.

---

## 1. Key Architectural Decisions

### Decision 1: Redis Streams vs. Heavyweight Message Brokers (Kafka / RabbitMQ / SQS)

| Dimension | Mandarin's Choice: Redis Streams | Alternative: Dedicated Message Broker (Kafka / RabbitMQ) |
| --- | --- | --- |
| **Operational Complexity** | Single stateful dependency. Redis handles entity caching, indexing, and stream queueing. | Requires operating and monitoring a separate cluster (JVM, Zookeeper/KRaft, or Erlang runtime). |
| **Ingestion Latency** | Sub-millisecond (in-memory `XADD` executes in < 50 microseconds). | Higher latency overhead from disk sync barriers or distributed consensus protocols. |
| **Resource Footprint** | Extremely low (idle Redis consumes ~15–30 MB RAM). | Substantial (idle Kafka or RabbitMQ clusters consume hundreds of MB to GBs of memory). |
| **Storage Cost & Retention** | High memory cost. Messages reside in RAM and require periodic trimming (`XTRIM`). | Low disk cost. Kafka stores gigabytes of immutable logs on disk or S3 with tiered compaction. |

**Rationale:**  
Mandarin is designed to be lightweight and self-hostable. Requiring operators to deploy Kafka or RabbitMQ alongside a database would increase the operational barrier ten-fold. Redis provides atomic stream consumption via Consumer Groups (`XREADGROUP`) with microsecond latency while keeping the entire infrastructure footprint down to a single lightweight service.

---

### Decision 2: Asynchronous Dispatch (`202 Accepted`) vs. Synchronous Webhook Delivery

| Dimension | Mandarin's Choice: Asynchronous Dispatch | Alternative: Synchronous Delivery |
| --- | --- | --- |
| **Publisher Latency** | Constant time (10–20 ms), whether fanning out to 1 or 10,000 subscribers. | Linear with subscriber count and subscriber network latency ($O(N)$ network round-trips). |
| **Failure Isolation** | A slow or offline subscriber cannot block the publishing application. | A single timing-out subscriber can hang or crash the publisher's HTTP request thread. |
| **Delivery Confirmation** | Eventual. Publisher must query `/events/{id}` to verify status. | Immediate. Publisher receives final HTTP codes in the direct response. |

**Rationale:**  
The golden rule of webhook gateways is to never make the event publisher pay the latency penalty of third-party networks. External webhook destinations can be slow, degraded, or down. By persisting the event, queuing the delivery jobs, and returning `202 Accepted` immediately, Mandarin isolates the publishing service from subscriber network instability.

---

### Decision 3: In-Memory Redis Sets for Subscriptions vs. Relational SQL Databases

| Dimension | Mandarin's Choice: Redis Sets | Alternative: Relational Database (PostgreSQL / MySQL) |
| --- | --- | --- |
| **Fan-Out Lookup Time** | $O(1)$ memory lookup via `SMEMBERS event_type:<name>`. | $O(\log N)$ or table-join index scan, subject to connection pool queueing. |
| **Query Flexibility** | Primitive. Can only look up by exact key set membership. | High. Supports ad-hoc filtering, joins, range scans, and complex boolean queries. |
| **Transactional Guarantees** | Command pipelines; no cross-key rollback or ACID transactions. | Full ACID transactions with multi-table rollbacks. |

**Rationale:**  
During high-volume event broadcasts, resolving thousands of subscribers must be near-instantaneous. Redis Sets store subscriber IDs directly in RAM. Querying a topic index does not require query parsing, planner execution, or connection pool contention, enabling sub-millisecond subscriber resolution even under high load.

---

### Decision 4: Pipelined Micro-Batching (1,000 items) vs. Individual Commands vs. Monolithic Pipelines

| Dimension | Individual Commands | Monolithic Pipeline (All $N$) | Mandarin's Choice: Micro-Batching (1,000) |
| --- | --- | --- | --- |
| **TCP Round-Trips** | 1 round-trip per command (20,000 round-trips for 10k items). | Exactly 1 round-trip for the entire batch. | 10 round-trips for 10,000 items (1 per 1,000). |
| **Latency for 10k** | Several seconds of pure network wait. | ~100–200 ms. | ~500–600 ms. |
| **Redis Event Loop Impact** | Safe, but extremely inefficient. | **Dangerous**: Blocks Redis's single-threaded event loop for tens of milliseconds. | **Balanced**: Interleaves seamlessly with concurrent read/write traffic. |

**Rationale:**  
While sending all 20,000 commands in a single pipeline is theoretically fastest, it holds Redis's single-threaded execution loop captive, causing latency spikes for any other application sharing the Redis server. Chunking into batches of 1,000 achieves a 99.9% reduction in network round-trips while keeping the Redis event loop responsive to concurrent traffic.

---

### Decision 5: Bounded Worker Pool vs. Unconstrained Goroutines Per Event

| Dimension | Mandarin's Choice: Bounded Worker Pool | Alternative: `go deliverWebhook(...)` per Job |
| --- | --- | --- |
| **Resource Control** | Strictly capped by `MANDARIN_WORKERS` (default: 10). | Unbounded. Spawns 10,000 goroutines during a 10k fan-out. |
| **Socket / OS Safety** | Reuses existing connections via an internal HTTP transport pool. | High risk of file-descriptor exhaustion (`too many open files`) and CPU thrashing. |
| **Downstream Protection** | Rate-governed by the number of active workers. | Risk of inadvertently launching a denial-of-service (DDoS) attack on receiver endpoints. |

**Rationale:**  
Spawning a goroutine per webhook delivery is a common Go anti-pattern. While goroutines are lightweight (~2 KB), 50,000 simultaneous network requests will exhaust file descriptors, trigger DNS resolver timeouts, and overwhelm receiver servers. A bounded worker pool provides steady, predictable throughput and safe resource utilization.

---

### Decision 6: Standard Library Minimalism vs. Web Frameworks (Gin / Echo / Fiber)

| Dimension | Mandarin's Choice: Standard `net/http` | Alternative: Third-Party Web Frameworks |
| --- | --- | --- |
| **Dependencies** | Zero external HTTP dependencies. | Multiple transitive dependencies, external routing engines, and custom context wrappers. |
| **Longevity & Stability** | Protected by the Go 1 backward compatibility guarantee. | Subject to breaking changes across framework major version upgrades. |
| **Feature Set** | Standard `ServeMux`, standard `http.HandlerFunc`, standard context. | Built-in middleware suites, automatic binding, and route groups. |

**Rationale:**  
Mandarin’s REST API is focused: managing endpoints, managing subscriptions, publishing events, and querying status. Go 1.22+ introduced enhanced pattern matching in `net/http.ServeMux` (e.g. `POST /events`, `GET /events/{id}`), removing the need for external routers. Relying on the standard library guarantees long-term stability and security with zero supply-chain risk.

---

### Decision 7: Fail-Fast Configuration (No Default for Redis Address)

| Approach | Behavior | Risk / Impact |
| --- | --- | --- |
| **Permissive Fallback** | Fall back to `localhost:6379` if omitted. | In production or containers, Mandarin silently connects to the wrong host or fails unpredictably with misleading error logs. |
| **Fail-Fast (Mandarin)** | Require `redis.address` in YAML or `MANDARIN_REDIS_ADDRESS`. Exit immediately if missing. | Prevents misconfigured containers from booting. Misconfiguration is caught in seconds at deploy time. |

**Rationale:**  
`localhost:6379` is a valid address only for local desktop development. In containerized environments (Kubernetes, ECS, Docker Compose), Redis is almost always at a distinct network hostname (e.g., `redis:6379` or `elasticache.amazonaws.com`). Forcing explicit configuration eliminates silent, confusing runtime errors.

---

## 2. Current System Boundaries and Limitations

Mandarin focuses on delivering high throughput with minimal complexity. Before running it in mission-critical environments, operators should be aware of these current operational boundaries:

### 1. Single-Attempt Delivery (At-Most-Once Delivery)
- **Current Behavior**: Mandarin attempts each webhook delivery once. If the target server returns a 5xx error, times out, or drops the connection, Mandarin records `status: failed` with the HTTP status and failure reason.
- **Boundary**: There is currently no retry scheduler, exponential backoff, jitter calculation, or dead-letter queue (DLQ).
- **Implication**: If a subscriber experiences a temporary 10-second network blip during dispatch, they will not receive that event unless the publisher re-broadcasts it.

### 2. No Worker Crash Recovery Loop (Pending Entries List)
- **Current Behavior**: When a worker pulls a job with `XREADGROUP`, Redis moves the entry into that consumer's Pending Entries List (PEL). If the worker delivers the webhook and calls `XACK`, the job is removed from the PEL.
- **Boundary**: If a worker process abruptly crashes (e.g., `kill -9` or node power failure) while holding in-flight jobs, those entries remain in the PEL indefinitely. Mandarin does not currently run a background `XAUTOCLAIM` loop to reassign orphaned messages to healthy workers.

### 3. Lack of Publisher Idempotency Keys
- **Current Behavior**: Every call to `POST /events` creates a new unique event ID (UUIDv4) and schedules a new batch of deliveries.
- **Boundary**: Mandarin does not accept or track an `Idempotency-Key` header from publishers.
- **Implication**: If a network timeout occurs between the publishing application and Mandarin, and the publisher retries the POST request, duplicate webhooks will be scheduled and dispatched to all subscribers.

### 4. No Cryptographic Webhook Signing or Built-in Auth
- **Current Behavior**: Webhook requests are sent as plain JSON payloads. The ingestion API accepts requests from any client that can reach the port.
- **Boundary**: Mandarin does not currently compute HMAC-SHA256 signatures (e.g. `X-Mandarin-Signature`) using a shared secret, nor does it enforce API keys or bearer tokens on ingestion.
- **Implication**: Mandarin should be deployed within a private internal network or behind an API gateway (e.g., Kong, Envoy, Traefik, or Nginx) that handles TLS termination, authentication, and authorization.

### 5. Unbounded Redis Key Retention (No Automatic Expiration)
- **Current Behavior**: Event hashes (`event:<id>`), delivery records (`delivery:<ev>:<ep>`), and stream entries remain in Redis permanently.
- **Boundary**: Mandarin does not currently set Redis TTLs on completed deliveries or run automatic background stream compaction (`XTRIM`).
- **Implication**: Over months of sustained high-volume traffic, Redis RAM usage will grow linearly with the number of processed events until manual key unlinking or stream trimming is performed.

### 6. Standalone Redis Topology
- **Current Behavior**: Mandarin connects to a single Redis endpoint via a standard Redis client connection.
- **Boundary**: The current configuration does not support Redis Sentinel failover discovery or Redis Cluster hash-slot routing.
- **Implication**: High availability depends on managed standalone Redis hosting (e.g., AWS ElastiCache, GCP Memorystore, or Azure Cache for Redis) that handles primary-replica failover transparently behind a single DNS endpoint.

---

## 3. Summary Matrix

| Capability | Current Status | Primary Benefit | Primary Trade-off |
| --- | --- | --- | --- |
| **Delivery Model** | At-most-once (single attempt) | Predictable, non-blocking throughput | No automatic retries on receiver failure |
| **Topic Storage** | Redis Sets in RAM | Sub-millisecond subscriber lookups | Cannot execute complex relational SQL queries |
| **Message Queue** | Redis Streams | Single stateful service, microsecond latency | High RAM cost compared to disk-backed brokers |
| **Fan-Out Dispatch** | Pipelined batches of 1,000 | 99.9% fewer network round-trips | Does not saturate single-thread Redis loop |
| **Dispatcher Model** | Bounded Go worker pool | Protects local and remote sockets/CPUs | Fixed upper bound on concurrent outbound HTTP connections |
| **Security & Auth** | Externalized to API Gateway / Private VPC | Zero framework complexity, standard library purity | No native HMAC payload signing or API token enforcement |
| **Configuration** | Strict fail-fast invariants | Zero ambiguous fallback failures | Ingestion refuses to boot without explicit Redis address |

---

## Related Documentation

- [README](../README.md) — Project overview, capabilities, and performance snapshot.
- [Architecture](architecture.md) — System design, components, Redis data schema, and event lifecycle.
- [Setup and Development](setup.md) — Local installation, Docker Compose, automated testing, and benchmarks.
