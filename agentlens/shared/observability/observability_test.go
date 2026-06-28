package observability

import (
	"context"
	"testing"
)

func TestInit_NoEndpointIsNoop(t *testing.T) {
	t.Setenv("AGENTLENS_OTLP_ENDPOINT", "")
	shutdown, err := Init(context.Background(), "test-svc")
	if err != nil {
		t.Fatalf("Init sin endpoint no debería fallar: %v", err)
	}
	if shutdown == nil {
		t.Fatal("shutdown no debería ser nil")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown no-op no debería fallar: %v", err)
	}
}
