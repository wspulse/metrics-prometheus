package prometheus_test

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	wsprom "github.com/wspulse/metrics-prometheus"
	wspulse "github.com/wspulse/server"
)

func newTestCollector(t *testing.T, opts ...wsprom.Option) (*wsprom.Collector, *prometheus.Registry) {
	t.Helper()
	reg := prometheus.NewRegistry()
	allOpts := append([]wsprom.Option{
		wsprom.WithRegisterer(reg),
		wsprom.WithGatherer(reg),
	}, opts...)
	return wsprom.NewCollector(allOpts...), reg
}

// metricValue gathers the named metric from the registry and returns its value.
// For metrics with multiple label sets, this helper sums the values across all series.
func metricValue(t *testing.T, reg *prometheus.Registry, name string) float64 {
	t.Helper()
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
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
	t.Fatalf("metric %q not found", name)
	return 0
}

// metricValueWithLabel gathers the named metric with a specific label value.
// Returns the value and whether a matching series was found.
func metricValueWithLabel(t *testing.T, reg *prometheus.Registry, name, labelName, labelValue string) (float64, bool) {
	t.Helper()
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
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
	if !found {
		t.Fatalf("metric %q with %s=%q not found", name, labelName, labelValue)
	}
	return v
}

// hasMetricWithName checks if a metric with the given name exists.
func hasMetricWithName(t *testing.T, reg *prometheus.Registry, name string) bool {
	t.Helper()
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
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
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
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
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for nil registerer")
		}
	}()
	_ = wsprom.WithRegisterer(nil)
}

func TestWithGatherer_NilPanics(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for nil gatherer")
		}
	}()
	_ = wsprom.WithGatherer(nil)
}

// ── Connection lifecycle ─────────────────────────────────────────────────────

func TestConnectionOpened(t *testing.T) {
	t.Parallel()
	c, reg := newTestCollector(t)

	c.ConnectionOpened("room1", "conn1")
	c.ConnectionOpened("room1", "conn2")
	c.ConnectionOpened("room2", "conn3")

	if got := requireMetricWithLabel(t, reg, "wspulse_connections_opened_total", "room_id", "room1"); got != 2 {
		t.Errorf("room1 opened: want 2, got %v", got)
	}
	if got := requireMetricWithLabel(t, reg, "wspulse_connections_opened_total", "room_id", "room2"); got != 1 {
		t.Errorf("room2 opened: want 1, got %v", got)
	}
	if got := requireMetricWithLabel(t, reg, "wspulse_connections_active", "room_id", "room1"); got != 2 {
		t.Errorf("room1 active: want 2, got %v", got)
	}
}

func TestConnectionClosed(t *testing.T) {
	t.Parallel()
	c, reg := newTestCollector(t)

	c.ConnectionOpened("room1", "conn1")
	c.ConnectionClosed("room1", "conn1", 5*time.Second, wspulse.DisconnectNormal)

	if got := requireMetricWithLabel(t, reg, "wspulse_connections_active", "room_id", "room1"); got != 0 {
		t.Errorf("room1 active: want 0, got %v", got)
	}
	if got := requireMetricWithLabel(t, reg, "wspulse_connections_closed_total", "reason", "normal"); got != 1 {
		t.Errorf("room1 closed (normal): want 1, got %v", got)
	}
	// Histogram observation: just verify the metric exists and has data.
	if !hasMetricWithName(t, reg, "wspulse_connection_duration_seconds") {
		t.Error("connection_duration_seconds metric missing")
	}
}

func TestResumeAttempt(t *testing.T) {
	t.Parallel()
	c, reg := newTestCollector(t)

	c.ResumeAttempt("room1", "conn1", true)

	if got := metricValue(t, reg, "wspulse_resume_attempts_total"); got != 1 {
		t.Errorf("resume attempts: want 1, got %v", got)
	}
}

// ── Room lifecycle ───────────────────────────────────────────────────────────

func TestRoomCreatedDestroyed(t *testing.T) {
	t.Parallel()
	c, reg := newTestCollector(t)

	c.RoomCreated("room1")
	c.RoomCreated("room2")

	if got := metricValue(t, reg, "wspulse_rooms_active"); got != 2 {
		t.Errorf("rooms active: want 2, got %v", got)
	}
	if got := metricValue(t, reg, "wspulse_rooms_created_total"); got != 2 {
		t.Errorf("rooms created: want 2, got %v", got)
	}

	c.RoomDestroyed("room1")

	if got := metricValue(t, reg, "wspulse_rooms_active"); got != 1 {
		t.Errorf("rooms active after destroy: want 1, got %v", got)
	}
	if got := metricValue(t, reg, "wspulse_rooms_destroyed_total"); got != 1 {
		t.Errorf("rooms destroyed: want 1, got %v", got)
	}
}

// ── Throughput ───────────────────────────────────────────────────────────────

func TestMessageReceived(t *testing.T) {
	t.Parallel()
	c, reg := newTestCollector(t)

	c.MessageReceived("room1", 100)
	c.MessageReceived("room1", 200)

	if got := requireMetricWithLabel(t, reg, "wspulse_messages_received_total", "room_id", "room1"); got != 2 {
		t.Errorf("room1 received: want 2, got %v", got)
	}
	if got := requireMetricWithLabel(t, reg, "wspulse_messages_received_bytes_total", "room_id", "room1"); got != 300 {
		t.Errorf("room1 received bytes: want 300, got %v", got)
	}
}

