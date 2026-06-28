package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/bysempet/agentlens/api/internal/store"
)

func wsURL(srv *httptest.Server) string {
	return "ws" + strings.TrimPrefix(srv.URL, "http") + "/v1/stream"
}

func TestStream_PushesNewTracesForTenant(t *testing.T) {
	m := store.NewMemoryStore()
	h := New(m, testKeys(), WithStreamInterval(20*time.Millisecond))
	srv := httptest.NewServer(h)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c, _, err := websocket.Dial(ctx, wsURL(srv), &websocket.DialOptions{
		HTTPHeader: http.Header{"Authorization": {"Bearer key-acme"}},
	})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.CloseNow()

	// Traza nueva (iniciada tras la conexión) -> debe llegar por el feed.
	m.AddTrace("acme",
		store.TraceSummary{TraceID: "live-1", StartTime: time.Now().Add(time.Second)},
		nil,
	)

	var got store.TraceSummary
	if err := wsjson.Read(ctx, c, &got); err != nil {
		t.Fatalf("read: %v", err)
	}
	if got.TraceID != "live-1" {
		t.Fatalf("esperado live-1 por el live feed, obtenido %q", got.TraceID)
	}
}

func TestStream_RequiresAuth(t *testing.T) {
	h := New(store.NewMemoryStore(), testKeys(), WithStreamInterval(20*time.Millisecond))
	srv := httptest.NewServer(h)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Sin cabecera de auth, el upgrade WebSocket debe rechazarse (401).
	c, _, err := websocket.Dial(ctx, wsURL(srv), nil)
	if err == nil {
		c.CloseNow()
		t.Fatal("se esperaba fallo de handshake sin API key")
	}
}
