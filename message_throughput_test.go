package prometheus_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ── Message throughput (component) ───────────────────────────────────────────

func TestMessageThroughput_ReceiveBroadcastSent(t *testing.T) {
	t.Parallel()
	c, _ := newTestCollector(t)

	// Simulate: 2 connections in room, 1 message received, broadcast to both.
	c.ConnectionOpened("test-room", "conn1")
	c.ConnectionOpened("test-room", "conn2")
	c.MessageReceived("test-room", 16)
	c.MessageBroadcast("test-room", 16, 2)
	c.MessageSent("test-room", "conn1", 16)
	c.MessageSent("test-room", "conn2", 16)

	body := scrapeBody(t, c)

	assert.Contains(t, body, `wspulse_messages_received_total{room_id="test-room"} 1`,
		"expected 1 message received")
	assert.Contains(t, body, `wspulse_messages_broadcast_total{room_id="test-room"} 1`,
		"expected 1 broadcast")
	assert.Contains(t, body, `wspulse_messages_sent_total{room_id="test-room"} 2`,
		"expected 2 messages sent (fanout to 2 connections)")
}

func TestMessageThroughput_ReceivedBytes(t *testing.T) {
	t.Parallel()
	c, reg := newTestCollector(t)

	c.MessageReceived("test-room", 100)
	c.MessageReceived("test-room", 250)

	assert.Equal(t, float64(350),
		requireMetricWithLabel(t, reg, "wspulse_messages_received_bytes_total", "room_id", "test-room"),
		"received bytes should sum")
}

func TestMessageThroughput_BroadcastFanoutHistogram(t *testing.T) {
	t.Parallel()
	c, reg := newTestCollector(t)

	c.MessageBroadcast("test-room", 16, 5)
	c.MessageBroadcast("test-room", 16, 10)
	c.MessageBroadcast("test-room", 16, 50)

	assert.Equal(t, uint64(3),
		histogramSampleCount(t, reg, "wspulse_broadcast_fanout"),
		"3 fanout observations")
	assert.Equal(t, float64(65),
		histogramSampleSum(t, reg, "wspulse_broadcast_fanout"),
		"fanout sum = 5 + 10 + 50")
}

func TestMessageThroughput_FrameDropped(t *testing.T) {
	t.Parallel()
	c, _ := newTestCollector(t)

	c.FrameDropped("test-room", "conn1")
	c.FrameDropped("test-room", "conn1")
	c.FrameDropped("test-room", "conn2")

	body := scrapeBody(t, c)

	assert.Contains(t, body, `wspulse_frames_dropped_total{room_id="test-room"} 3`,
		"expected 3 frames dropped")
}

func TestMessageThroughput_SendBufferUtilization(t *testing.T) {
	t.Parallel()
	c, reg := newTestCollector(t)

	c.SendBufferUtilization("test-room", "conn1", 128, 256) // ratio=0.5
	c.SendBufferUtilization("test-room", "conn2", 192, 256) // ratio=0.75

	assert.Equal(t, uint64(2),
		histogramSampleCount(t, reg, "wspulse_send_buffer_utilization"),
		"2 buffer utilization observations")
	assert.Equal(t, 1.25,
		histogramSampleSum(t, reg, "wspulse_send_buffer_utilization"),
		"utilization sum = 0.5 + 0.75")
}

func TestMessageThroughput_MultipleRooms(t *testing.T) {
	t.Parallel()
	c, reg := newTestCollector(t)

	c.MessageReceived("room-a", 100)
	c.MessageReceived("room-b", 200)
	c.MessageSent("room-a", "conn1", 100)
	c.MessageSent("room-b", "conn2", 200)

	assert.Equal(t, float64(1),
		requireMetricWithLabel(t, reg, "wspulse_messages_received_total", "room_id", "room-a"),
		"room-a received 1")
	assert.Equal(t, float64(1),
		requireMetricWithLabel(t, reg, "wspulse_messages_received_total", "room_id", "room-b"),
		"room-b received 1")
	assert.Equal(t, float64(1),
		requireMetricWithLabel(t, reg, "wspulse_messages_sent_total", "room_id", "room-a"),
		"room-a sent 1")
	assert.Equal(t, float64(1),
		requireMetricWithLabel(t, reg, "wspulse_messages_sent_total", "room_id", "room-b"),
		"room-b sent 1")
}
