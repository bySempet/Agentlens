package store

import (
	"context"
	"testing"
	"time"
)

func ts(min int) time.Time {
	return time.Date(2026, 6, 25, 10, min, 0, 0, time.UTC)
}

func seeded() *MemoryStore {
	m := NewMemoryStore()
	for i, id := range []string{"A", "B", "C"} {
		m.AddTrace("acme", TraceSummary{TraceID: id, StartTime: ts(i)}, []Span{{SpanID: "s-" + id}})
	}
	m.AddTrace("globex", TraceSummary{TraceID: "G", StartTime: ts(0)}, nil)
	return m
}

func TestMemory_ListTracesOrderedDescAndPaginated(t *testing.T) {
	m := seeded()
	got, _ := m.ListTraces(context.Background(), "acme", Page{Limit: 2, Offset: 0})
	if len(got) != 2 || got[0].TraceID != "C" || got[1].TraceID != "B" {
		t.Fatalf("orden/paginación incorrectos: %+v", got)
	}
	page2, _ := m.ListTraces(context.Background(), "acme", Page{Limit: 2, Offset: 2})
	if len(page2) != 1 || page2[0].TraceID != "A" {
		t.Fatalf("segunda página incorrecta: %+v", page2)
	}
}

func TestMemory_TenantIsolation(t *testing.T) {
	m := seeded()
	got, _ := m.ListTraces(context.Background(), "acme", Page{Limit: 50})
	if len(got) != 3 {
		t.Fatalf("acme debería tener 3 trazas, %d", len(got))
	}
	for _, tr := range got {
		if tr.TraceID == "G" {
			t.Fatal("se filtró una traza de globex en acme")
		}
	}
}

func TestMemory_ListTracesSince(t *testing.T) {
	m := seeded()
	got, _ := m.ListTracesSince(context.Background(), "acme", ts(0), 50)
	// Estrictamente posteriores a ts(0): B y C, en orden ascendente.
	if len(got) != 2 || got[0].TraceID != "B" || got[1].TraceID != "C" {
		t.Fatalf("ListTracesSince incorrecto: %+v", got)
	}
}

func TestMemory_GetTrace(t *testing.T) {
	m := seeded()
	spans, _ := m.GetTrace(context.Background(), "acme", "A")
	if len(spans) != 1 || spans[0].SpanID != "s-A" {
		t.Fatalf("GetTrace incorrecto: %+v", spans)
	}
	none, _ := m.GetTrace(context.Background(), "acme", "noexiste")
	if len(none) != 0 {
		t.Fatalf("traza inexistente debería devolver vacío, %+v", none)
	}
}

func TestMemory_CostRows(t *testing.T) {
	m := NewMemoryStore()
	m.AddCostRow("acme", CostRow{AgentID: "a", Model: "gpt-4o", InputTokens: 10, OutputTokens: 5})
	rows, _ := m.CostRows(context.Background(), "acme")
	if len(rows) != 1 || rows[0].InputTokens != 10 {
		t.Fatalf("CostRows incorrecto: %+v", rows)
	}
}
