# Changelog

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
- `connections_closed_total` and `connection_duration_seconds` include a `reason` label (`normal`, `kick`, `grace_expired`, `server_close`, `duplicate`)
- `resume_attempts_total` counter tracking session resumptions
- Explicit histogram bucket boundaries for `connection_duration_seconds` (1s-24h), `broadcast_fanout` (1-1000), and `send_buffer_utilization` (0.1-1.0)
- `doc/usage.md` with metrics table, labels reference, histogram boundaries, and configuration examples
- Benchmark suite for all hot-path methods
