# wspulse/metrics-prometheus

[![CI](https://github.com/wspulse/metrics-prometheus/actions/workflows/ci.yml/badge.svg)](https://github.com/wspulse/metrics-prometheus/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/wspulse/metrics-prometheus.svg)](https://pkg.go.dev/github.com/wspulse/metrics-prometheus)
[![Go](https://img.shields.io/badge/Go-1.26-blue.svg?logo=go)](https://go.dev)
[![Prometheus](https://img.shields.io/badge/Prometheus-v1.22.0-blue.svg?logo=prometheus)](https://github.com/prometheus/client_golang)
[![wspulse/hub](https://img.shields.io/badge/wspulse%2Fhub-v0.8.1-blue.svg)](https://github.com/wspulse/hub)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Prometheus adapter for [wspulse/hub](https://github.com/wspulse/hub)'s `MetricsCollector` interface.

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

    wspulse "github.com/wspulse/hub"
    wsprom "github.com/wspulse/metrics-prometheus"
)

collector := wsprom.NewCollector()

hub := wspulse.NewHub(connect,
    wspulse.WithMetrics(collector),
)

http.Handle("/ws", hub)
http.Handle("/metrics", collector.Handler())
http.ListenAndServe(":8080", nil)
```

Custom registry with per-room metrics (opt-in — only when room count is bounded):

```go
reg := prometheus.NewRegistry()
collector := wsprom.NewCollector(
    wsprom.WithRegisterer(reg),
    wsprom.WithGatherer(reg),
    wsprom.WithNamespace("myapp"),
    wsprom.WithRoomLabel(true),
)
```

---

## Documentation

- [Usage Guide](doc/usage.md) — configuration options and metrics reference
- [Metrics Integration Guide](https://github.com/wspulse/docs/blob/main/guides/metrics.md)

## Related Modules

- [wspulse/hub](https://github.com/wspulse/hub) — WebSocket server library
- [wspulse/metrics-otel](https://github.com/wspulse/metrics-otel) — OpenTelemetry adapter
- [wspulse/docs](https://github.com/wspulse/docs) — User-facing documentation
