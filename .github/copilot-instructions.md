# Copilot Instructions — wspulse/metrics-prometheus

## Project Overview

wspulse/metrics-prometheus is a **Prometheus adapter** for wspulse/server's `MetricsCollector` interface. It translates server lifecycle events into Prometheus counters, gauges, and histograms. Module path: `github.com/wspulse/metrics-prometheus`. Package name: `prometheus`.

## Architecture

- **`collector.go`** — `Collector` struct implementing `wspulse.MetricsCollector`. Registers all Prometheus metrics on construction. Each interface method increments/observes the corresponding metric.
- **`options.go`** — `Option` functional options: `WithRegisterer`, `WithGatherer`, `WithNamespace`, `WithRoomLabel`.
- **`handler.go`** — `Handler()` method returning `http.Handler` for `/metrics` endpoint.
- **`collector_test.go`** — Unit tests verifying metric values after hook calls.

## Dependencies

- `github.com/wspulse/server` — source of `MetricsCollector` interface
- `github.com/prometheus/client_golang` — Prometheus client library

## Development Workflow

```bash
make fmt        # format source files
make check      # fmt + lint + test (pre-commit gate)
make test       # unit tests with race detector
make test-cover # tests with coverage report
make bench      # benchmarks
make tidy       # go mod tidy
```

## Conventions

- **Go style**: same as wspulse/server — `gofmt`/`goimports`, GoDoc on all public symbols.
- **Naming**: interface names use full words. Package name is `prometheus`.
- **Metric naming**: `wspulse_` prefix, snake_case, follows [Prometheus naming conventions](https://prometheus.io/docs/practices/naming/).
- **Error format**: `fmt.Errorf("wspulse/metrics-prometheus: <context>: %w", err)`.
- **Markdown**: no emojis in documentation files.
- **Git**: commit messages follow [commit-message-instructions.md](instructions/commit-message-instructions.md). Branch strategy: `feat/`, `fix/`, `chore/`. Never push directly to `main`.

## Critical Rules

1. **Read before write** — read the target file before editing.
2. **STOP — test first, fix second** — when a bug is discovered or reported, do NOT touch production code until a failing test exists. Follow this exact sequence: (1) write a failing test, (2) confirm it fails, (3) fix the code, (4) confirm it passes, (5) run `make check`.
3. **`make check` gates every commit** — fmt + lint + test must pass.
4. **Minimal changes** — one concern per edit.
5. **No breaking changes without version bump** — exported symbols are a public contract.
6. **Thread safety** — all `Collector` methods are called concurrently from server goroutines. Prometheus client handles this internally, but verify any custom state is properly synchronized.
7. **Accuracy** — verify metric names, types, and label sets against the plan in the workspace `doc/local/plan/metrics-prometheus.md`.
8. **Documentation sync** — when changing public API or options, update `docs/reference/` and `docs/guides/metrics.md` in the docs repo.
