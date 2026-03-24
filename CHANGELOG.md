# Changelog

## [Unreleased]

### Added

- Initial release: `Collector` implementing `wspulse.MetricsCollector` with Prometheus backend
- `NewCollector(opts ...Option)` constructor
- `Handler()` for serving `/metrics` endpoint
- Options: `WithRegisterer`, `WithGatherer`, `WithNamespace`, `WithRoomLabel`
