# Usage Guide

## Installation

```bash
go get github.com/wspulse/metrics-prometheus
```

## Quick Start

```go
import (
    "net/http"

    wspulse "github.com/wspulse/server"
    wsprom "github.com/wspulse/metrics-prometheus"
)

collector := wsprom.NewCollector()

srv := wspulse.NewServer(connect,
    wspulse.WithMetrics(collector),
)

http.Handle("/ws", srv)
http.Handle("/metrics", collector.Handler())
http.ListenAndServe(":8080", nil)
```

## Options

### WithRegisterer / WithGatherer

Use a custom Prometheus registry instead of the global default. This is useful
when running multiple collectors or when you want isolated metric namespaces.

```go
reg := prometheus.NewRegistry()
collector := wsprom.NewCollector(
    wsprom.WithRegisterer(reg),
    wsprom.WithGatherer(reg),
)
```

### WithNamespace

Add a prefix to all metric names. For example, `WithNamespace("myapp")` produces
`myapp_wspulse_connections_opened_total` instead of `wspulse_connections_opened_total`.

```go
collector := wsprom.NewCollector(
    wsprom.WithNamespace("myapp"),
)
```

The namespace must match `[a-zA-Z_][a-zA-Z0-9_]*`. Invalid values cause a panic
at construction time.

### WithRoomLabel

Controls whether `room_id` is included as a label on per-room metrics.

**Default: `false`** (no `room_id` label).

Enable this only when the number of distinct rooms is bounded and known. In
high-cardinality environments (one room per user, per livestream, etc.), enabling
this option causes excessive label combinations that may exhaust Prometheus memory.

```go
// Safe: fixed set of chat rooms
collector := wsprom.NewCollector(
    wsprom.WithRoomLabel(true),
)

// Default: aggregated metrics without room_id
collector := wsprom.NewCollector()
```

### WithConnectionDurationBuckets

Override the default histogram buckets for `connection_duration_seconds`.

Default buckets: `1, 5, 15, 30, 60, 300, 900, 3600, 7200, 14400, 43200, 86400`
(1 second to 24 hours).

```go
collector := wsprom.NewCollector(
    wsprom.WithConnectionDurationBuckets([]float64{1, 10, 60, 600, 3600}),
)
```

### WithBroadcastFanoutBuckets

Override the default histogram buckets for `broadcast_fanout`.

Default buckets: `1, 2, 5, 10, 25, 50, 100, 500, 1000`.

```go
collector := wsprom.NewCollector(
    wsprom.WithBroadcastFanoutBuckets([]float64{1, 5, 10, 50, 100}),
)
```

### WithSendBufferUtilizationBuckets

Override the default histogram buckets for `send_buffer_utilization`.

Default buckets: `0.1, 0.25, 0.5, 0.75, 0.9, 0.95, 0.99, 1.0`.

```go
collector := wsprom.NewCollector(
    wsprom.WithSendBufferUtilizationBuckets([]float64{0.25, 0.5, 0.75, 1.0}),
)
```

## Metrics Reference

### Connection Lifecycle

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `wspulse_connections_opened_total` | Counter | `room_id`* | Total connections opened |
| `wspulse_connections_closed_total` | Counter | `room_id`*, `reason` | Total connections closed |
| `wspulse_connections_active` | Gauge | `room_id`* | Currently active connections |
| `wspulse_connection_duration_seconds` | Histogram | `room_id`*, `reason` | Connection duration distribution |
| `wspulse_resume_attempts_total` | Counter | `room_id`*, `success` | Session resume attempts |

### Room

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `wspulse_rooms_active` | Gauge | -- | Currently active rooms |
| `wspulse_rooms_created_total` | Counter | -- | Total rooms created |
| `wspulse_rooms_destroyed_total` | Counter | -- | Total rooms destroyed |

### Throughput

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `wspulse_messages_received_total` | Counter | `room_id`* | Total messages received |
| `wspulse_messages_received_bytes_total` | Counter | `room_id`* | Total bytes received |
| `wspulse_messages_broadcast_total` | Counter | `room_id`* | Total messages broadcast |
| `wspulse_broadcast_fanout` | Histogram | `room_id`* | Recipients per broadcast |
| `wspulse_messages_sent_total` | Counter | `room_id`* | Total messages sent to connections |
| `wspulse_frames_dropped_total` | Counter | `room_id`* | Frames dropped due to backpressure |
| `wspulse_send_buffer_utilization` | Histogram | `room_id`* | Send buffer usage ratio (0.0--1.0) |

### Heartbeat

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `wspulse_pong_timeouts_total` | Counter | `room_id`* | Pong timeouts |

\* `room_id` label is only present when `WithRoomLabel(true)` is set.

## Full Example

```go
reg := prometheus.NewRegistry()
collector := wsprom.NewCollector(
    wsprom.WithRegisterer(reg),
    wsprom.WithGatherer(reg),
    wsprom.WithNamespace("myapp"),
    wsprom.WithRoomLabel(true),
    wsprom.WithConnectionDurationBuckets([]float64{1, 30, 300, 3600, 86400}),
)

srv := wspulse.NewServer(connect,
    wspulse.WithMetrics(collector),
)

mux := http.NewServeMux()
mux.Handle("/ws", srv)
mux.Handle("/metrics", collector.Handler())
http.ListenAndServe(":8080", mux)
```
