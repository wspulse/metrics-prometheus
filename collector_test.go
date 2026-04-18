package prometheus_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	wspulse "github.com/wspulse/hub"
	wsprom "github.com/wspulse/metrics-prometheus"
)

// newTestCollector creates a Collector backed by an isolated registry with
// room labels enabled. Pass additional options to override defaults.
func newTestCollector(t *testing.T, opts ...wsprom.Option) (*wsprom.Collector, *prometheus.Registry) {
	t.Helper()
	reg := prometheus.NewRegistry()
	allOpts := append([]wsprom.Option{
		wsprom.WithRegisterer(reg),
		wsprom.WithGatherer(reg),
		wsprom.WithRoomLabel(true),
	}, opts...)
	return wsprom.NewCollector(allOpts...), reg
}

// metricValue gathers the named metric from the registry and returns its value.
// For metrics with multiple label sets, this helper sums the values across all series.
func metricValue(t *testing.T, reg *prometheus.Registry, name string) float64 {
	t.Helper()
	mfs, err := reg.Gather()
	require.NoError(t, err, "gather")
	for _, mf := range mfs {
		if mf.GetName() == name {
			ms := mf.GetMetric()
			if len(ms) == 0 {
				return 0
			}
			// Sum all series for simplicity.
			var total float64
			for _, m := range ms {
				if m.GetCounter() != nil {
					total += m.GetCounter().GetValue()
				} else if m.GetGauge() != nil {
					total += m.GetGauge().GetValue()
				}
			}
			return total
		}
	}
	require.Failf(t, "metric not found", "metric %q not found", name)
	return 0
}

// histogramSampleCount returns the total sample count across all series of a histogram metric.
func histogramSampleCount(t *testing.T, reg *prometheus.Registry, name string) uint64 {
	t.Helper()
	mfs, err := reg.Gather()
	require.NoError(t, err, "gather")
	for _, mf := range mfs {
		if mf.GetName() == name {
			var total uint64
			for _, m := range mf.GetMetric() {
				if m.GetHistogram() != nil {
					total += m.GetHistogram().GetSampleCount()
				}
			}
			return total
		}
	}
	require.Failf(t, "histogram not found", "histogram %q not found", name)
	return 0
}

// histogramSampleSum returns the total sample sum across all series of a histogram metric.
func histogramSampleSum(t *testing.T, reg *prometheus.Registry, name string) float64 {
	t.Helper()
	mfs, err := reg.Gather()
	require.NoError(t, err, "gather")
	for _, mf := range mfs {
		if mf.GetName() == name {
			var total float64
			for _, m := range mf.GetMetric() {
				if m.GetHistogram() != nil {
					total += m.GetHistogram().GetSampleSum()
				}
			}
			return total
		}
	}
	require.Failf(t, "histogram not found", "histogram %q not found", name)
	return 0
}

// metricValueWithLabel gathers the named metric with a specific label value.
// Returns the value and whether a matching series was found.
func metricValueWithLabel(t *testing.T, reg *prometheus.Registry, name, labelName, labelValue string) (float64, bool) {
	t.Helper()
	mfs, err := reg.Gather()
	require.NoError(t, err, "gather")
	for _, mf := range mfs {
		if mf.GetName() == name {
			for _, m := range mf.GetMetric() {
				for _, lp := range m.GetLabel() {
					if lp.GetName() == labelName && lp.GetValue() == labelValue {
						if m.GetCounter() != nil {
							return m.GetCounter().GetValue(), true
						}
						if m.GetGauge() != nil {
							return m.GetGauge().GetValue(), true
						}
					}
				}
			}
		}
	}
	return 0, false
}

// requireMetricWithLabel is a helper that calls metricValueWithLabel and
// fails the test if the metric/label combination is not found.
func requireMetricWithLabel(t *testing.T, reg *prometheus.Registry, name, labelName, labelValue string) float64 {
	t.Helper()
	v, found := metricValueWithLabel(t, reg, name, labelName, labelValue)
	require.Truef(t, found, "metric %q with %s=%q not found", name, labelName, labelValue)
	return v
}

// hasMetricWithName checks if a metric with the given name exists.
func hasMetricWithName(t *testing.T, reg *prometheus.Registry, name string) bool {
	t.Helper()
	mfs, err := reg.Gather()
	require.NoError(t, err, "gather")
	for _, mf := range mfs {
		if mf.GetName() == name {
			return true
		}
	}
	return false
}

