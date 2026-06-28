package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bysempet/agentlens/api/internal/agents"
)

func doJSON(t *testing.T, h http.Handler, method, target, key, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func agentHandler() http.Handler {
	return New(seededStore(), testKeys(), WithAgentStore(agents.NewMemoryStore()))
}

func TestAgents_CRUDLifecycle(t *testing.T) {
	h := agentHandler()

	// Create
	rec := doJSON(t, h, "POST", "/v1/agents", "key-acme", `{"name":"Support Bot","framework":"langchain"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create -> %d (%s)", rec.Code, rec.Body.String())
	}
	var created agents.Agent
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	if created.AgentID == "" || created.Status != "active" {
		t.Fatalf("agente creado inesperado: %+v", created)
	}

	// List
	rec = do(t, h, "GET", "/v1/agents", "key-acme")
	body := decode(t, rec)
	if body["count"].(float64) != 1 {
		t.Fatalf("se esperaba 1 agente, %v", body["count"])
	}

	// Get
	rec = do(t, h, "GET", "/v1/agents/"+created.AgentID, "key-acme")
	if rec.Code != http.StatusOK {
		t.Fatalf("get -> %d", rec.Code)
	}

	// Delete (baja)
	rec = do(t, h, "DELETE", "/v1/agents/"+created.AgentID, "key-acme")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete -> %d", rec.Code)
	}
}

func TestAgents_CreateRequiresName(t *testing.T) {
	h := agentHandler()
	rec := doJSON(t, h, "POST", "/v1/agents", "key-acme", `{"framework":"x"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("sin nombre debería ser 400, %d", rec.Code)
	}
}

func TestAgents_TenantIsolation(t *testing.T) {
	h := agentHandler()
	// acme crea un agente.
	rec := doJSON(t, h, "POST", "/v1/agents", "key-acme", `{"name":"Bot"}`)
	var a agents.Agent
	_ = json.Unmarshal(rec.Body.Bytes(), &a)
	// globex no debe verlo.
	rec = do(t, h, "GET", "/v1/agents/"+a.AgentID, "key-globex")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("el agente de acme no debería verse desde globex, %d", rec.Code)
	}
}

func TestAgents_RequiresAuth(t *testing.T) {
	h := agentHandler()
	rec := do(t, h, "GET", "/v1/agents", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("sin key debería ser 401, %d", rec.Code)
	}
}

func TestAgents_DisabledWhenNoStore(t *testing.T) {
	// Sin WithAgentStore, las rutas de agentes no existen (404 del mux).
	h := New(seededStore(), testKeys())
	rec := do(t, h, "GET", "/v1/agents", "key-acme")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("sin agent store, /v1/agents no debería existir, %d", rec.Code)
	}
}
