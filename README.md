<p align="left">
  <img src="assets/images/mandarin_logo.svg" alt="Mandarin" width="280" />
</p>

<p align="left">
  <a href="https://github.com/sarthak-shubham/mandarin/actions/workflows/ci.yml"><img alt="CI status" src="https://github.com/sarthak-shubham/mandarin/actions/workflows/ci.yml/badge.svg?branch=main" /></a>
  <a href="go.mod"><img alt="Go version" src="https://img.shields.io/github/go-mod/go-version/sarthak-shubham/mandarin" /></a>
  <a href="LICENSE"><img alt="License" src="https://img.shields.io/github/license/sarthak-shubham/mandarin" /></a>
</p>

Mandarin is a self-hosted webhook fan-out gateway written in Go. A publishing application sends an event once, and Mandarin schedules a webhook delivery for each registered endpoint subscribed to that event type.

For example, an application can publish `privacy_policy.updated`. Mandarin finds the endpoints subscribed to that event type and sends each endpoint a webhook containing the event details and payload. The publishing application does not need to make a separate request to every subscriber.

## What Mandarin provides

- **Endpoint management.** Register webhook URLs and subscribe them to event types.
- **Event publishing.** Accept JSON events through an HTTP API.
- **Asynchronous delivery.** Store delivery jobs in Redis and process them in the background with a configurable Go worker pool.
- **Delivery status.** Record whether each delivery is pending, successful, or failed, along with available HTTP status and timing information.
- **Batched dispatch.** Schedule large groups of deliveries through Redis pipelines rather than issuing each Redis command separately.

Publishing an event returns `202 Accepted` after Mandarin has scheduled the matching deliveries. It does not mean every subscriber has received the webhook. The event API can be used to inspect delivery progress.

## Where to go next

**Using Mandarin**

For consumer-facing instructions—running the container image or Docker Compose setup, configuring Redis, and sending requests—see the [Mandarin GitHub Wiki](https://github.com/sarthak-shubham/mandarin/wiki).

**Developing Mandarin**

After cloning the repository, start with these guides:

- [Setup and development](docs/setup.md) — local setup, configuration, tests, benchmarks, and troubleshooting.
- [Architecture](docs/architecture.md) — components, event flow, Redis data model, design decisions, and scaling assumptions.
- [Design decisions and trade-offs](docs/decision.md) — delivery behaviour, failure modes, constraints, and the costs of design choices.

## Performance snapshot

One reported local benchmark broadcast an event to 10,000 subscriber endpoints:

| Measurement | Result |
|---|---:|
| Time to enqueue deliveries | 586 ms |
| Delivery duration | 24.68 s |
| Delivery throughput | 405.12 webhooks/s |
| Successful deliveries | 10,000 of 10,000 |

The run used Windows 11, an Intel Core i3-8145U, 8 GB RAM, Redis 7 in Docker, loopback networking, 10 workers, and a batch size of 1,000. These numbers describe that run, not a throughput guarantee for other systems or webhook receivers.

The benchmark notes also contain a rough projection for one event sent to one million subscriber endpoints. That scale has **not** been measured by the reported benchmark. See the architecture guide for the assumptions and distinctions between measured results and estimates.

## Current boundaries

Mandarin keeps the delivery path simple. Before using it for critical production traffic, be aware of these current limitations:

- Each delivery gets one attempt. Failures are recorded; there is no retry schedule, backoff, or dead-letter queue.
- There is no automatic recovery loop for messages left pending if a worker stops during delivery.
- Publishing the same request again creates a new event; publisher idempotency keys are not supported.
- Outbound webhooks do not include cryptographic signatures, and the API does not provide built-in client authentication.
- Event and delivery records have no automatic expiration, so Redis storage grows as records accumulate.
- The implementation targets a standalone Redis instance; Redis Cluster is not supported as-is.

The limitations and trade-offs guide explains the consequences of these boundaries and the reasoning behind the main design choices.

## License

Mandarin is released under the [MIT License](LICENSE).
