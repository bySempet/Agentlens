// Package keystore resuelve una API key a su tenant. Es compartido por el
// Ingestion Gateway (gRPC) y la API de lectura (HTTP) para evitar divergencia en
// la lógica de autenticación por clave.
package keystore

import (
	"fmt"
	"strings"
)

// KeyStore resuelve una API key a su tenant.
type KeyStore interface {
	// TenantForKey devuelve el tenant asociado a la key y true si es válida.
	TenantForKey(apiKey string) (tenantID string, ok bool)
}

// StaticKeyStore es un KeyStore en memoria (key -> tenant). Apto para el MVP,
// tests y despliegues air-gapped con un set fijo de claves. Sustituible por uno
// respaldado por Postgres/Redis sin tocar a los consumidores.
type StaticKeyStore struct {
	keys map[string]string
}

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

// ParseSpec interpreta "key1:tenantA,key2:tenantB" como mapa key -> tenant.
// Devuelve error si algún par está malformado.
func ParseSpec(raw string) (map[string]string, error) {
	out := map[string]string{}
	for _, pair := range strings.Split(raw, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		k, t, ok := strings.Cut(pair, ":")
		if !ok || k == "" || t == "" {
			return nil, fmt.Errorf("par API key inválido (esperado key:tenant): %q", pair)
		}
		out[k] = t
	}
	return out, nil
}
