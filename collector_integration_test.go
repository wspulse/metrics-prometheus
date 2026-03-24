//go:build integration

package prometheus_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus"

	wsprom "github.com/wspulse/metrics-prometheus"
	wspulse "github.com/wspulse/server"
)

func dialWS(t *testing.T, url string) *websocket.Conn {
	t.Helper()
	dialer := websocket.Dialer{HandshakeTimeout: 3 * time.Second}
	c, _, err := dialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	return c
}

func startServer(t *testing.T, collector *wsprom.Collector) (*httptest.Server, wspulse.Server) {
	t.Helper()
	srv := wspulse.NewServer(
		func(r *http.Request) (string, string, error) {
			return "test-room", "", nil
		},
		wspulse.WithMetrics(collector),
	)
	ts := httptest.NewServer(srv)
	t.Cleanup(func() {
		srv.Close()
		ts.Close()
	})
	return ts, srv
}

func scrapeMetrics(t *testing.T, handler http.Handler) string {
	t.Helper()
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("scrape /metrics: status %d", rec.Code)
	}
	return rec.Body.String()
}

// ── Integration tests ────────────────────────────────────────────────────────

func TestIntegration_ConnectionLifecycle(t *testing.T) {
	reg := prometheus.NewRegistry()
	collector := wsprom.NewCollector(
		wsprom.WithRegisterer(reg),
		wsprom.WithGatherer(reg),
	)

	ts, _ := startServer(t, collector)
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")

	// Open 2 connections.
	c1 := dialWS(t, wsURL)
	c2 := dialWS(t, wsURL)

	// Give server time to process registrations.
	time.Sleep(100 * time.Millisecond)

	body := scrapeMetrics(t, collector.Handler())

	if !strings.Contains(body, `wspulse_connections_opened_total{room_id="test-room"} 2`) {
		t.Errorf("expected 2 connections opened, got:\n%s", body)
	}
	if !strings.Contains(body, `wspulse_connections_active{room_id="test-room"} 2`) {
		t.Errorf("expected 2 active connections, got:\n%s", body)
	}
	if !strings.Contains(body, `wspulse_rooms_active 1`) {
		t.Errorf("expected 1 active room, got:\n%s", body)
	}

	// Close connections.
	_ = c1.Close()
	_ = c2.Close()
	time.Sleep(200 * time.Millisecond)

	body = scrapeMetrics(t, collector.Handler())

	if !strings.Contains(body, `wspulse_connections_closed_total{room_id="test-room"} 2`) {
		t.Errorf("expected 2 connections closed, got:\n%s", body)
	}
	if !strings.Contains(body, `wspulse_connections_active{room_id="test-room"} 0`) {
		t.Errorf("expected 0 active connections after close, got:\n%s", body)
	}
	if !strings.Contains(body, `wspulse_rooms_active 0`) {
		t.Errorf("expected 0 active rooms after close, got:\n%s", body)
	}
}

func TestIntegration_MessageMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	collector := wsprom.NewCollector(
		wsprom.WithRegisterer(reg),
		wsprom.WithGatherer(reg),
	)

	var srv wspulse.Server
	srv = wspulse.NewServer(
		func(r *http.Request) (string, string, error) {
			return "test-room", "", nil
		},
		wspulse.WithMetrics(collector),
		wspulse.WithOnMessage(func(conn wspulse.Connection, f wspulse.Frame) {
			// Echo back as broadcast.
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

	time.Sleep(100 * time.Millisecond)

	// Send a message from c1 — should trigger MessageReceived + MessageBroadcast.
	err := c1.WriteMessage(websocket.TextMessage, []byte(`{"event":"ping"}`))
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	// Wait for broadcast to be processed.
	time.Sleep(200 * time.Millisecond)

	body := scrapeMetrics(t, collector.Handler())

	if !strings.Contains(body, `wspulse_messages_received_total{room_id="test-room"} 1`) {
		t.Errorf("expected 1 message received, got:\n%s", body)
	}
	if !strings.Contains(body, `wspulse_messages_broadcast_total{room_id="test-room"} 1`) {
		t.Errorf("expected 1 broadcast, got:\n%s", body)
	}
	// 2 connections in room → fanout = 2, each gets a MessageSent.
	if !strings.Contains(body, `wspulse_messages_sent_total{room_id="test-room"} 2`) {
		t.Errorf("expected 2 messages sent (fanout to 2 connections), got:\n%s", body)
	}
}
