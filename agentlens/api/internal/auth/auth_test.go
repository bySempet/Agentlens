package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func store() KeyStore {
	return NewStaticKeyStore(map[string]string{"key-acme": "acme"})
}

// echoTenant es un handler final que escribe el tenant resuelto, o 500 si falta.
func echoTenant(w http.ResponseWriter, r *http.Request) {
	tenant, ok := TenantFromContext(r.Context())
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	_, _ = w.Write([]byte(tenant))
}

func TestMiddleware_ValidBearerResolvesTenant(t *testing.T) {
	h := Middleware(http.HandlerFunc(echoTenant), store())
	req := httptest.NewRequest("GET", "/v1/traces", nil)
	req.Header.Set("Authorization", "Bearer key-acme")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "acme" {
		t.Fatalf("esperado tenant acme, code=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestMiddleware_XAgentLensKeyHeaderAlsoWorks(t *testing.T) {
	h := Middleware(http.HandlerFunc(echoTenant), store())
	req := httptest.NewRequest("GET", "/v1/traces", nil)
	req.Header.Set("X-AgentLens-Key", "key-acme")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Body.String() != "acme" {
		t.Fatalf("X-AgentLens-Key debería resolver el tenant, body=%q", rec.Body.String())
	}
}

func TestMiddleware_QueryParamKeyOnlyForWebSocket(t *testing.T) {
	// Con upgrade WebSocket, el query param api_key autentica.
	h := Middleware(http.HandlerFunc(echoTenant), store())
	req := httptest.NewRequest("GET", "/v1/stream?api_key=key-acme", nil)
	req.Header.Set("Upgrade", "websocket")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Body.String() != "acme" {
		t.Fatalf("api_key por query (WS) debería resolver el tenant, body=%q", rec.Body.String())
	}
}

func TestMiddleware_QueryParamRejectedForREST(t *testing.T) {
	// Sin upgrade (REST normal), el query param NO debe autenticar.
	h := Middleware(http.HandlerFunc(echoTenant), store())
	req := httptest.NewRequest("GET", "/v1/traces?api_key=key-acme", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("api_key por query en REST debería rechazarse (401), code=%d", rec.Code)
	}
}

func TestMiddleware_MissingKeyIs401(t *testing.T) {
	h := Middleware(http.HandlerFunc(echoTenant), store())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/traces", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("esperado 401, %d", rec.Code)
	}
	if rec.Header().Get("WWW-Authenticate") == "" {
		t.Fatal("falta la cabecera WWW-Authenticate en el 401")
	}
}

func TestMiddleware_InvalidKeyIs401(t *testing.T) {
	h := Middleware(http.HandlerFunc(echoTenant), store())
	req := httptest.NewRequest("GET", "/v1/traces", nil)
	req.Header.Set("Authorization", "Bearer nope")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("esperado 401, %d", rec.Code)
	}
}

func TestMiddleware_PublicPathSkipsAuth(t *testing.T) {
	called := false
	final := func(w http.ResponseWriter, _ *http.Request) { called = true }
	h := Middleware(http.HandlerFunc(final), store(), "/healthz")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/healthz", nil))
	if !called {
		t.Fatal("la ruta pública debería ejecutarse sin auth")
	}
}