// hasLabel checks if any series of the named metric contains a specific label.
func hasLabel(t *testing.T, reg *prometheus.Registry, metricName, labelName string) bool {
	t.Helper()
	mfs, err := reg.Gather()
	require.NoError(t, err, "gather")
	for _, mf := range mfs {
		if mf.GetName() == metricName {
			for _, m := range mf.GetMetric() {
				for _, lp := range m.GetLabel() {
					if lp.GetName() == labelName {
						return true
					}
				}
			}
		}
	}
	return false
}

// ── Interface compliance ─────────────────────────────────────────────────────

func TestCollector_ImplementsMetricsCollector(t *testing.T) {
	t.Parallel()
	var _ wspulse.MetricsCollector = (*wsprom.Collector)(nil)
}

// ── Option validation ────────────────────────────────────────────────────────

func TestWithRegisterer_NilPanics(t *testing.T) {
	t.Parallel()
	require.Panics(t, func() {
		_ = wsprom.WithRegisterer(nil)
	})
}

func TestWithGatherer_NilPanics(t *testing.T) {
	t.Parallel()
	require.Panics(t, func() {
		_ = wsprom.WithGatherer(nil)
	})
}

func TestWithNamespace_InvalidPanics(t *testing.T) {
	t.Parallel()
	cases := []string{"my-app", "123abc", "has space", "special!char"}
	for _, ns := range cases {
		assert.Panicsf(t, func() {
			_ = wsprom.WithNamespace(ns)
		}, "expected panic for namespace %q", ns)
	}
}

func TestWithNamespace_ValidDoesNotPanic(t *testing.T) {
	t.Parallel()
	valid := []string{"", "myapp", "my_app", "_private", "App123"}
	for _, ns := range valid {
		_ = wsprom.WithNamespace(ns) // must not panic
	}
}

func TestWithConnectionDurationBuckets_EmptyPanics(t *testing.T) {
	t.Parallel()
	require.Panics(t, func() {
		_ = wsprom.WithConnectionDurationBuckets(nil)
	})
}

func TestWithBroadcastFanoutBuckets_EmptyPanics(t *testing.T) {
	t.Parallel()
	require.Panics(t, func() {
		_ = wsprom.WithBroadcastFanoutBuckets(nil)
	})
}

func TestWithSendBufferUtilizationBuckets_EmptyPanics(t *testing.T) {
	t.Parallel()
	require.Panics(t, func() {
		_ = wsprom.WithSendBufferUtilizationBuckets(nil)
	})
}

func TestNewCollector_DuplicateRegistrationPanics(t *testing.T) {
	t.Parallel()
	reg := prometheus.NewRegistry()
	_ = wsprom.NewCollector(wsprom.WithRegisterer(reg), wsprom.WithGatherer(reg))

	require.Panics(t, func() {
		_ = wsprom.NewCollector(wsprom.WithRegisterer(reg), wsprom.WithGatherer(reg))
	})
}

// ── Connection lifecycle ─────────────────────────────────────────────────────

func TestConnectionOpened(t *testing.T) {
	t.Parallel()
	c, reg := newTestCollector(t)

	c.ConnectionOpened("room1", "conn1")
	c.ConnectionOpened("room1", "conn2")
	c.ConnectionOpened("room2", "conn3")

	assert.Equal(t, float64(2), requireMetricWithLabel(t, reg, "wspulse_connections_opened_total", "room_id", "room1"), "room1 opened")
	assert.Equal(t, float64(1), requireMetricWithLabel(t, reg, "wspulse_connections_opened_total", "room_id", "room2"), "room2 opened")
	assert.Equal(t, float64(2), requireMetricWithLabel(t, reg, "wspulse_connections_active", "room_id", "room1"), "room1 active")
}

func TestConnectionClosed(t *testing.T) {
	t.Parallel()
	c, reg := newTestCollector(t)

	c.ConnectionOpened("room1", "conn1")
	c.ConnectionClosed("room1", "conn1", 5*time.Second, wspulse.DisconnectNormal)

	assert.Equal(t, float64(0), requireMetricWithLabel(t, reg, "wspulse_connections_active", "room_id", "room1"), "room1 active")
	assert.Equal(t, float64(1), requireMetricWithLabel(t, reg, "wspulse_connections_closed_total", "reason", "normal"), "room1 closed (reason=normal)")
	assert.Equal(t, float64(1), requireMetricWithLabel(t, reg, "wspulse_connections_closed_total", "room_id", "room1"), "room1 closed (room_id=room1)")
	// Histogram observation: verify metric exists and carries both labels.
	assert.True(t, hasMetricWithName(t, reg, "wspulse_connection_duration_seconds"), "connection_duration_seconds metric missing")
	assert.True(t, hasLabel(t, reg, "wspulse_connection_duration_seconds", "reason"), "connection_duration_seconds missing reason label")
}

