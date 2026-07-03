package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bysempet/agentlens/api/internal/store"
)

// fakeStore devuelve datos preparados y registra la última consulta recibida.
type fakeStore struct {
	traces    []store.TraceSummary
	spans     map[string][]store.Span
	lastQuery store.ListQuery
}

func (f *fakeStore) ListTraces(_ context.Context, q store.ListQuery) ([]store.TraceSummary, error) {
	f.lastQuery = q
	if q.Limit < len(f.traces) {
		return f.traces[:q.Limit], nil
	}
	return f.traces, nil
}

func (f *fakeStore) TraceSpans(_ context.Context, tenantID, traceID string) ([]store.Span, error) {
	return f.spans[tenantID+"/"+traceID], nil
}

func newServer(f *fakeStore) *httptest.Server {
	return httptest.NewServer(NewHandler(f, map[string]string{"key-acme": "acme"}))
}

func get(t *testing.T, url, apiKey string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("no se pudo crear la petición: %v", err)
	}
	if apiKey != "" {
		req.Header.Set(HeaderAPIKey, apiKey)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("petición fallida: %v", err)
	}
	return resp
}

func decode[T any](t *testing.T, resp *http.Response) T {
	t.Helper()
	defer resp.Body.Close()
	var v T
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		t.Fatalf("respuesta ilegible: %v", err)
	}
	return v
}

func summaries(n int) []store.TraceSummary {
	out := make([]store.TraceSummary, n)
	for i := range out {
		out[i] = store.TraceSummary{
			TraceID:       fmt.Sprintf("trace-%03d", n-i), // descendente, como el store real
			StartUnixNano: int64((n - i) * 1_000),
		}
	}
	return out
}

func TestListTraces_RequiresAuth(t *testing.T) {
	srv := newServer(&fakeStore{})
	defer srv.Close()

	if resp := get(t, srv.URL+"/v1/traces", ""); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("sin key: esperado 401, obtenido %d", resp.StatusCode)
	}
	if resp := get(t, srv.URL+"/v1/traces", "key-falsa"); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("key inválida: esperado 401, obtenido %d", resp.StatusCode)
	}
}

func TestListTraces_ScopesToAuthenticatedTenant(t *testing.T) {
	f := &fakeStore{}
	srv := newServer(f)
	defer srv.Close()

	resp := get(t, srv.URL+"/v1/traces?agent_id=support-bot", "key-acme")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("esperado 200, obtenido %d", resp.StatusCode)
	}
	resp.Body.Close()
	if f.lastQuery.TenantID != "acme" {
		t.Fatalf("la consulta debe acotarse al tenant de la key (acme), obtenido %q", f.lastQuery.TenantID)
	}
	if f.lastQuery.AgentID != "support-bot" {
		t.Fatalf("filtro agent_id no propagado: %q", f.lastQuery.AgentID)
	}
}

func TestListTraces_PaginatesWithCursor(t *testing.T) {
	f := &fakeStore{traces: summaries(5)}
	srv := newServer(f)
	defer srv.Close()

	// Página 1: hay más resultados que el límite -> next_cursor presente.
	page1 := decode[listResponse](t, get(t, srv.URL+"/v1/traces?limit=3", "key-acme"))
	if len(page1.Traces) != 3 {
		t.Fatalf("página de 3 esperada, obtenidos %d", len(page1.Traces))
	}
	if page1.NextCursor == "" {
		t.Fatal("con más resultados disponibles debe haber next_cursor")
	}
	// El store recibió limit+1 (para detectar si hay siguiente página).
	if f.lastQuery.Limit != 4 {
		t.Fatalf("el store debe recibir limit+1=4, obtenido %d", f.lastQuery.Limit)
	}

	// Página 2: el cursor decodifica a la posición del último elemento servido.
	f.traces = summaries(2)
	page2 := decode[listResponse](t, get(t, srv.URL+"/v1/traces?limit=3&cursor="+page1.NextCursor, "key-acme"))
	last := page1.Traces[2]
	if f.lastQuery.BeforeStartUnixNano != last.StartUnixNano || f.lastQuery.BeforeTraceID != last.TraceID {
		t.Fatalf("cursor mal decodificado: %+v (esperado %d/%s)",
			f.lastQuery, last.StartUnixNano, last.TraceID)
	}
	// Última página: sin next_cursor.
	if page2.NextCursor != "" {
		t.Fatalf("última página no debe llevar next_cursor, obtenido %q", page2.NextCursor)
	}
}

func TestListTraces_RejectsBadParams(t *testing.T) {
	srv := newServer(&fakeStore{})
	defer srv.Close()

	if resp := get(t, srv.URL+"/v1/traces?limit=cero", "key-acme"); resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("limit inválido: esperado 400, obtenido %d", resp.StatusCode)
	}
	if resp := get(t, srv.URL+"/v1/traces?cursor=@@@", "key-acme"); resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("cursor inválido: esperado 400, obtenido %d", resp.StatusCode)
	}
}

func TestGetTrace_ReturnsSpansForOwnTenantOnly(t *testing.T) {
	f := &fakeStore{spans: map[string][]store.Span{
		"acme/t1":   {{SpanID: "s1", Name: "invoke_agent support-bot"}},
		"globex/t2": {{SpanID: "s2"}},
	}}
	srv := newServer(f)
	defer srv.Close()

	got := decode[traceResponse](t, get(t, srv.URL+"/v1/traces/t1", "key-acme"))
	if got.TraceID != "t1" || len(got.Spans) != 1 || got.Spans[0].SpanID != "s1" {
		t.Fatalf("detalle inesperado: %+v", got)
	}

	// La traza de otro tenant no existe para esta key -> 404, no 403 (no se
	// revela su existencia).
	if resp := get(t, srv.URL+"/v1/traces/t2", "key-acme"); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("traza ajena: esperado 404, obtenido %d", resp.StatusCode)
	}
}

func TestCursor_RoundTrip(t *testing.T) {
	cursor := encodeCursor(1_234_567, "trace-abc")
	ts, id, err := decodeCursor(cursor)
	if err != nil {
		t.Fatalf("cursor propio no decodifica: %v", err)
	}
	if ts != 1_234_567 || id != "trace-abc" {
		t.Fatalf("roundtrip incorrecto: %d/%s", ts, id)
	}
}
