package store

import (
	"context"
	"sort"
	"sync"
	"time"
)

// MemoryStore es un TraceStore en memoria para tests y desarrollo sin ClickHouse.
// Es seguro para uso concurrente (el live feed lo lee mientras los tests escriben).
type MemoryStore struct {
	mu sync.RWMutex
	// summaries[tenant] -> trazas; spans[tenant][traceID] -> spans
	summaries map[string][]TraceSummary
	spans     map[string]map[string][]Span
	costs     map[string][]CostRow
}

// NewMemoryStore crea un store vacío.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		summaries: map[string][]TraceSummary{},
		spans:     map[string]map[string][]Span{},
		costs:     map[string][]CostRow{},
	}
}

// AddTrace registra el resumen de una traza y sus spans para un tenant.
func (m *MemoryStore) AddTrace(tenantID string, summary TraceSummary, spans []Span) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.summaries[tenantID] = append(m.summaries[tenantID], summary)
	if m.spans[tenantID] == nil {
		m.spans[tenantID] = map[string][]Span{}
	}
	m.spans[tenantID][summary.TraceID] = spans
}

// AddCostRow registra un agregado de coste para un tenant.
func (m *MemoryStore) AddCostRow(tenantID string, row CostRow) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.costs[tenantID] = append(m.costs[tenantID], row)
}

// ListTraces ordena por inicio descendente y aplica limit/offset.
func (m *MemoryStore) ListTraces(_ context.Context, tenantID string, page Page) ([]TraceSummary, error) {
	m.mu.RLock()
	all := append([]TraceSummary(nil), m.summaries[tenantID]...)
	m.mu.RUnlock()

	sort.Slice(all, func(i, j int) bool { return all[i].StartTime.After(all[j].StartTime) })
	if page.Offset >= len(all) {
		return []TraceSummary{}, nil
	}
	end := page.Offset + page.Limit
	if end > len(all) {
		end = len(all)
	}
	return all[page.Offset:end], nil
}

// ListTracesSince devuelve las trazas iniciadas tras `since`, en orden ascendente.
func (m *MemoryStore) ListTracesSince(_ context.Context, tenantID string, since time.Time, limit int) ([]TraceSummary, error) {
	m.mu.RLock()
	all := append([]TraceSummary(nil), m.summaries[tenantID]...)
	m.mu.RUnlock()

	out := []TraceSummary{}
	for _, t := range all {
		if t.StartTime.After(since) {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartTime.Before(out[j].StartTime) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// GetTrace devuelve los spans de una traza del tenant (vacío si no existe).
func (m *MemoryStore) GetTrace(_ context.Context, tenantID, traceID string) ([]Span, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if byTrace, ok := m.spans[tenantID]; ok {
		return byTrace[traceID], nil
	}
	return nil, nil
}

// CostRows devuelve los agregados de coste del tenant.
func (m *MemoryStore) CostRows(_ context.Context, tenantID string) ([]CostRow, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]CostRow(nil), m.costs[tenantID]...), nil
}
