# Changelog

## [Unreleased]

### Changed

- Upgrade `wspulse/server` dependency from v0.4.0 to v0.5.0
- `ConnectionClosed` now accepts a `DisconnectReason` parameter (matching server v0.5.0 interface)
- `connections_closed_total` and `connection_duration_seconds` now include a `reason` label (`normal`, `kick`, `grace_expired`, `server_close`, `duplicate`)
- `send_buffer_utilization` instrument changed from Gauge to Histogram for accurate multi-connection distribution

### Added

- Initial release: `Collector` implementing `wspulse.MetricsCollector` with Prometheus backend
- `NewCollector(opts ...Option)` constructor
- `Handler()` for serving `/metrics` endpoint
- Options: `WithRegisterer`, `WithGatherer`, `WithNamespace`, `WithRoomLabel`
- Options: `WithConnectionDurationBuckets`, `WithBroadcastFanoutBuckets`, `WithSendBufferUtilizationBuckets` for custom histogram bucket configuration
- `resume_attempts_total` counter includes a `success` label indicating whether the resume succeeded
- Explicit histogram bucket boundaries for `connection_duration_seconds` (1s-24h), `broadcast_fanout` (1-1000), and `send_buffer_utilization` (0.1-1.0)
- `doc/usage.md` with metrics table, labels reference, histogram boundaries, and configuration examples
- Benchmark suite for all hot-path methods
