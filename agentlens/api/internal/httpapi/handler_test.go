package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bysempet/agentlens/api/internal/store"
)

func seededStore() *store.MemoryStore {
	m := store.NewMemoryStore()
	base := time.Date(2026, 6, 25, 10, 0, 0, 0, time.UTC)
	// acme: 3 trazas (distinto inicio para comprobar el orden); globex: 1.
	for i, id := range []string{"T1", "T2", "T3"} {
		m.AddTrace("acme",
			store.TraceSummary{
				TraceID:   id,
				StartTime: base.Add(time.Duration(i) * time.Minute),
				SpanCount: uint64(i + 1),
			},
			[]store.Span{{SpanID: "s-" + id, SpanName: "invoke_agent"}},
		)
	}
	m.AddTrace("globex",
		store.TraceSummary{TraceID: "G1", StartTime: base},
		[]store.Span{{SpanID: "s-G1"}},
	)
	return m
}

func do(t *testing.T, h http.Handler, method, target, tenant string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, nil)
	if tenant != "" {
		req.Header.Set(TenantHeader, tenant)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("respuesta no es JSON: %v (%s)", err, rec.Body.String())
	}
	return body
}

func TestListTraces_OrderedAndTenantIsolated(t *testing.T) {
	h := New(seededStore())
	rec := do(t, h, "GET", "/v1/traces", "acme")
	if rec.Code != http.StatusOK {
		t.Fatalf("código %d", rec.Code)
	}
	body := decode(t, rec)
	traces := body["traces"].([]any)
	if len(traces) != 3 {
		t.Fatalf("esperadas 3 trazas de acme, %d", len(traces))
	}
	// Orden por inicio descendente: T3, T2, T1.
	first := traces[0].(map[string]any)
	if first["trace_id"] != "T3" {
		t.Fatalf("esperado T3 primero, %v", first["trace_id"])
	}
	if body["count"].(float64) != 3 {
		t.Fatalf("count incorrecto: %v", body["count"])
	}
}

func TestListTraces_Pagination(t *testing.T) {
	h := New(seededStore())
	rec := do(t, h, "GET", "/v1/traces?limit=1&offset=1", "acme")
	body := decode(t, rec)
	traces := body["traces"].([]any)
	if len(traces) != 1 {
		t.Fatalf("limit=1 debería devolver 1 traza, %d", len(traces))
	}
	// offset=1 sobre [T3,T2,T1] -> T2.
	if traces[0].(map[string]any)["trace_id"] != "T2" {
		t.Fatalf("offset=1 debería dar T2, %v", traces[0])
	}
	if body["limit"].(float64) != 1 || body["offset"].(float64) != 1 {
		t.Fatalf("eco de paginación incorrecto: %v", body)
	}
}

func TestListTraces_LimitClampedToMax(t *testing.T) {
	h := New(seededStore())
	rec := do(t, h, "GET", "/v1/traces?limit=99999", "acme")
	body := decode(t, rec)
	if body["limit"].(float64) != maxLimit {
		t.Fatalf("limit debería topar en %d, %v", maxLimit, body["limit"])
	}
}

func TestListTraces_MissingTenant(t *testing.T) {
	h := New(seededStore())
	rec := do(t, h, "GET", "/v1/traces", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("sin tenant debería ser 400, %d", rec.Code)
	}
}

func TestListTraces_InvalidLimit(t *testing.T) {
	h := New(seededStore())
	for _, bad := range []string{"limit=0", "limit=-3", "limit=abc", "offset=-1"} {
		rec := do(t, h, "GET", "/v1/traces?"+bad, "acme")
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%q debería ser 400, %d", bad, rec.Code)
		}
	}
}

func TestGetTrace_OK(t *testing.T) {
	h := New(seededStore())
	rec := do(t, h, "GET", "/v1/traces/T1", "acme")
	if rec.Code != http.StatusOK {
		t.Fatalf("código %d (%s)", rec.Code, rec.Body.String())
	}
	body := decode(t, rec)
	if body["trace_id"] != "T1" {
		t.Fatalf("trace_id incorrecto: %v", body["trace_id"])
	}
	if len(body["spans"].([]any)) != 1 {
		t.Fatalf("esperado 1 span, %v", body["spans"])
	}
}

func TestGetTrace_NotFoundForOtherTenant(t *testing.T) {
	h := New(seededStore())
	// T1 es de acme; globex no debe verla.
	rec := do(t, h, "GET", "/v1/traces/T1", "globex")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("traza de otro tenant debería ser 404, %d", rec.Code)
	}
}

func TestGetTrace_MissingTenant(t *testing.T) {
	h := New(seededStore())
	rec := do(t, h, "GET", "/v1/traces/T1", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("sin tenant debería ser 400, %d", rec.Code)
	}
}
