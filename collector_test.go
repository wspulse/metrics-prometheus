package prometheus_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	wsprom "github.com/wspulse/metrics-prometheus"
	wspulse "github.com/wspulse/server"
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

// histogramSampleCount returns the total sample count across all series of a histogram metric.
func histogramSampleCount(t *testing.T, reg *prometheus.Registry, name string) uint64 {
	t.Helper()
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
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
	t.Fatalf("histogram %q not found", name)
	return 0
}

// histogramSampleSum returns the total sample sum across all series of a histogram metric.
func histogramSampleSum(t *testing.T, reg *prometheus.Registry, name string) float64 {
	t.Helper()
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
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
	t.Fatalf("histogram %q not found", name)
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

func TestWithNamespace_InvalidPanics(t *testing.T) {
	t.Parallel()
	cases := []string{"my-app", "123abc", "has space", "special!char"}
	for _, ns := range cases {
		func(ns string) {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("expected panic for namespace %q", ns)
				}
			}()
			_ = wsprom.WithNamespace(ns)
		}(ns)
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
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for empty buckets")
		}
	}()
	_ = wsprom.WithConnectionDurationBuckets(nil)
}

func TestWithBroadcastFanoutBuckets_EmptyPanics(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for empty buckets")
		}
	}()
	_ = wsprom.WithBroadcastFanoutBuckets(nil)
}

func TestWithSendBufferUtilizationBuckets_EmptyPanics(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for empty buckets")
		}
	}()
	_ = wsprom.WithSendBufferUtilizationBuckets(nil)
}

func TestNewCollector_DuplicateRegistrationPanics(t *testing.T) {
	t.Parallel()
	reg := prometheus.NewRegistry()
	_ = wsprom.NewCollector(wsprom.WithRegisterer(reg), wsprom.WithGatherer(reg))

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for duplicate registration")
		}
	}()
	_ = wsprom.NewCollector(wsprom.WithRegisterer(reg), wsprom.WithGatherer(reg))
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
		t.Errorf("room1 closed (reason=normal): want 1, got %v", got)
	}
	if got := requireMetricWithLabel(t, reg, "wspulse_connections_closed_total", "room_id", "room1"); got != 1 {
		t.Errorf("room1 closed (room_id=room1): want 1, got %v", got)
	}
	// Histogram observation: verify metric exists and carries both labels.
	if !hasMetricWithName(t, reg, "wspulse_connection_duration_seconds") {
		t.Error("connection_duration_seconds metric missing")
	}
	if !hasLabel(t, reg, "wspulse_connection_duration_seconds", "reason") {
		t.Error("connection_duration_seconds missing reason label")
	}
}

func TestConnectionClosed_WithRoomLabelFalse(t *testing.T) {
	t.Parallel()
	c, reg := newTestCollector(t, wsprom.WithRoomLabel(false))

	c.ConnectionOpened("room1", "conn1")
	c.ConnectionClosed("room1", "conn1", 5*time.Second, wspulse.DisconnectNormal)

	if got := metricValue(t, reg, "wspulse_connections_active"); got != 0 {
		t.Errorf("active: want 0, got %v", got)
	}
	if got := metricValue(t, reg, "wspulse_connections_closed_total"); got != 1 {
		t.Errorf("closed: want 1, got %v", got)
	}
	if hasLabel(t, reg, "wspulse_connections_closed_total", "room_id") {
		t.Error("room_id label should not exist when WithRoomLabel(false)")
	}
	if !hasLabel(t, reg, "wspulse_connections_closed_total", "reason") {
		t.Error("reason label should still exist when WithRoomLabel(false)")
	}
}

func TestResumeAttempt(t *testing.T) {
	t.Parallel()
	c, reg := newTestCollector(t)

	c.ResumeAttempt("room1", "conn1")

	if got := requireMetricWithLabel(t, reg, "wspulse_resume_attempts_total", "room_id", "room1"); got != 1 {
		t.Errorf("resume attempts: want 1, got %v", got)
	}
	if hasLabel(t, reg, "wspulse_resume_attempts_total", "success") {
		t.Error("success label should not exist on resume_attempts_total")
	}
}

