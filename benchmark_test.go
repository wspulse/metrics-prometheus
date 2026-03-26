package prometheus_test

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	wsprom "github.com/wspulse/metrics-prometheus"
	wspulse "github.com/wspulse/server"
)

func newBenchCollector(b *testing.B, opts ...wsprom.Option) *wsprom.Collector {
	b.Helper()
	reg := prometheus.NewRegistry()
	allOpts := append([]wsprom.Option{
		wsprom.WithRegisterer(reg),
		wsprom.WithGatherer(reg),
		wsprom.WithRoomLabel(true),
	}, opts...)
	return wsprom.NewCollector(allOpts...)
}

func BenchmarkConnectionOpened(b *testing.B) {
	c := newBenchCollector(b)
	b.ResetTimer()
	for b.Loop() {
		c.ConnectionOpened("room1", "conn1")
	}
}

func BenchmarkConnectionClosed(b *testing.B) {
	c := newBenchCollector(b)
	b.ResetTimer()
	for b.Loop() {
		c.ConnectionClosed("room1", "conn1", 5*time.Second, wspulse.DisconnectNormal)
	}
}

func BenchmarkMessageReceived(b *testing.B) {
	c := newBenchCollector(b)
	b.ResetTimer()
	for b.Loop() {
		c.MessageReceived("room1", 256)
	}
}

func BenchmarkMessageBroadcast(b *testing.B) {
	c := newBenchCollector(b)
	b.ResetTimer()
	for b.Loop() {
		c.MessageBroadcast("room1", 256, 10)
	}
}

func BenchmarkMessageSent(b *testing.B) {
	c := newBenchCollector(b)
	b.ResetTimer()
	for b.Loop() {
		c.MessageSent("room1", "conn1", 256)
	}
}

func BenchmarkSendBufferUtilization(b *testing.B) {
	c := newBenchCollector(b)
	b.ResetTimer()
	for b.Loop() {
		c.SendBufferUtilization("room1", "conn1", 128, 256)
	}
}

func BenchmarkFrameDropped(b *testing.B) {
	c := newBenchCollector(b)
	b.ResetTimer()
	for b.Loop() {
		c.FrameDropped("room1", "conn1")
	}
}

func BenchmarkPongTimeout(b *testing.B) {
	c := newBenchCollector(b)
	b.ResetTimer()
	for b.Loop() {
		c.PongTimeout("room1", "conn1")
	}
}

// BenchmarkConnectionOpened_NoRoomLabel measures the hot path without room_id
// label resolution.
func BenchmarkConnectionOpened_NoRoomLabel(b *testing.B) {
	c := newBenchCollector(b, wsprom.WithRoomLabel(false))
	b.ResetTimer()
	for b.Loop() {
		c.ConnectionOpened("room1", "conn1")
	}
}
