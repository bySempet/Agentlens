// Package auth implementa la autenticación por API key del Ingestion Gateway
// (E2-T05). Cada API key identifica de forma unívoca a un tenant; el gateway
// rechaza claves inválidas y, para las válidas, inyecta el tenant resuelto en el
// contexto para que el resto del pipeline no confíe en lo que diga el cliente.
package auth

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// MetadataKey es la cabecera gRPC donde el SDK envía la API key. Coincide con
// la que emite el SDK Python (OTLPSpanExporter headers=("x-agentlens-key", ...)).
const MetadataKey = "x-agentlens-key"

type tenantCtxKey struct{}

// KeyStore resuelve una API key a su tenant. La interfaz permite sustituir el
// almacén en memoria del MVP por uno respaldado por Postgres/Redis sin tocar el
// interceptor ni el servidor.
type KeyStore interface {
	// TenantForKey devuelve el tenant asociado a la key y true si es válida.
	TenantForKey(apiKey string) (tenantID string, ok bool)
}

// StaticKeyStore es un KeyStore en memoria (key -> tenant). Apto para el MVP,
// tests y despliegues air-gapped con un set fijo de claves.
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
	tenant, ok := s.keys[apiKey]
	return tenant, ok
}

// TenantFromContext recupera el tenant inyectado por el interceptor. El segundo
// valor es false si la petición no pasó por autenticación.
func TenantFromContext(ctx context.Context) (string, bool) {
	tenant, ok := ctx.Value(tenantCtxKey{}).(string)
	return tenant, ok
}

// withTenant devuelve un contexto derivado que transporta el tenant resuelto.
func withTenant(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantCtxKey{}, tenantID)
}

// apiKeyFromContext extrae la API key de la metadata gRPC entrante.
func apiKeyFromContext(ctx context.Context) (string, bool) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", false
	}
	values := md.Get(MetadataKey)
	if len(values) == 0 || values[0] == "" {
		return "", false
	}
	return values[0], true
}

// UnaryInterceptor autentica cada RPC unario (OTLP Export es unario). Rechaza
// con codes.Unauthenticated cuando falta la key o no es válida; en caso válido
// continúa con el tenant resuelto en el contexto.
func UnaryInterceptor(store KeyStore) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		tenantID, err := authenticate(ctx, store)
		if err != nil {
			return nil, err
		}
		return handler(withTenant(ctx, tenantID), req)
	}
}

// authenticate centraliza la lógica para poder testearla de forma aislada.
func authenticate(ctx context.Context, store KeyStore) (string, error) {
	apiKey, ok := apiKeyFromContext(ctx)
	if !ok {
		return "", status.Errorf(codes.Unauthenticated, "falta la cabecera %s", MetadataKey)
	}
	tenantID, ok := store.TenantForKey(apiKey)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "API key inválida")
	}
	return tenantID, nil
}
