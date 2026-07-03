package policy

import (
	"context"
	"testing"
	"time"
)

func newEngine(t testing.TB) *Engine {
	t.Helper()
	e, err := New(context.Background(), DefaultConfig())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return e
}

func TestEvaluate_AllowsCleanAction(t *testing.T) {
	e := newEngine(t)
	d, err := e.Evaluate(context.Background(), Input{
		Tenant: "acme", Agent: "support-bot", Operation: "chat",
		Tool: "lookup_customer", Model: "gpt-4o",
		Destination: "internal", OutputTokens: 200,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !d.Allow || len(d.Denials) != 0 {
		t.Fatalf("acción limpia debería permitirse, %+v", d)
	}
}

func TestEvaluate_BlocksDangerousTool(t *testing.T) {
	e := newEngine(t)
	d, err := e.Evaluate(context.Background(), Input{Tenant: "acme", Tool: "shell.exec"})
	if err != nil {
		t.Fatal(err)
	}
	if d.Allow {
		t.Fatal("shell.exec debería bloquearse")
	}
	if len(d.Denials) == 0 {
		t.Fatal("debería haber un motivo de denegación")
	}
}

func TestEvaluate_BlocksPIIToExternal(t *testing.T) {
	e := newEngine(t)
	d, _ := e.Evaluate(context.Background(), Input{
		Tenant: "acme", PIIDetected: true, Destination: "external",
	})
	if d.Allow {
		t.Fatalf("PII a destino externo debería bloquearse, %+v", d)
	}
}

func TestEvaluate_PIIToInternalAllowed(t *testing.T) {
	e := newEngine(t)
	d, _ := e.Evaluate(context.Background(), Input{
		Tenant: "acme", PIIDetected: true, Destination: "internal", OutputTokens: 10,
	})
	if !d.Allow {
		t.Fatalf("PII interna debería permitirse, %+v", d)
	}
}

func TestEvaluate_BlocksOverTokenLimit(t *testing.T) {
	e := newEngine(t)
	d, _ := e.Evaluate(context.Background(), Input{Tenant: "acme", OutputTokens: 200_000})
	if d.Allow {
		t.Fatalf("por encima del límite debería bloquearse, %+v", d)
	}
}

func TestEvaluate_MultipleDenials(t *testing.T) {
	e := newEngine(t)
	d, _ := e.Evaluate(context.Background(), Input{
		Tenant: "acme", Tool: "shell.exec", PIIDetected: true,
		Destination: "external", OutputTokens: 500_000,
	})
	if d.Allow || len(d.Denials) < 3 {
		t.Fatalf("deberían acumularse 3 denegaciones, %+v", d)
	}
}

// Criterio E4-T01: evaluación de política < 5 ms (tras compilar).
func TestEvaluate_LatencyUnder5ms(t *testing.T) {
	e := newEngine(t)
	in := Input{Tenant: "acme", Tool: "lookup_customer", OutputTokens: 100}
	// Calentamiento.
	_, _ = e.Evaluate(context.Background(), in)

	const n = 200
	start := time.Now()
	for i := 0; i < n; i++ {
		if _, err := e.Evaluate(context.Background(), in); err != nil {
			t.Fatal(err)
		}
	}
	avg := time.Since(start) / n
	t.Logf("latencia media de evaluación: %v", avg)
	if avg > 5*time.Millisecond {
		t.Fatalf("evaluación demasiado lenta: %v (> 5ms)", avg)
	}
}

func BenchmarkEvaluate(b *testing.B) {
	e := newEngine(b)
	in := Input{Tenant: "acme", Tool: "lookup_customer", OutputTokens: 100}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = e.Evaluate(context.Background(), in)
	}
}
