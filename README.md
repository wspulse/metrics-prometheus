# wspulse/metrics-prometheus

[![CI](https://github.com/wspulse/metrics-prometheus/actions/workflows/ci.yml/badge.svg)](https://github.com/wspulse/metrics-prometheus/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/wspulse/metrics-prometheus.svg)](https://pkg.go.dev/github.com/wspulse/metrics-prometheus)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Prometheus adapter for [wspulse/server](https://github.com/wspulse/server)'s `MetricsCollector` interface.

---

## Install

```bash
go get github.com/wspulse/metrics-prometheus
```

---

## Quick Start

```go
import (
    "net/http"

    "github.com/wspulse/server"
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

Custom registry and namespace:

```go
reg := prometheus.NewRegistry()
collector := wsprom.NewCollector(
    wsprom.WithRegisterer(reg),
    wsprom.WithGatherer(reg),
    wsprom.WithNamespace("myapp"),
    wsprom.WithRoomLabel(false), // disable room_id label for high-cardinality environments
)
```

---

## Documentation

- [Metrics Integration Guide](https://github.com/wspulse/docs/blob/main/guides/metrics.md)

## Related Modules

- [wspulse/server](https://github.com/wspulse/server) — WebSocket server library
- [wspulse/metrics-otel](https://github.com/wspulse/metrics-otel) — OpenTelemetry adapter
- [wspulse/docs](https://github.com/wspulse/docs) — User-facing documentation
