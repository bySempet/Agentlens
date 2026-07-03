package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bysempet/agentlens/api/internal/auth"
	"github.com/bysempet/agentlens/api/internal/store"
)

// keys de prueba: cada API key resuelve a un tenant.
func testKeys() auth.KeyStore {
	return auth.NewStaticKeyStore(map[string]string{
		"key-acme":   "acme",
		"key-globex": "globex",
	})
}

func seededStore() *store.MemoryStore {
	m := store.NewMemoryStore()
	base := time.Date(2026, 6, 25, 10, 0, 0, 0, time.UTC)
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

// do ejecuta una petición autenticada con la API key dada (vacía = sin auth).
func do(t *testing.T, h http.Handler, method, target, key string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, nil)
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
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
	h := New(seededStore(), testKeys())
	rec := do(t, h, "GET", "/v1/traces", "key-acme")
	if rec.Code != http.StatusOK {
		t.Fatalf("código %d", rec.Code)
	}
	body := decode(t, rec)
	traces := body["traces"].([]any)
	if len(traces) != 3 {
		t.Fatalf("esperadas 3 trazas de acme, %d", len(traces))
	}
	if traces[0].(map[string]any)["trace_id"] != "T3" {
		t.Fatalf("esperado T3 primero (orden desc), %v", traces[0])
	}
}

func TestListTraces_Pagination(t *testing.T) {
	h := New(seededStore(), testKeys())
	rec := do(t, h, "GET", "/v1/traces?limit=1&offset=1", "key-acme")
	body := decode(t, rec)
	traces := body["traces"].([]any)
	if len(traces) != 1 || traces[0].(map[string]any)["trace_id"] != "T2" {
		t.Fatalf("limit=1&offset=1 debería dar [T2], %v", traces)
	}
	if body["limit"].(float64) != 1 || body["offset"].(float64) != 1 {
		t.Fatalf("eco de paginación incorrecto: %v", body)
	}
}

func TestListTraces_LimitClampedToMax(t *testing.T) {
	h := New(seededStore(), testKeys())
	rec := do(t, h, "GET", "/v1/traces?limit=99999", "key-acme")
	if decode(t, rec)["limit"].(float64) != maxLimit {
		t.Fatalf("limit debería topar en %d", maxLimit)
	}
}

func TestListTraces_InvalidParams(t *testing.T) {
	h := New(seededStore(), testKeys())
	for _, bad := range []string{"limit=0", "limit=-3", "limit=abc", "offset=-1"} {
		rec := do(t, h, "GET", "/v1/traces?"+bad, "key-acme")
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%q debería ser 400, %d", bad, rec.Code)
		}
	}
}

func TestAuth_MissingKeyIs401(t *testing.T) {
	h := New(seededStore(), testKeys())
	rec := do(t, h, "GET", "/v1/traces", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("sin API key debería ser 401, %d", rec.Code)
	}
}

func TestAuth_InvalidKeyIs401(t *testing.T) {
	h := New(seededStore(), testKeys())
	rec := do(t, h, "GET", "/v1/traces", "key-falsa")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("API key inválida debería ser 401, %d", rec.Code)
	}
}

func TestGetTrace_OK(t *testing.T) {
	h := New(seededStore(), testKeys())
	rec := do(t, h, "GET", "/v1/traces/T1", "key-acme")
	if rec.Code != http.StatusOK {
		t.Fatalf("código %d (%s)", rec.Code, rec.Body.String())
	}
	body := decode(t, rec)
	if body["trace_id"] != "T1" || len(body["spans"].([]any)) != 1 {
		t.Fatalf("detalle incorrecto: %v", body)
	}
}

func TestGetTrace_TenantIsolatedByKey(t *testing.T) {
	h := New(seededStore(), testKeys())
	// T1 es de acme; con la key de globex no debe verse (404, no 200).
	rec := do(t, h, "GET", "/v1/traces/T1", "key-globex")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("traza de otro tenant debería ser 404, %d", rec.Code)
	}
}

func TestPublicEndpoints_NoAuth(t *testing.T) {
	h := New(seededStore(), testKeys())
	for _, path := range []string{"/healthz", "/openapi.yaml"} {
		rec := do(t, h, "GET", path, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("%s debería ser público (200), %d", path, rec.Code)
		}
	}
}

func TestGetCost_AggregatesByAgentAndModel(t *testing.T) {
	m := seededStore()
	m.AddCostRow("acme", store.CostRow{AgentID: "support-bot", Model: "gpt-4o", InputTokens: 1_000_000, OutputTokens: 0})
	m.AddCostRow("acme", store.CostRow{AgentID: "support-bot", Model: "gpt-4o", InputTokens: 0, OutputTokens: 1_000_000})
	h := New(m, testKeys())

	rec := do(t, h, "GET", "/v1/cost", "key-acme")
	if rec.Code != http.StatusOK {
		t.Fatalf("código %d (%s)", rec.Code, rec.Body.String())
	}
	body := decode(t, rec)
	// 2.5 (in) + 10 (out) = 12.5
	if total := body["total_cost_usd"].(float64); total < 12.49 || total > 12.51 {
		t.Fatalf("total_cost_usd = %v, esperado ~12.5", total)
	}
	byAgent := body["by_agent"].([]any)
	if len(byAgent) != 1 || byAgent[0].(map[string]any)["agent_id"] != "support-bot" {
		t.Fatalf("by_agent incorrecto: %v", byAgent)
	}
}

func TestGetCost_RequiresAuth(t *testing.T) {
	h := New(seededStore(), testKeys())
	rec := do(t, h, "GET", "/v1/cost", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("sin key debería ser 401, %d", rec.Code)
	}
}

func TestOpenAPI_ServesSpec(t *testing.T) {
	h := New(seededStore(), testKeys())
	rec := do(t, h, "GET", "/openapi.yaml", "")
	if ct := rec.Header().Get("Content-Type"); ct != "application/yaml" {
		t.Fatalf("content-type inesperado: %q", ct)
	}
	if len(rec.Body.Bytes()) == 0 {
		t.Fatal("la spec OpenAPI servida está vacía")
	}
}