func TestConnectionClosed_AllReasons(t *testing.T) {
	t.Parallel()

	reasons := []struct {
		reason wspulse.DisconnectReason
		want   string
	}{
		{wspulse.DisconnectNormal, "normal"},
		{wspulse.DisconnectKick, "kick"},
		{wspulse.DisconnectGraceExpired, "grace_expired"},
		{wspulse.DisconnectHubClose, "hub_close"},
		{wspulse.DisconnectDuplicate, "duplicate"},
	}

	for _, tt := range reasons {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			c, reg := newTestCollector(t)

			c.ConnectionClosed("room1", "conn1", 2*time.Second, tt.reason)

			assert.Equal(t, float64(1),
				requireMetricWithLabel(t, reg, "wspulse_connections_closed_total", "reason", tt.want),
				"reason=%s", tt.want)
		})
	}
}

func TestConnectionClosed_WithRoomLabelFalse(t *testing.T) {
	t.Parallel()
	c, reg := newTestCollector(t, wsprom.WithRoomLabel(false))

	c.ConnectionOpened("room1", "conn1")
	c.ConnectionClosed("room1", "conn1", 5*time.Second, wspulse.DisconnectNormal)

	assert.Equal(t, float64(0), metricValue(t, reg, "wspulse_connections_active"), "active")
	assert.Equal(t, float64(1), metricValue(t, reg, "wspulse_connections_closed_total"), "closed")
	assert.False(t, hasLabel(t, reg, "wspulse_connections_closed_total", "room_id"), "room_id label should not exist when WithRoomLabel(false)")
	assert.True(t, hasLabel(t, reg, "wspulse_connections_closed_total", "reason"), "reason label should still exist when WithRoomLabel(false)")
}

func TestResumeAttempt(t *testing.T) {
	t.Parallel()
	c, reg := newTestCollector(t)

	c.ResumeAttempt("room1", "conn1")

	assert.Equal(t, float64(1), requireMetricWithLabel(t, reg, "wspulse_resume_attempts_total", "room_id", "room1"), "resume attempts")
	assert.False(t, hasLabel(t, reg, "wspulse_resume_attempts_total", "success"), "success label should not exist on resume_attempts_total")
}

func TestResumeAttempt_WithRoomLabelFalse(t *testing.T) {
	t.Parallel()
	c, reg := newTestCollector(t, wsprom.WithRoomLabel(false))

	c.ResumeAttempt("room1", "conn1")

	assert.Equal(t, float64(1), metricValue(t, reg, "wspulse_resume_attempts_total"), "resume attempts")
	assert.False(t, hasLabel(t, reg, "wspulse_resume_attempts_total", "room_id"), "room_id label should not exist when WithRoomLabel(false)")
	assert.False(t, hasLabel(t, reg, "wspulse_resume_attempts_total", "success"), "success label should not exist on resume_attempts_total")
}

// ── Room lifecycle ───────────────────────────────────────────────────────────

func TestRoomCreatedDestroyed(t *testing.T) {
	t.Parallel()
	c, reg := newTestCollector(t)

	c.RoomCreated("room1")
	c.RoomCreated("room2")

	assert.Equal(t, float64(2), metricValue(t, reg, "wspulse_rooms_active"), "rooms active")
	assert.Equal(t, float64(2), metricValue(t, reg, "wspulse_rooms_created_total"), "rooms created")

	c.RoomDestroyed("room1")

	assert.Equal(t, float64(1), metricValue(t, reg, "wspulse_rooms_active"), "rooms active after destroy")
	assert.Equal(t, float64(1), metricValue(t, reg, "wspulse_rooms_destroyed_total"), "rooms destroyed")
}

// ── Throughput ─────────────────────────────────────────────────────────────────────

func TestMessageReceived(t *testing.T) {
	t.Parallel()
	c, reg := newTestCollector(t)

	c.MessageReceived("room1", 100)
	c.MessageReceived("room1", 200)

	assert.Equal(t, float64(2), requireMetricWithLabel(t, reg, "wspulse_messages_received_total", "room_id", "room1"), "room1 received")
	assert.Equal(t, float64(300), requireMetricWithLabel(t, reg, "wspulse_messages_received_bytes_total", "room_id", "room1"), "room1 received bytes")
}