func TestResumeAttempt_WithRoomLabelFalse(t *testing.T) {
	t.Parallel()
	c, reg := newTestCollector(t, wsprom.WithRoomLabel(false))

	c.ResumeAttempt("room1", "conn1")

	if got := metricValue(t, reg, "wspulse_resume_attempts_total"); got != 1 {
		t.Errorf("resume attempts: want 1, got %v", got)
	}
	if hasLabel(t, reg, "wspulse_resume_attempts_total", "room_id") {
		t.Error("room_id label should not exist when WithRoomLabel(false)")
	}
	if hasLabel(t, reg, "wspulse_resume_attempts_total", "success") {
		t.Error("success label should not exist on resume_attempts_total")
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

// ── Throughput ─────────────────────────────────────────────────────────────────────

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

func TestMessageBroadcast_FanoutHistogramValue(t *testing.T) {
	t.Parallel()
	c, reg := newTestCollector(t)

	c.MessageBroadcast("room1", 50, 10)
	c.MessageBroadcast("room1", 50, 20)

	if got := histogramSampleCount(t, reg, "wspulse_broadcast_fanout"); got != 2 {
		t.Errorf("fanout sample count: want 2, got %v", got)
	}
	if got := histogramSampleSum(t, reg, "wspulse_broadcast_fanout"); got != 30 {
		t.Errorf("fanout sample sum: want 30 (10+20), got %v", got)
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
	c.SendBufferUtilization("room1", "conn2", 64, 256)

	// Histogram: two observations, sum = 0.5 + 0.25 = 0.75
	if got := histogramSampleCount(t, reg, "wspulse_send_buffer_utilization"); got != 2 {
		t.Errorf("buffer utilization sample count: want 2, got %v", got)
	}
	if got := histogramSampleSum(t, reg, "wspulse_send_buffer_utilization"); got != 0.75 {
		t.Errorf("buffer utilization sample sum: want 0.75, got %v", got)
	}
}

func TestSendBufferUtilization_ZeroCapacity(t *testing.T) {
	t.Parallel()
	c, reg := newTestCollector(t)

	c.SendBufferUtilization("room1", "conn1", 0, 0)

	if got := histogramSampleCount(t, reg, "wspulse_send_buffer_utilization"); got != 1 {
		t.Errorf("buffer utilization sample count: want 1, got %v", got)
	}
	if got := histogramSampleSum(t, reg, "wspulse_send_buffer_utilization"); got != 0 {
		t.Errorf("buffer utilization sample sum: want 0, got %v", got)
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

func TestWithRoomLabel_DefaultIsFalse(t *testing.T) {
	t.Parallel()
	reg := prometheus.NewRegistry()
	c := wsprom.NewCollector(wsprom.WithRegisterer(reg), wsprom.WithGatherer(reg))

	c.ConnectionOpened("room1", "conn1")

	if hasLabel(t, reg, "wspulse_connections_opened_total", "room_id") {
		t.Error("default roomLabel should be false; room_id label should not exist")
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

// ── Custom bucket options ────────────────────────────────────────────────────

func TestWithConnectionDurationBuckets(t *testing.T) {
	t.Parallel()
	c, reg := newTestCollector(t, wsprom.WithConnectionDurationBuckets([]float64{1, 10, 100}))

	c.ConnectionClosed("room1", "conn1", 5*time.Second, wspulse.DisconnectNormal)

	if got := histogramSampleCount(t, reg, "wspulse_connection_duration_seconds"); got != 1 {
		t.Errorf("sample count: want 1, got %v", got)
	}
}

func TestWithBroadcastFanoutBuckets(t *testing.T) {
	t.Parallel()
	c, reg := newTestCollector(t, wsprom.WithBroadcastFanoutBuckets([]float64{5, 50}))

	c.MessageBroadcast("room1", 50, 10)

	if got := histogramSampleCount(t, reg, "wspulse_broadcast_fanout"); got != 1 {
		t.Errorf("sample count: want 1, got %v", got)
	}
}

func TestWithSendBufferUtilizationBuckets(t *testing.T) {
	t.Parallel()
	c, reg := newTestCollector(t, wsprom.WithSendBufferUtilizationBuckets([]float64{0.5, 1.0}))

	c.SendBufferUtilization("room1", "conn1", 128, 256)

	if got := histogramSampleCount(t, reg, "wspulse_send_buffer_utilization"); got != 1 {
		t.Errorf("sample count: want 1, got %v", got)
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

func TestHandler_ServesPrometheusFormat(t *testing.T) {
	t.Parallel()
	c, _ := newTestCollector(t)

	c.RoomCreated("room1")
	c.ConnectionOpened("room1", "conn1")

	rec := httptest.NewRecorder()
	c.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "wspulse_rooms_created_total 1") {
		t.Errorf("expected rooms_created_total in scrape output, got:\n%s", body)
	}
	if !strings.Contains(body, "# HELP wspulse_rooms_created_total") {
		t.Error("expected HELP line in scrape output")
	}
	if !strings.Contains(body, "# TYPE wspulse_rooms_created_total counter") {
		t.Error("expected TYPE line in scrape output")
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
