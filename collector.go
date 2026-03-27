// Package prometheus provides a Prometheus adapter for wspulse/server's
// MetricsCollector interface. It translates server lifecycle events into
// Prometheus counters, gauges, and histograms.
package prometheus

import (
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	wspulse "github.com/wspulse/server"
)

// Collector implements wspulse.MetricsCollector using Prometheus metrics.
// All methods are safe for concurrent use.
type Collector struct {
	cfg *collectorConfig

	// Connection lifecycle
	connectionsOpened  *prometheus.CounterVec
	connectionsClosed  *prometheus.CounterVec
	connectionsActive  *prometheus.GaugeVec
	connectionDuration *prometheus.HistogramVec
	resumeAttempts     *prometheus.CounterVec

	// Room
	roomsActive    prometheus.Gauge
	roomsCreated   prometheus.Counter
	roomsDestroyed prometheus.Counter

	// Throughput
	messagesReceived      *prometheus.CounterVec
	messagesReceivedBytes *prometheus.CounterVec
	messagesBroadcast     *prometheus.CounterVec
	broadcastFanout       *prometheus.HistogramVec
	messagesSent          *prometheus.CounterVec
	framesDropped         *prometheus.CounterVec
	sendBufferUtilization *prometheus.HistogramVec

	// Heartbeat
	pongTimeouts *prometheus.CounterVec
}

// compile-time check: Collector must satisfy wspulse.MetricsCollector.
var _ wspulse.MetricsCollector = (*Collector)(nil)

// NewCollector creates a Collector and registers all Prometheus metrics.
// Panics if metric registration fails (e.g. duplicate metric names).
func NewCollector(opts ...Option) *Collector {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	ns := "wspulse"
	if cfg.namespace != "" {
		ns = cfg.namespace + "_wspulse"
	}

	roomLabels := []string{}
	if cfg.roomLabel {
		roomLabels = []string{"room_id"}
	}

	roomReasonLabels := append([]string{}, roomLabels...)
	roomReasonLabels = append(roomReasonLabels, "reason")

	c := &Collector{
		cfg: cfg,

		// Connection lifecycle
		connectionsOpened: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns,
			Name:      "connections_opened_total",
			Help:      "Total number of connections opened.",
		}, roomLabels),
		connectionsClosed: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns,
			Name:      "connections_closed_total",
			Help:      "Total number of connections closed.",
		}, roomReasonLabels),
		connectionsActive: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns,
			Name:      "connections_active",
			Help:      "Number of currently active connections.",
		}, roomLabels),
		connectionDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: ns,
			Name:      "connection_duration_seconds",
			Help:      "Duration of connections in seconds.",
			Buckets:   cfg.connectionDurationBuckets,
		}, roomReasonLabels),
		resumeAttempts: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns,
			Name:      "resume_attempts_total",
			Help:      "Total number of session resume attempts.",
		}, roomLabels),

		// Room
		roomsActive: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: ns,
			Name:      "rooms_active",
			Help:      "Number of currently active rooms.",
		}),
		roomsCreated: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: ns,
			Name:      "rooms_created_total",
			Help:      "Total number of rooms created.",
		}),
		roomsDestroyed: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: ns,
			Name:      "rooms_destroyed_total",
			Help:      "Total number of rooms destroyed.",
		}),

		// Throughput
		messagesReceived: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns,
			Name:      "messages_received_total",
			Help:      "Total number of messages received.",
		}, roomLabels),
		messagesReceivedBytes: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns,
			Name:      "messages_received_bytes_total",
			Help:      "Total bytes of messages received.",
		}, roomLabels),
		messagesBroadcast: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns,
			Name:      "messages_broadcast_total",
			Help:      "Total number of messages broadcast.",
		}, roomLabels),
		broadcastFanout: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: ns,
			Name:      "broadcast_fanout",
			Help:      "Number of recipients per broadcast.",
			Buckets:   cfg.broadcastFanoutBuckets,
		}, roomLabels),
		messagesSent: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns,
			Name:      "messages_sent_total",
			Help:      "Total number of messages sent to connections.",
		}, roomLabels),
		framesDropped: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns,
			Name:      "frames_dropped_total",
			Help:      "Total number of frames dropped due to backpressure.",
		}, roomLabels),
		sendBufferUtilization: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: ns,
			Name:      "send_buffer_utilization",
			Help:      "Send buffer utilization ratio (used/capacity). Each connection report is one observation.",
			Buckets:   cfg.sendBufferUtilizationBuckets,
		}, roomLabels),

		// Heartbeat
		pongTimeouts: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns,
			Name:      "pong_timeouts_total",
			Help:      "Total number of pong timeouts.",
		}, roomLabels),
	}

	// Register all metrics.
	collectors := []prometheus.Collector{
		c.connectionsOpened, c.connectionsClosed, c.connectionsActive,
		c.connectionDuration, c.resumeAttempts,
		c.roomsActive, c.roomsCreated, c.roomsDestroyed,
		c.messagesReceived, c.messagesReceivedBytes,
		c.messagesBroadcast, c.broadcastFanout,
		c.messagesSent, c.framesDropped, c.sendBufferUtilization,
		c.pongTimeouts,
	}
	for _, col := range collectors {
		if err := cfg.registerer.Register(col); err != nil {
			panic(fmt.Sprintf("wspulse: failed to register metric: %v", err))
		}
	}

	return c
}

