package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Option configures a Collector.
type Option func(*collectorConfig)

type collectorConfig struct {
	registerer prometheus.Registerer
	gatherer   prometheus.Gatherer
	namespace  string
	roomLabel  bool
}

func defaultConfig() *collectorConfig {
	return &collectorConfig{
		registerer: prometheus.DefaultRegisterer,
		gatherer:   prometheus.DefaultGatherer,
		namespace:  "",
		roomLabel:  true,
	}
}

// WithRegisterer sets the prometheus.Registerer used to register metrics.
// Defaults to prometheus.DefaultRegisterer.
// Panics if r is nil.
func WithRegisterer(r prometheus.Registerer) Option {
	if r == nil {
		panic("wspulse/metrics-prometheus: WithRegisterer: registerer must not be nil")
	}
	return func(c *collectorConfig) { c.registerer = r }
}

// WithGatherer sets the prometheus.Gatherer used by Handler().
// Defaults to prometheus.DefaultGatherer.
// Panics if g is nil.
func WithGatherer(g prometheus.Gatherer) Option {
	if g == nil {
		panic("wspulse/metrics-prometheus: WithGatherer: gatherer must not be nil")
	}
	return func(c *collectorConfig) { c.gatherer = g }
}

// WithNamespace sets a prefix for all metric names.
// For example, WithNamespace("myapp") produces "myapp_wspulse_connections_opened_total".
// Defaults to "" (metrics named "wspulse_...").
func WithNamespace(ns string) Option {
	return func(c *collectorConfig) { c.namespace = ns }
}

// WithRoomLabel controls whether room_id is included as a label on metrics.
// Defaults to true. Set to false in high-cardinality environments (e.g. one
// room per livestream) to avoid excessive label combinations.
func WithRoomLabel(enabled bool) Option {
	return func(c *collectorConfig) { c.roomLabel = enabled }
}
