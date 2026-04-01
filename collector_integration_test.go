//go:build integration

package prometheus_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	wsprom "github.com/wspulse/metrics-prometheus"
	wspulse "github.com/wspulse/server"
)

func awaitChan(t *testing.T, ch <-chan struct{}, label string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(3 * time.Second):
		require.Failf(t, "timeout", "timed out waiting for %s", label)
	}
}

func dialWS(t *testing.T, url string) *websocket.Conn {
	t.Helper()
	dialer := websocket.Dialer{HandshakeTimeout: 3 * time.Second}
	c, resp, err := dialer.Dial(url, nil)
	require.NoError(t, err, "Dial failed")
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	return c
}

func scrapeMetrics(t *testing.T, handler http.Handler) string {
	t.Helper()
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	require.Equal(t, http.StatusOK, rec.Code, "scrape /metrics")
	return rec.Body.String()
}

// ── Integration tests ────────────────────────────────────────────────────────

func TestIntegration_ConnectionLifecycle(t *testing.T) {
	reg := prometheus.NewRegistry()
	collector := wsprom.NewCollector(
		wsprom.WithRegisterer(reg),
		wsprom.WithGatherer(reg),
		wsprom.WithRoomLabel(true),
	)

	connected := make(chan struct{}, 4)
	disconnected := make(chan struct{}, 4)

	srv := wspulse.NewServer(
		func(r *http.Request) (string, string, error) {
			return "test-room", "", nil
		},
		wspulse.WithMetrics(collector),
		wspulse.WithOnConnect(func(_ wspulse.Connection) {
			connected <- struct{}{}
		}),
		wspulse.WithOnDisconnect(func(_ wspulse.Connection, _ error) {
			disconnected <- struct{}{}
		}),
	)
	ts := httptest.NewServer(srv)
	t.Cleanup(func() {
		srv.Close()
		ts.Close()
	})

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")

	// Open 2 connections and wait for server to register them.
	c1 := dialWS(t, wsURL)
	c2 := dialWS(t, wsURL)
	awaitChan(t, connected, "connected (c1)")
	awaitChan(t, connected, "connected (c2)")

	body := scrapeMetrics(t, collector.Handler())

	assert.Contains(t, body, `wspulse_connections_opened_total{room_id="test-room"} 2`, "expected 2 connections opened")
	assert.Contains(t, body, `wspulse_connections_active{room_id="test-room"} 2`, "expected 2 active connections")
	assert.Contains(t, body, `wspulse_rooms_active 1`, "expected 1 active room")

	// Close connections and wait for server to process disconnects.
	_ = c1.Close()
	_ = c2.Close()
	awaitChan(t, disconnected, "disconnected (c1)")
	awaitChan(t, disconnected, "disconnected (c2)")

	body = scrapeMetrics(t, collector.Handler())

	closedMatch := strings.Contains(body, `wspulse_connections_closed_total{reason="normal",room_id="test-room"} 2`) ||
		strings.Contains(body, `wspulse_connections_closed_total{room_id="test-room",reason="normal"} 2`)
	assert.True(t, closedMatch, "expected 2 connections closed (reason=normal)")
	assert.Contains(t, body, `wspulse_connections_active{room_id="test-room"} 0`, "expected 0 active connections after close")
	assert.Contains(t, body, `wspulse_rooms_active 0`, "expected 0 active rooms after close")
}

func TestIntegration_MessageMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	collector := wsprom.NewCollector(
		wsprom.WithRegisterer(reg),
		wsprom.WithGatherer(reg),
		wsprom.WithRoomLabel(true),
	)

	connected := make(chan struct{}, 4)
	var broadcastDone sync.WaitGroup

	var srv wspulse.Server
	srv = wspulse.NewServer(
		func(r *http.Request) (string, string, error) {
			return "test-room", "", nil
		},
		wspulse.WithMetrics(collector),
		wspulse.WithOnConnect(func(_ wspulse.Connection) {
			connected <- struct{}{}
		}),
		wspulse.WithOnMessage(func(conn wspulse.Connection, f wspulse.Frame) {
			defer broadcastDone.Done()
			_ = srv.Broadcast(conn.RoomID(), f)
		}),
	)
	ts := httptest.NewServer(srv)
	t.Cleanup(func() {
		srv.Close()
		ts.Close()
	})

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")

	c1 := dialWS(t, wsURL)
	defer c1.Close()
	c2 := dialWS(t, wsURL)
	defer c2.Close()
	awaitChan(t, connected, "connected (c1)")
	awaitChan(t, connected, "connected (c2)")

	// Send a message from c1 — triggers MessageReceived + MessageBroadcast.
	broadcastDone.Add(1)
	err := c1.WriteMessage(websocket.TextMessage, []byte(`{"event":"ping"}`))
	require.NoError(t, err, "write")

	// Wait for broadcast to complete.
	broadcastDone.Wait()

	// Read the broadcast messages on both clients to ensure MessageSent hooks fired.
	// writePump sends asynchronously, so read from both to synchronize.
	c1.SetReadDeadline(time.Now().Add(3 * time.Second))
	c2.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, _, err = c1.ReadMessage()
	require.NoError(t, err, "read from c1")
	_, _, err = c2.ReadMessage()
	require.NoError(t, err, "read from c2")

	body := scrapeMetrics(t, collector.Handler())

	assert.Contains(t, body, `wspulse_messages_received_total{room_id="test-room"} 1`, "expected 1 message received")
	assert.Contains(t, body, `wspulse_messages_broadcast_total{room_id="test-room"} 1`, "expected 1 broadcast")
	assert.Contains(t, body, `wspulse_messages_sent_total{room_id="test-room"} 2`, "expected 2 messages sent (fanout to 2 connections)")
}
