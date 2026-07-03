package auth

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func ctxWithKey(key string) context.Context {
	md := metadata.New(map[string]string{MetadataKey: key})
	return metadata.NewIncomingContext(context.Background(), md)
}

func newStore() KeyStore {
	return NewStaticKeyStore(map[string]Tenant{
		"key-acme":   {ID: "acme", Plan: "starter"},
		"key-globex": {ID: "globex", Plan: "pro"},
	})
}

func TestAuthenticate_ValidKeyResolvesTenant(t *testing.T) {
	tenant, err := authenticate(ctxWithKey("key-acme"), newStore())
	if err != nil {
		t.Fatalf("clave válida rechazada: %v", err)
	}
	if tenant.ID != "acme" {
		t.Fatalf("tenant esperado acme, obtenido %q", tenant.ID)
	}
	if tenant.Plan != "starter" {
		t.Fatalf("plan esperado starter, obtenido %q", tenant.Plan)
	}
}

func TestAuthenticate_InvalidKeyRejected(t *testing.T) {
	_, err := authenticate(ctxWithKey("key-desconocida"), newStore())
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("esperado Unauthenticated, obtenido %v", err)
	}
}

func TestAuthenticate_MissingKeyRejected(t *testing.T) {
	_, err := authenticate(context.Background(), newStore())
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("esperado Unauthenticated por falta de cabecera, obtenido %v", err)
	}
}

func TestAuthenticate_EmptyKeyRejected(t *testing.T) {
	_, err := authenticate(ctxWithKey(""), newStore())
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("esperado Unauthenticated por cabecera vacía, obtenido %v", err)
	}
}

func TestUnaryInterceptor_InjectsTenantForValidKey(t *testing.T) {
	var gotTenant Tenant
	var handlerCalled bool
	handler := func(ctx context.Context, req any) (any, error) {
		handlerCalled = true
		gotTenant, _ = TenantInfoFromContext(ctx)
		return "ok", nil
	}
	interceptor := UnaryInterceptor(newStore())
	resp, err := interceptor(ctxWithKey("key-globex"), nil, nil, handler)
	if err != nil {
		t.Fatalf("interceptor falló con clave válida: %v", err)
	}
	if !handlerCalled {
		t.Fatal("el handler no se invocó para una clave válida")
	}
	if resp != "ok" {
		t.Fatalf("respuesta inesperada: %v", resp)
	}
	if gotTenant.ID != "globex" {
		t.Fatalf("tenant en contexto esperado globex, obtenido %q", gotTenant.ID)
	}
	if gotTenant.Plan != "pro" {
		t.Fatalf("plan en contexto esperado pro, obtenido %q", gotTenant.Plan)
	}
}

func TestUnaryInterceptor_BlocksInvalidKey(t *testing.T) {
	handlerCalled := false
	handler := func(ctx context.Context, req any) (any, error) {
		handlerCalled = true
		return nil, nil
	}
	interceptor := UnaryInterceptor(newStore())
	_, err := interceptor(ctxWithKey("nope"), nil, nil, handler)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("esperado Unauthenticated, obtenido %v", err)
	}
	if handlerCalled {
		t.Fatal("el handler NO debe invocarse con clave inválida")
	}
}
