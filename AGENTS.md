# AGENTS.md — wspulse/metrics-prometheus

This file is the entry point for all AI coding agents (GitHub Copilot, Codex,
Cursor, Claude, etc.). Full working rules are in
`.github/copilot-instructions.md` — read it completely before
making any changes.

---

## Quick Reference

**Module**: `github.com/wspulse/metrics-prometheus` | **Package**: `prometheus`

**Key files**:

- `collector.go` — `Collector` struct implementing `wspulse.MetricsCollector`
- `options.go` — `Option` functional options: `WithRegisterer`, `WithGatherer`, `WithNamespace`, `WithRoomLabel`
- `handler.go` — `Handler()` method returning `http.Handler` for `/metrics` endpoint
- `collector_test.go` — Unit tests verifying metric values after hook calls

**Pre-commit gate**: `make check` (fmt → lint → test)

---

## Non-negotiable Rules

1. **Read before write** — read the target file before any edit.
2. **Thread safety** — all `Collector` methods are called concurrently from server goroutines. Verify any custom state is properly synchronized.
3. **Metric naming** — `wspulse_` prefix, snake_case, follows [Prometheus naming conventions](https://prometheus.io/docs/practices/naming/).
4. **No breaking changes without version bump.**
5. **No hardcoded secrets.**
6. **Minimal changes** — one concern per edit; no drive-by refactors.
7. **Documentation sync** — when changing public API or options, update `docs/reference/` and `docs/guides/metrics.md` in the docs repo.

---

## Session Protocol

> `doc/local/` is git-ignored. Never commit files under it.

- **Start of session**: read `doc/local/ai-learning.md` in full (create with header if missing) and check `doc/local/plan/` for any in-progress plan.
- **Feature work**: save plan to `doc/local/plan/<feature-name>.md` before starting.
- **End of session**: append at least one entry to `doc/local/ai-learning.md` — **mandatory even if no mistakes were made**. An empty file proves the session protocol was ignored.
  Format: `Date` / `Issue or Learning` / `Root Cause` / `Prevention Rule`.
