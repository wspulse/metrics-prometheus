# Changelog

## [Unreleased]

### Changed

- **BREAKING:** Upgrade `wspulse/server` dependency from v0.4.0 to v0.5.0
- `ConnectionClosed` now accepts a `DisconnectReason` parameter (matching server v0.5.0 interface)
- `connections_closed_total` and `connection_duration_seconds` now include a `reason` label (`normal`, `kick`, `grace_expired`, `server_close`, `duplicate`)

### Added

- Initial release: `Collector` implementing `wspulse.MetricsCollector` with Prometheus backend
- `NewCollector(opts ...Option)` constructor
- `Handler()` for serving `/metrics` endpoint
- Options: `WithRegisterer`, `WithGatherer`, `WithNamespace`, `WithRoomLabel`
