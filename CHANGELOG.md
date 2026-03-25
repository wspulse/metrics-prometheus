# Changelog

## [Unreleased]

### Changed

- **BREAKING:** Upgrade `wspulse/server` dependency from v0.4.0 to v0.5.0
- **BREAKING:** `WithRoomLabel` now defaults to `false` -- callers must opt in with `WithRoomLabel(true)` when the number of distinct rooms is bounded
- **BREAKING:** `send_buffer_utilization_ratio` (Gauge) replaced by `send_buffer_utilization` (Histogram) -- captures distribution across connections instead of last-write-wins
- `ConnectionClosed` now accepts a `DisconnectReason` parameter (matching server v0.5.0 interface)
- `connections_closed_total` and `connection_duration_seconds` now include a `reason` label (`normal`, `kick`, `grace_expired`, `server_close`, `duplicate`)
- `connection_duration_seconds` default buckets extended to cover long-lived WebSocket sessions (up to 24 hours)
- `WithNamespace` now validates the namespace against Prometheus naming rules and panics on invalid input

### Added

- Initial release: `Collector` implementing `wspulse.MetricsCollector` with Prometheus backend
- `NewCollector(opts ...Option)` constructor
- `Handler()` for serving `/metrics` endpoint
- Options: `WithRegisterer`, `WithGatherer`, `WithNamespace`, `WithRoomLabel`
- Options: `WithConnectionDurationBuckets`, `WithBroadcastFanoutBuckets`, `WithSendBufferUtilizationBuckets` for custom histogram bucket configuration
- Benchmark suite for all hot-path methods