func TestMessageBroadcast(t *testing.T) {
	t.Parallel()
	c, reg := newTestCollector(t)

	c.MessageBroadcast("room1", 50, 10)

	if got := requireMetricWithLabel(t, reg, "wspulse_messages_broadcast_total", "room_id", "room1"); got != 1 {
		t.Errorf("room1 broadcast: want 1, got %v", got)
	}
	if !hasMetricWithName(t, reg, "wspulse_broadcast_fanout") {
		t.Error("broadcast_fanout metric missing")
	}
}

func TestMessageSent(t *testing.T) {
	t.Parallel()
	c, reg := newTestCollector(t)

	c.MessageSent("room1", "conn1", 100)

	if got := requireMetricWithLabel(t, reg, "wspulse_messages_sent_total", "room_id", "room1"); got != 1 {
		t.Errorf("room1 sent: want 1, got %v", got)
	}
}

func TestFrameDropped(t *testing.T) {
	t.Parallel()
	c, reg := newTestCollector(t)

	c.FrameDropped("room1", "conn1")
	c.FrameDropped("room1", "conn1")

	if got := requireMetricWithLabel(t, reg, "wspulse_frames_dropped_total", "room_id", "room1"); got != 2 {
		t.Errorf("room1 dropped: want 2, got %v", got)
	}
}

func TestSendBufferUtilization(t *testing.T) {
	t.Parallel()
	c, reg := newTestCollector(t)

	c.SendBufferUtilization("room1", "conn1", 128, 256)

	if got := requireMetricWithLabel(t, reg, "wspulse_send_buffer_utilization_ratio", "room_id", "room1"); got != 0.5 {
		t.Errorf("room1 buffer utilization: want 0.5, got %v", got)
	}
}

func TestSendBufferUtilization_ZeroCapacity(t *testing.T) {
	t.Parallel()
	c, reg := newTestCollector(t)

	c.SendBufferUtilization("room1", "conn1", 0, 0)

	if got := requireMetricWithLabel(t, reg, "wspulse_send_buffer_utilization_ratio", "room_id", "room1"); got != 0 {
		t.Errorf("room1 buffer utilization (zero cap): want 0, got %v", got)
	}
}

// ── Heartbeat ────────────────────────────────────────────────────────────────

func TestPongTimeout(t *testing.T) {
	t.Parallel()
	c, reg := newTestCollector(t)

	c.PongTimeout("room1", "conn1")

	if got := requireMetricWithLabel(t, reg, "wspulse_pong_timeouts_total", "room_id", "room1"); got != 1 {
		t.Errorf("room1 pong timeouts: want 1, got %v", got)
	}
}

// ── WithRoomLabel(false) ─────────────────────────────────────────────────────

func TestWithRoomLabel_False(t *testing.T) {
	t.Parallel()
	c, reg := newTestCollector(t, wsprom.WithRoomLabel(false))

	c.ConnectionOpened("room1", "conn1")
	c.MessageReceived("room1", 100)
	c.FrameDropped("room1", "conn1")
	c.PongTimeout("room1", "conn1")

	// Verify metrics exist but have no room_id label.
	if got := metricValue(t, reg, "wspulse_connections_opened_total"); got != 1 {
		t.Errorf("opened (no room label): want 1, got %v", got)
	}
	if hasLabel(t, reg, "wspulse_connections_opened_total", "room_id") {
		t.Error("room_id label should not exist when WithRoomLabel(false)")
	}
	if hasLabel(t, reg, "wspulse_messages_received_total", "room_id") {
		t.Error("room_id label should not exist on messages_received_total")
	}
}

// ── WithNamespace ────────────────────────────────────────────────────────────

func TestWithNamespace(t *testing.T) {
	t.Parallel()
	c, reg := newTestCollector(t, wsprom.WithNamespace("myapp"))

	c.RoomCreated("room1")
	c.ConnectionOpened("room1", "conn1")

	if !hasMetricWithName(t, reg, "myapp_wspulse_rooms_created_total") {
		mfs, _ := reg.Gather()
		names := make([]string, 0, len(mfs))
		for _, mf := range mfs {
			names = append(names, mf.GetName())
		}
		t.Errorf("expected myapp_wspulse_rooms_created_total, got: %v", names)
	}
	if !hasMetricWithName(t, reg, "myapp_wspulse_connections_opened_total") {
		t.Error("expected myapp_wspulse_connections_opened_total")
	}
}

// ── Handler ──────────────────────────────────────────────────────────────────

func TestHandler_ReturnsNonNil(t *testing.T) {
	t.Parallel()
	c, _ := newTestCollector(t)
	if c.Handler() == nil {
		t.Error("Handler() returned nil")
	}
}

// ── Scrape output ────────────────────────────────────────────────────────────

func TestScrapeOutput_ContainsExpectedMetrics(t *testing.T) {
	t.Parallel()
	c, reg := newTestCollector(t)

	c.ConnectionOpened("room1", "conn1")
	c.MessageReceived("room1", 42)
	c.RoomCreated("room1")

	mfs, gatherErr := reg.Gather()
	if gatherErr != nil {
		t.Fatalf("gather: %v", gatherErr)
	}
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
		if !gatheredNames[name] {
			t.Errorf("missing expected metric %q in scrape output", name)
		}
	}

}
