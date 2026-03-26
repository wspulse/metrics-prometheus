package prometheus

import (
	"regexp"

	"github.com/prometheus/client_golang/prometheus"
)

// Option configures a Collector.
type Option func(*collectorConfig)

type collectorConfig struct {
	registerer prometheus.Registerer
	gatherer   prometheus.Gatherer
	namespace  string
	roomLabel  bool

	connectionDurationBuckets    []float64
	broadcastFanoutBuckets       []float64
	sendBufferUtilizationBuckets []float64
}

// defaultConnectionDurationBuckets covers short-lived to long-lived WebSocket
// connections (up to 24 hours).
var defaultConnectionDurationBuckets = []float64{
	1, 5, 15, 30, 60, 300, 900, 3600, 7200, 14400, 43200, 86400,
}

// defaultBroadcastFanoutBuckets covers small rooms (1-10) to large broadcast
// rooms (up to 1000 recipients).
var defaultBroadcastFanoutBuckets = []float64{
	1, 2, 5, 10, 25, 50, 100, 500, 1000,
}

// defaultSendBufferUtilizationBuckets covers the 0.0-1.0 ratio range with
// finer granularity near saturation.
var defaultSendBufferUtilizationBuckets = []float64{
	0.1, 0.25, 0.5, 0.75, 0.9, 0.95, 0.99, 1.0,
}

func defaultConfig() *collectorConfig {
	return &collectorConfig{
		registerer:                   prometheus.DefaultRegisterer,
		gatherer:                     prometheus.DefaultGatherer,
		namespace:                    "",
		roomLabel:                    false,
		connectionDurationBuckets:    defaultConnectionDurationBuckets,
		broadcastFanoutBuckets:       defaultBroadcastFanoutBuckets,
		sendBufferUtilizationBuckets: defaultSendBufferUtilizationBuckets,
	}
}

// validNamespace matches Prometheus metric name components: letters, digits,
// and underscores, starting with a letter or underscore.
var validNamespace = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// WithRegisterer sets the prometheus.Registerer used to register metrics.
// Defaults to prometheus.DefaultRegisterer.
// Panics if r is nil.
func WithRegisterer(r prometheus.Registerer) Option {
	if r == nil {
		panic("wspulse: WithRegisterer: registerer must not be nil")
	}
	return func(c *collectorConfig) { c.registerer = r }
}

// WithGatherer sets the prometheus.Gatherer used by Handler().
// Defaults to prometheus.DefaultGatherer.
// Panics if g is nil.
func WithGatherer(g prometheus.Gatherer) Option {
	if g == nil {
		panic("wspulse: WithGatherer: gatherer must not be nil")
	}
	return func(c *collectorConfig) { c.gatherer = g }
}

// WithNamespace sets a prefix for all metric names.
// For example, WithNamespace("myapp") produces "myapp_wspulse_connections_opened_total".
// Defaults to "" (metrics named "wspulse_...").
// Panics if ns contains characters outside [a-zA-Z0-9_] or starts with a digit.
func WithNamespace(ns string) Option {
	if ns != "" && !validNamespace.MatchString(ns) {
		panic("wspulse: WithNamespace: namespace must match [a-zA-Z_][a-zA-Z0-9_]*, got " + ns)
	}
	return func(c *collectorConfig) { c.namespace = ns }
}

// WithRoomLabel controls whether room_id is included as a label on per-room
// metrics (i.e. connection, throughput, and heartbeat metrics that are scoped
// to a specific room). Room-level aggregate metrics (rooms_active, rooms_created,
// rooms_destroyed) never include room_id.
//
// Defaults to false. Enable with WithRoomLabel(true) only when the number of
// distinct rooms is bounded and known (e.g. a fixed set of chat rooms). In
// high-cardinality environments (e.g. one room per user or per livestream),
// enabling this option causes excessive label combinations that may exhaust
// Prometheus memory.
func WithRoomLabel(enabled bool) Option {
	return func(c *collectorConfig) { c.roomLabel = enabled }
}

// WithConnectionDurationBuckets sets custom histogram buckets for the
// connection_duration_seconds metric. Defaults cover 1s to 24h.
// Panics if buckets is empty.
func WithConnectionDurationBuckets(buckets []float64) Option {
	if len(buckets) == 0 {
		panic("wspulse: WithConnectionDurationBuckets: buckets must not be empty")
	}
	return func(c *collectorConfig) { c.connectionDurationBuckets = buckets }
}

// WithBroadcastFanoutBuckets sets custom histogram buckets for the
// broadcast_fanout metric. Defaults cover 1 to 1000 recipients.
// Panics if buckets is empty.
func WithBroadcastFanoutBuckets(buckets []float64) Option {
	if len(buckets) == 0 {
		panic("wspulse: WithBroadcastFanoutBuckets: buckets must not be empty")
	}
	return func(c *collectorConfig) { c.broadcastFanoutBuckets = buckets }
}

// WithSendBufferUtilizationBuckets sets custom histogram buckets for the
// send_buffer_utilization metric. Defaults cover 0.1 to 1.0 ratio with
// finer granularity near saturation.
// Panics if buckets is empty.
func WithSendBufferUtilizationBuckets(buckets []float64) Option {
	if len(buckets) == 0 {
		panic("wspulse: WithSendBufferUtilizationBuckets: buckets must not be empty")
	}
	return func(c *collectorConfig) { c.sendBufferUtilizationBuckets = buckets }
}
