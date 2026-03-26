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

	wsprom "github.com/wspulse/metrics-prometheus"
	wspulse "github.com/wspulse/server"
)

func dialWS(t *testing.T, url string) *websocket.Conn {
	t.Helper()
	dialer := websocket.Dialer{HandshakeTimeout: 3 * time.Second}
	c, resp, err := dialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	return c
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
	<-connected
	<-connected

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

	// Close connections and wait for server to process disconnects.
	_ = c1.Close()
	_ = c2.Close()
	<-disconnected
	<-disconnected

	body = scrapeMetrics(t, collector.Handler())

	if !strings.Contains(body, `wspulse_connections_closed_total{reason="normal",room_id="test-room"} 2`) {
		t.Errorf("expected 2 connections closed (reason=normal), got:\n%s", body)
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
	<-connected
	<-connected

	// Send a message from c1 — triggers MessageReceived + MessageBroadcast.
	broadcastDone.Add(1)
	err := c1.WriteMessage(websocket.TextMessage, []byte(`{"event":"ping"}`))
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	// Wait for broadcast to complete.
	broadcastDone.Wait()

	// Read the broadcast messages on both clients to ensure MessageSent hooks fired.
	// writePump sends asynchronously, so read from both to synchronize.
	c1.SetReadDeadline(time.Now().Add(3 * time.Second))
	c2.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, _, err := c1.ReadMessage(); err != nil {
		t.Fatalf("read from c1: %v", err)
	}
	if _, _, err := c2.ReadMessage(); err != nil {
		t.Fatalf("read from c2: %v", err)
	}

	body := scrapeMetrics(t, collector.Handler())

	if !strings.Contains(body, `wspulse_messages_received_total{room_id="test-room"} 1`) {
		t.Errorf("expected 1 message received, got:\n%s", body)
	}
	if !strings.Contains(body, `wspulse_messages_broadcast_total{room_id="test-room"} 1`) {
		t.Errorf("expected 1 broadcast, got:\n%s", body)
	}
	if !strings.Contains(body, `wspulse_messages_sent_total{room_id="test-room"} 2`) {
		t.Errorf("expected 2 messages sent (fanout to 2 connections), got:\n%s", body)
	}
}
