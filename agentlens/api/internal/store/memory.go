package store

import (
	"context"
	"sort"
)

// MemoryStore es un TraceStore en memoria para tests y desarrollo sin ClickHouse.
type MemoryStore struct {
	// summaries[tenant] -> trazas; spans[tenant][traceID] -> spans
	summaries map[string][]TraceSummary
	spans     map[string]map[string][]Span
}

// NewMemoryStore crea un store vacío.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		summaries: map[string][]TraceSummary{},
		spans:     map[string]map[string][]Span{},
	}
}

// AddTrace registra el resumen de una traza y sus spans para un tenant.
func (m *MemoryStore) AddTrace(tenantID string, summary TraceSummary, spans []Span) {
	m.summaries[tenantID] = append(m.summaries[tenantID], summary)
	if m.spans[tenantID] == nil {
		m.spans[tenantID] = map[string][]Span{}
	}
	m.spans[tenantID][summary.TraceID] = spans
}

// ListTraces ordena por inicio descendente y aplica limit/offset.
func (m *MemoryStore) ListTraces(_ context.Context, tenantID string, page Page) ([]TraceSummary, error) {
	all := append([]TraceSummary(nil), m.summaries[tenantID]...)
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

// GetTrace devuelve los spans de una traza del tenant (vacío si no existe).
func (m *MemoryStore) GetTrace(_ context.Context, tenantID, traceID string) ([]Span, error) {
	if byTrace, ok := m.spans[tenantID]; ok {
		return byTrace[traceID], nil
	}
	return nil, nil
}