func TestMessageBroadcast(t *testing.T) {
	t.Parallel()
	c, reg := newTestCollector(t)

	c.MessageBroadcast("room1", 50, 10)

	assert.Equal(t, float64(1), requireMetricWithLabel(t, reg, "wspulse_messages_broadcast_total", "room_id", "room1"), "room1 broadcast")
	assert.True(t, hasMetricWithName(t, reg, "wspulse_broadcast_fanout"), "broadcast_fanout metric missing")
}

func TestMessageBroadcast_FanoutHistogramValue(t *testing.T) {
	t.Parallel()
	c, reg := newTestCollector(t)

	c.MessageBroadcast("room1", 50, 10)
	c.MessageBroadcast("room1", 50, 20)

	assert.Equal(t, uint64(2), histogramSampleCount(t, reg, "wspulse_broadcast_fanout"), "fanout sample count")
	assert.Equal(t, float64(30), histogramSampleSum(t, reg, "wspulse_broadcast_fanout"), "fanout sample sum")
}

func TestMessageSent(t *testing.T) {
	t.Parallel()
	c, reg := newTestCollector(t)

	c.MessageSent("room1", "conn1", 100)

	assert.Equal(t, float64(1), requireMetricWithLabel(t, reg, "wspulse_messages_sent_total", "room_id", "room1"), "room1 sent")
}

func TestFrameDropped(t *testing.T) {
	t.Parallel()
	c, reg := newTestCollector(t)

	c.FrameDropped("room1", "conn1")
	c.FrameDropped("room1", "conn1")

	assert.Equal(t, float64(2), requireMetricWithLabel(t, reg, "wspulse_frames_dropped_total", "room_id", "room1"), "room1 dropped")
}

func TestSendBufferUtilization(t *testing.T) {
	t.Parallel()
	c, reg := newTestCollector(t)

	c.SendBufferUtilization("room1", "conn1", 128, 256)
	c.SendBufferUtilization("room1", "conn2", 64, 256)

	// Histogram: two observations, sum = 0.5 + 0.25 = 0.75
	assert.Equal(t, uint64(2), histogramSampleCount(t, reg, "wspulse_send_buffer_utilization"), "buffer utilization sample count")
	assert.Equal(t, 0.75, histogramSampleSum(t, reg, "wspulse_send_buffer_utilization"), "buffer utilization sample sum")
}

func TestSendBufferUtilization_ZeroCapacity(t *testing.T) {
	t.Parallel()
	c, reg := newTestCollector(t)

	c.SendBufferUtilization("room1", "conn1", 0, 0)

	assert.Equal(t, uint64(1), histogramSampleCount(t, reg, "wspulse_send_buffer_utilization"), "buffer utilization sample count")
	assert.Equal(t, float64(0), histogramSampleSum(t, reg, "wspulse_send_buffer_utilization"), "buffer utilization sample sum")
}

// ── Heartbeat ────────────────────────────────────────────────────────────────

func TestHeartbeatFailed(t *testing.T) {
	t.Parallel()
	c, reg := newTestCollector(t)

	c.HeartbeatFailed("room1", "conn1")

	assert.Equal(t, float64(1), requireMetricWithLabel(t, reg, "wspulse_heartbeat_failures_total", "room_id", "room1"), "room1 heartbeat failures")
}

// ── WithRoomLabel(false) ─────────────────────────────────────────────────────

func TestWithRoomLabel_False(t *testing.T) {
	t.Parallel()
	c, reg := newTestCollector(t, wsprom.WithRoomLabel(false))

	c.ConnectionOpened("room1", "conn1")
	c.MessageReceived("room1", 100)
	c.FrameDropped("room1", "conn1")
	c.HeartbeatFailed("room1", "conn1")

	// Verify metrics exist but have no room_id label.
	assert.Equal(t, float64(1), metricValue(t, reg, "wspulse_connections_opened_total"), "opened (no room label)")
	assert.False(t, hasLabel(t, reg, "wspulse_connections_opened_total", "room_id"), "room_id label should not exist when WithRoomLabel(false)")
	assert.False(t, hasLabel(t, reg, "wspulse_messages_received_total", "room_id"), "room_id label should not exist on messages_received_total")
}