// ConnectionOpened increments the connections opened counter and active gauge.
func (c *Collector) ConnectionOpened(roomID, _ string) {
	if c.cfg.roomLabel {
		c.connectionsOpened.WithLabelValues(roomID).Inc()
		c.connectionsActive.WithLabelValues(roomID).Inc()
	} else {
		c.connectionsOpened.WithLabelValues().Inc()
		c.connectionsActive.WithLabelValues().Inc()
	}
}

// ConnectionClosed increments the connections closed counter, decrements the
// active gauge, and observes the connection duration. The reason label
// distinguishes disconnect causes (normal, kick, grace_expired, etc.).
func (c *Collector) ConnectionClosed(roomID, _ string, duration time.Duration, reason wspulse.DisconnectReason) {
	r := string(reason)
	if c.cfg.roomLabel {
		c.connectionsClosed.WithLabelValues(roomID, r).Inc()
		c.connectionsActive.WithLabelValues(roomID).Dec()
		c.connectionDuration.WithLabelValues(roomID, r).Observe(duration.Seconds())
	} else {
		c.connectionsClosed.WithLabelValues(r).Inc()
		c.connectionsActive.WithLabelValues().Dec()
		c.connectionDuration.WithLabelValues(r).Observe(duration.Seconds())
	}
}

// ResumeAttempt increments the resume attempts counter.
func (c *Collector) ResumeAttempt(roomID, _ string) {
	if c.cfg.roomLabel {
		c.resumeAttempts.WithLabelValues(roomID).Inc()
	} else {
		c.resumeAttempts.WithLabelValues().Inc()
	}
}

// RoomCreated increments the rooms created counter and active gauge.
func (c *Collector) RoomCreated(_ string) {
	c.roomsCreated.Inc()
	c.roomsActive.Inc()
}

// RoomDestroyed increments the rooms destroyed counter and decrements the active gauge.
func (c *Collector) RoomDestroyed(_ string) {
	c.roomsDestroyed.Inc()
	c.roomsActive.Dec()
}

// MessageReceived increments the messages received counter and bytes counter.
func (c *Collector) MessageReceived(roomID string, sizeBytes int) {
	if c.cfg.roomLabel {
		c.messagesReceived.WithLabelValues(roomID).Inc()
		c.messagesReceivedBytes.WithLabelValues(roomID).Add(float64(sizeBytes))
	} else {
		c.messagesReceived.WithLabelValues().Inc()
		c.messagesReceivedBytes.WithLabelValues().Add(float64(sizeBytes))
	}
}

// MessageBroadcast increments the messages broadcast counter and observes fanout.
func (c *Collector) MessageBroadcast(roomID string, _ int, fanOut int) {
	if c.cfg.roomLabel {
		c.messagesBroadcast.WithLabelValues(roomID).Inc()
		c.broadcastFanout.WithLabelValues(roomID).Observe(float64(fanOut))
	} else {
		c.messagesBroadcast.WithLabelValues().Inc()
		c.broadcastFanout.WithLabelValues().Observe(float64(fanOut))
	}
}

// MessageSent increments the messages sent counter.
func (c *Collector) MessageSent(roomID, _ string, _ int) {
	if c.cfg.roomLabel {
		c.messagesSent.WithLabelValues(roomID).Inc()
	} else {
		c.messagesSent.WithLabelValues().Inc()
	}
}

// FrameDropped increments the frames dropped counter.
func (c *Collector) FrameDropped(roomID, _ string) {
	if c.cfg.roomLabel {
		c.framesDropped.WithLabelValues(roomID).Inc()
	} else {
		c.framesDropped.WithLabelValues().Inc()
	}
}

// SendBufferUtilization observes the send buffer utilization ratio.
// Each call records one observation in the histogram, capturing the
// distribution across all connections rather than a last-write-wins value.
func (c *Collector) SendBufferUtilization(roomID, _ string, used, capacity int) {
	ratio := 0.0
	if capacity > 0 {
		ratio = float64(used) / float64(capacity)
	}
	if c.cfg.roomLabel {
		c.sendBufferUtilization.WithLabelValues(roomID).Observe(ratio)
	} else {
		c.sendBufferUtilization.WithLabelValues().Observe(ratio)
	}
}

// PongTimeout increments the pong timeouts counter.
func (c *Collector) PongTimeout(roomID, _ string) {
	if c.cfg.roomLabel {
		c.pongTimeouts.WithLabelValues(roomID).Inc()
	} else {
		c.pongTimeouts.WithLabelValues().Inc()
	}
}
