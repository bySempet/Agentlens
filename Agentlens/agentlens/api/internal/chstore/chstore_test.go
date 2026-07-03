package chstore

import (
	"context"
	"os"
	"testing"

	"github.com/bysempet/agentlens/api/internal/store"
)

// Test de integración contra un ClickHouse real con el esquema E2-T07 aplicado.
// Se activa con AGENTLENS_TEST_CLICKHOUSE (p. ej. http://127.0.0.1:8123); las
// credenciales van en AGENTLENS_TEST_CLICKHOUSE_USER/_PASSWORD (def. vacías).
func integrationClient(t *testing.T) *Client {
	t.Helper()
	base := os.Getenv("AGENTLENS_TEST_CLICKHOUSE")
	if base == "" {
		t.Skip("AGENTLENS_TEST_CLICKHOUSE no definido; test de integración omitido")
	}
	return New(base, os.Getenv("AGENTLENS_TEST_CLICKHOUSE_USER"), os.Getenv("AGENTLENS_TEST_CLICKHOUSE_PASSWORD"))
}

func TestIntegration_ListAndDetail(t *testing.T) {
	c := integrationClient(t)
	ctx := context.Background()

	traces, err := c.ListTraces(ctx, store.ListQuery{TenantID: "acme-corp", Limit: 10})
	if err != nil {
		t.Fatalf("ListTraces: %v", err)
	}
	if len(traces) == 0 {
		t.Skip("sin trazas del tenant acme-corp en este ClickHouse; nada que verificar")
	}

	// Orden descendente por inicio.
	for i := 1; i < len(traces); i++ {
		if traces[i].StartUnixNano > traces[i-1].StartUnixNano {
			t.Fatalf("listado sin ordenar: pos %d (%d) > pos %d (%d)",
				i, traces[i].StartUnixNano, i-1, traces[i-1].StartUnixNano)
		}
	}

	// Detalle de la primera traza: debe tener spans y pertenecer al tenant.
	spans, err := c.TraceSpans(ctx, "acme-corp", traces[0].TraceID)
	if err != nil {
		t.Fatalf("TraceSpans: %v", err)
	}
	if len(spans) == 0 {
		t.Fatalf("la traza %s aparece en el listado pero no tiene spans", traces[0].TraceID)
	}
	if int(traces[0].SpanCount) != len(spans) {
		t.Fatalf("span_count del resumen (%d) != spans del detalle (%d)", traces[0].SpanCount, len(spans))
	}

	// Una traza existente consultada con otro tenant no debe devolver nada.
	ajenos, err := c.TraceSpans(ctx, "tenant-inexistente", traces[0].TraceID)
	if err != nil {
		t.Fatalf("TraceSpans (tenant ajeno): %v", err)
	}
	if len(ajenos) != 0 {
		t.Fatalf("una traza no debe ser visible desde otro tenant; obtenidos %d spans", len(ajenos))
	}
}

func TestIntegration_CursorPagination(t *testing.T) {
	c := integrationClient(t)
	ctx := context.Background()

	all, err := c.ListTraces(ctx, store.ListQuery{TenantID: "acme-corp", Limit: 100})
	if err != nil {
		t.Fatalf("ListTraces: %v", err)
	}
	if len(all) < 2 {
		t.Skip("hacen falta >= 2 trazas para verificar la paginación")
	}

	first, err := c.ListTraces(ctx, store.ListQuery{TenantID: "acme-corp", Limit: 1})
	if err != nil {
		t.Fatalf("ListTraces (página 1): %v", err)
	}
	rest, err := c.ListTraces(ctx, store.ListQuery{
		TenantID:            "acme-corp",
		Limit:               100,
		BeforeStartUnixNano: first[0].StartUnixNano,
		BeforeTraceID:       first[0].TraceID,
	})
	if err != nil {
		t.Fatalf("ListTraces (página 2): %v", err)
	}
	if len(rest) != len(all)-1 {
		t.Fatalf("tras el cursor deben quedar %d trazas, obtenidas %d", len(all)-1, len(rest))
	}
	for _, tr := range rest {
		if tr.TraceID == first[0].TraceID {
			t.Fatalf("la traza del cursor (%s) no debe repetirse en la página siguiente", tr.TraceID)
		}
	}
}