func TestWithRoomLabel_DefaultIsFalse(t *testing.T) {
	t.Parallel()
	reg := prometheus.NewRegistry()
	c := wsprom.NewCollector(wsprom.WithRegisterer(reg), wsprom.WithGatherer(reg))

	c.ConnectionOpened("room1", "conn1")

	assert.False(t, hasLabel(t, reg, "wspulse_connections_opened_total", "room_id"), "default roomLabel should be false; room_id label should not exist")
}

// ── WithNamespace ────────────────────────────────────────────────────────────

func TestWithNamespace(t *testing.T) {
	t.Parallel()
	c, reg := newTestCollector(t, wsprom.WithNamespace("myapp"))

	c.RoomCreated("room1")
	c.ConnectionOpened("room1", "conn1")

	metrics, err := reg.Gather()
	require.NoError(t, err)
	var names []string
	for _, m := range metrics {
		names = append(names, m.GetName())
	}
	assert.Contains(t, names, "myapp_wspulse_rooms_created_total", "gathered: %v", names)
	assert.Contains(t, names, "myapp_wspulse_connections_opened_total", "gathered: %v", names)
}

// ── Custom bucket options ────────────────────────────────────────────────────

func TestWithConnectionDurationBuckets(t *testing.T) {
	t.Parallel()
	c, reg := newTestCollector(t, wsprom.WithConnectionDurationBuckets([]float64{1, 10, 100}))

	c.ConnectionClosed("room1", "conn1", 5*time.Second, wspulse.DisconnectNormal)

	assert.Equal(t, uint64(1), histogramSampleCount(t, reg, "wspulse_connection_duration_seconds"), "sample count")
}

func TestWithBroadcastFanoutBuckets(t *testing.T) {
	t.Parallel()
	c, reg := newTestCollector(t, wsprom.WithBroadcastFanoutBuckets([]float64{5, 50}))

	c.MessageBroadcast("room1", 50, 10)

	assert.Equal(t, uint64(1), histogramSampleCount(t, reg, "wspulse_broadcast_fanout"), "sample count")
}

func TestWithSendBufferUtilizationBuckets(t *testing.T) {
	t.Parallel()
	c, reg := newTestCollector(t, wsprom.WithSendBufferUtilizationBuckets([]float64{0.5, 1.0}))

	c.SendBufferUtilization("room1", "conn1", 128, 256)

	assert.Equal(t, uint64(1), histogramSampleCount(t, reg, "wspulse_send_buffer_utilization"), "sample count")
}

// ── Handler ──────────────────────────────────────────────────────────────────

func TestHandler_ReturnsNonNil(t *testing.T) {
	t.Parallel()
	c, _ := newTestCollector(t)
	assert.NotNil(t, c.Handler(), "Handler() returned nil")
}

func TestHandler_ServesPrometheusFormat(t *testing.T) {
	t.Parallel()
	c, _ := newTestCollector(t)

	c.RoomCreated("room1")
	c.ConnectionOpened("room1", "conn1")

	rec := httptest.NewRecorder()
	c.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	require.Equal(t, http.StatusOK, rec.Code, "status")
	body := rec.Body.String()
	assert.Contains(t, body, "wspulse_rooms_created_total 1", "expected rooms_created_total in scrape output")
	assert.Contains(t, body, "# HELP wspulse_rooms_created_total", "expected HELP line in scrape output")
	assert.Contains(t, body, "# TYPE wspulse_rooms_created_total counter", "expected TYPE line in scrape output")
}

// ── Scrape output ────────────────────────────────────────────────────────────

func TestScrapeOutput_ContainsExpectedMetrics(t *testing.T) {
	t.Parallel()
	c, reg := newTestCollector(t)

	c.ConnectionOpened("room1", "conn1")
	c.MessageReceived("room1", 42)
	c.RoomCreated("room1")

	mfs, gatherErr := reg.Gather()
	require.NoError(t, gatherErr, "gather")
	expectedNames := []string{
		"wspulse_connections_opened_total",
		"wspulse_connections_active",
		"wspulse_messages_received_total",
		"wspulse_messages_received_bytes_total",
		"wspulse_rooms_active",
		"wspulse_rooms_created_total",
	}
	gatheredNames := make(map[string]bool, len(mfs))
	for _, mf := range mfs {
		gatheredNames[mf.GetName()] = true
	}
	for _, name := range expectedNames {
		assert.Truef(t, gatheredNames[name], "missing expected metric %q in scrape output", name)
	}
}
