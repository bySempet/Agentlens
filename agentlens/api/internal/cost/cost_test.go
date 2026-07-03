package cost

import (
	"math"
	"testing"

	"github.com/bysempet/agentlens/api/internal/store"
)

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestUSD_KnownModel(t *testing.T) {
	// gpt-4o: 2.5/M in, 10/M out. 1M in + 1M out = 2.5 + 10 = 12.5
	got := USD("gpt-4o", 1_000_000, 1_000_000)
	if !approx(got, 12.5) {
		t.Fatalf("coste gpt-4o = %v, esperado 12.5", got)
	}
}

func TestUSD_UnknownModelUsesDefault(t *testing.T) {
	// default: 1/M in, 3/M out. 1M+1M = 4
	got := USD("modelo-raro", 1_000_000, 1_000_000)
	if !approx(got, 4) {
		t.Fatalf("coste por defecto = %v, esperado 4", got)
	}
}

func TestSummarize_RollupsAndTotals(t *testing.T) {
	rows := []store.CostRow{
		{AgentID: "a1", Model: "gpt-4o", InputTokens: 1_000_000, OutputTokens: 0},          // 2.5
		{AgentID: "a1", Model: "gpt-4o-mini", InputTokens: 0, OutputTokens: 1_000_000},     // 0.6
		{AgentID: "a2", Model: "gpt-4o", InputTokens: 0, OutputTokens: 1_000_000},          // 10
	}
	s := Summarize(rows)

	if !approx(s.TotalCostUSD, 13.1) {
		t.Fatalf("total = %v, esperado 13.1", s.TotalCostUSD)
	}
	if s.TotalInputTokens != 1_000_000 || s.TotalOutputTokens != 2_000_000 {
		t.Fatalf("totales de tokens incorrectos: %+v", s)
	}

	// Orden por coste desc: por agente a1 (3.1) vs a2 (10) -> a2 primero.
	if s.ByAgent[0].AgentID != "a2" {
		t.Fatalf("esperado a2 primero por coste, %+v", s.ByAgent)
	}
	// Por modelo: gpt-4o (12.5) vs gpt-4o-mini (0.6) -> gpt-4o primero.
	if s.ByModel[0].Model != "gpt-4o" || !approx(s.ByModel[0].CostUSD, 12.5) {
		t.Fatalf("rollup por modelo incorrecto: %+v", s.ByModel)
	}
}

func TestSummarize_Empty(t *testing.T) {
	s := Summarize(nil)
	if s.TotalCostUSD != 0 || len(s.ByAgent) != 0 || len(s.ByModel) != 0 {
		t.Fatalf("resumen vacío no nulo: %+v", s)
	}
}
