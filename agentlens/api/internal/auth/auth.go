// Package auth resuelve el tenant a partir de una API key en la API de lectura
// (E3-T02). Sustituye la confianza en una cabecera de tenant (spoofeable) por
// autenticación real: la key identifica al tenant y el cliente solo ve sus datos.
package auth

import (
	"context"
	"net/http"
	"strings"
)

type tenantCtxKey struct{}

// KeyStore resuelve una API key a su tenant.
type KeyStore interface {
	TenantForKey(apiKey string) (tenantID string, ok bool)
}

// StaticKeyStore es un KeyStore en memoria (key -> tenant) para MVP y tests.
type StaticKeyStore struct{ keys map[string]string }

// NewStaticKeyStore crea el store a partir de un mapa key -> tenantID.
func NewStaticKeyStore(keys map[string]string) *StaticKeyStore {
	cp := make(map[string]string, len(keys))
	for k, v := range keys {
		cp[k] = v
	}
	return &StaticKeyStore{keys: cp}
}

// TenantForKey implementa KeyStore.
func (s *StaticKeyStore) TenantForKey(apiKey string) (string, bool) {
	t, ok := s.keys[apiKey]
	return t, ok
}

// TenantFromContext recupera el tenant inyectado por el middleware.
func TenantFromContext(ctx context.Context) (string, bool) {
	t, ok := ctx.Value(tenantCtxKey{}).(string)
	return t, ok
}

// apiKeyFromRequest extrae la key de "Authorization: Bearer <key>", de la
// cabecera X-AgentLens-Key o, como último recurso, del query param `api_key`.
// El query param existe porque los WebSocket del navegador no pueden fijar
// cabeceras; úsalo solo sobre TLS (la key puede acabar en logs/URLs).
func apiKeyFromRequest(r *http.Request) string {
	if h := r.Header.Get("Authorization"); h != "" {
		if after, ok := strings.CutPrefix(h, "Bearer "); ok {
			return strings.TrimSpace(after)
		}
	}
	if k := strings.TrimSpace(r.Header.Get("X-AgentLens-Key")); k != "" {
		return k
	}
	return strings.TrimSpace(r.URL.Query().Get("api_key"))
}

// Middleware autentica las peticiones y deja el tenant en el contexto. Las rutas
// en `public` (p.ej. /healthz, /openapi.yaml) se sirven sin autenticación.
func Middleware(next http.Handler, store KeyStore, public ...string) http.Handler {
	publicSet := make(map[string]struct{}, len(public))
	for _, p := range public {
		publicSet[p] = struct{}{}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := publicSet[r.URL.Path]; ok {
			next.ServeHTTP(w, r)
			return
		}
		key := apiKeyFromRequest(r)
		if key == "" {
			unauthorized(w, "falta la API key (Authorization: Bearer ...)")
			return
		}
		tenant, ok := store.TenantForKey(key)
		if !ok {
			unauthorized(w, "API key inválida")
			return
		}
		ctx := context.WithValue(r.Context(), tenantCtxKey{}, tenant)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func unauthorized(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("WWW-Authenticate", "Bearer")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":"` + msg + `"}`))
}
