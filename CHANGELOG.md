# Changelog

## [Unreleased]

## [v0.4.0] - 2026-04-18

### Breaking changes

- Renamed `PongTimeout` method to `HeartbeatFailed` to match the updated
  `MetricsCollector` interface. Prometheus metric renamed from
  `wspulse_pong_timeouts_total` to `wspulse_heartbeat_failures_total`.

### Tests

- Added `TestConnectionClosed_AllReasons` covering all 5 `DisconnectReason` values (`normal`, `kick`, `grace_expired`, `hub_close`, `duplicate`) — matches the exhaustive table-driven pattern in `metrics-otel`

### Changed

- Upgraded `github.com/wspulse/hub` from v0.8.1 to v0.10.0.

---

## [v0.3.0] - 2026-04-04

### Changed

- Replaced integration tests with direct `Collector` component tests for deterministic metric assertions
- Removed network I/O from the test suite
- Adopted `testify` for test assertions

---

## [v0.2.0] - 2026-03-27

### Added

- Initial release: `Collector` implementing `wspulse.MetricsCollector` with Prometheus backend
- `NewCollector(opts ...Option)` constructor
- `Handler()` for serving `/metrics` endpoint
- Options: `WithRegisterer`, `WithGatherer`, `WithNamespace`, `WithRoomLabel`
- Options: `WithConnectionDurationBuckets`, `WithBroadcastFanoutBuckets`, `WithSendBufferUtilizationBuckets` for custom histogram bucket configuration
- `connections_closed_total` and `connection_duration_seconds` include a `reason` label (`normal`, `kick`, `grace_expired`, `hub_close`, `duplicate`)
- `resume_attempts_total` counter tracking session resumptions
- Explicit histogram bucket boundaries for `connection_duration_seconds` (1s-24h), `broadcast_fanout` (1-1000), and `send_buffer_utilization` (0.1-1.0)
- `doc/usage.md` with metrics table, labels reference, histogram boundaries, and configuration examples
- Benchmark suite for all hot-path methods
