package prometheus_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	wsprom "github.com/wspulse/metrics-prometheus"
	wspulse "github.com/wspulse/server"
)

// scrapeBody performs a GET /metrics against the collector's handler and
// returns the response body as a string.
func scrapeBody(t *testing.T, c *wsprom.Collector) string {
	t.Helper()
	rec := httptest.NewRecorder()
	c.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusOK, rec.Code, "scrape /metrics")
	return rec.Body.String()
}

// ── Connection lifecycle (component) ─────────────────────────────────────────

func TestConnectionLifecycle_OpenAndActive(t *testing.T) {
	t.Parallel()
	c, _ := newTestCollector(t)

	c.ConnectionOpened("test-room", "conn1")
	c.ConnectionOpened("test-room", "conn2")
	c.RoomCreated("test-room")

	body := scrapeBody(t, c)

	assert.Contains(t, body, `wspulse_connections_opened_total{room_id="test-room"} 2`,
		"expected 2 connections opened")
	assert.Contains(t, body, `wspulse_connections_active{room_id="test-room"} 2`,
		"expected 2 active connections")
	assert.Contains(t, body, `wspulse_rooms_active 1`,
		"expected 1 active room")
}

func TestConnectionLifecycle_CloseReducesActive(t *testing.T) {
	t.Parallel()
	c, _ := newTestCollector(t)

	c.RoomCreated("test-room")
	c.ConnectionOpened("test-room", "conn1")
	c.ConnectionOpened("test-room", "conn2")

	c.ConnectionClosed("test-room", "conn1", 5*time.Second, wspulse.DisconnectNormal)
	c.ConnectionClosed("test-room", "conn2", 10*time.Second, wspulse.DisconnectNormal)
	c.RoomDestroyed("test-room")

	body := scrapeBody(t, c)

	// Closed count with reason=normal. Label order may vary.
	closedMatch := assert.Condition(t, func() bool {
		return contains(body, `wspulse_connections_closed_total{reason="normal",room_id="test-room"} 2`) ||
			contains(body, `wspulse_connections_closed_total{room_id="test-room",reason="normal"} 2`)
	}, "expected 2 connections closed (reason=normal) in scrape output:\n%s", body)
	_ = closedMatch

	assert.Contains(t, body, `wspulse_connections_active{room_id="test-room"} 0`,
		"expected 0 active connections after close")
	assert.Contains(t, body, `wspulse_rooms_active 0`,
		"expected 0 active rooms after close")
}

func TestConnectionLifecycle_DurationRecorded(t *testing.T) {
	t.Parallel()
	c, reg := newTestCollector(t)

	c.ConnectionClosed("test-room", "conn1", 5*time.Second, wspulse.DisconnectNormal)
	c.ConnectionClosed("test-room", "conn2", 15*time.Second, wspulse.DisconnectNormal)

	assert.Equal(t, uint64(2),
		histogramSampleCount(t, reg, "wspulse_connection_duration_seconds"),
		"expected 2 duration observations")
	assert.Equal(t, float64(20),
		histogramSampleSum(t, reg, "wspulse_connection_duration_seconds"),
		"expected sum of durations = 5 + 15 = 20 seconds")
}

func TestConnectionLifecycle_MultipleRooms(t *testing.T) {
	t.Parallel()
	c, reg := newTestCollector(t)

	c.RoomCreated("room-a")
	c.RoomCreated("room-b")
	c.ConnectionOpened("room-a", "conn1")
	c.ConnectionOpened("room-b", "conn2")

	assert.Equal(t, float64(1),
		requireMetricWithLabel(t, reg, "wspulse_connections_opened_total", "room_id", "room-a"),
		"room-a opened")
	assert.Equal(t, float64(1),
		requireMetricWithLabel(t, reg, "wspulse_connections_opened_total", "room_id", "room-b"),
		"room-b opened")
	assert.Equal(t, float64(2),
		metricValue(t, reg, "wspulse_rooms_active"),
		"2 rooms active")
}

func TestConnectionLifecycle_DisconnectReasons(t *testing.T) {
	t.Parallel()
	c, reg := newTestCollector(t)

	c.ConnectionClosed("test-room", "conn1", time.Second, wspulse.DisconnectNormal)
	c.ConnectionClosed("test-room", "conn2", time.Second, wspulse.DisconnectKick)

	assert.Equal(t, float64(1),
		requireMetricWithLabel(t, reg, "wspulse_connections_closed_total", "reason", "normal"),
		"1 normal disconnect")
	assert.Equal(t, float64(1),
		requireMetricWithLabel(t, reg, "wspulse_connections_closed_total", "reason", "kick"),
		"1 kick disconnect")
}

// contains is a helper to avoid importing strings in the test file.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
