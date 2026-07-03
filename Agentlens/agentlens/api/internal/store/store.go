// Package store define el contrato de lectura de trazas del hot path (E3-T01).
// La interfaz aísla a los handlers HTTP del backend concreto (ClickHouse en el
// MVP) y permite un doble en tests.
package store

import "context"

// TraceSummary es una fila del listado de trazas (vista agentlens.traces).
type TraceSummary struct {
	TraceID       string `json:"trace_id"`
	AgentID       string `json:"agent_id"`
	ServiceName   string `json:"service_name"`
	RootSpanName  string `json:"root_span_name"`
	StartUnixNano int64  `json:"start_unix_nano"`
	DurationNs    int64  `json:"duration_ns"`
	SpanCount     uint64 `json:"span_count"`
	ErrorCount    uint64 `json:"error_count"`
	InputTokens   uint64 `json:"input_tokens"`
	OutputTokens  uint64 `json:"output_tokens"`
	Model         string `json:"model"`
}

// Span es un span individual del detalle de una traza.
type Span struct {
	SpanID        string            `json:"span_id"`
	ParentSpanID  string            `json:"parent_span_id"`
	Name          string            `json:"name"`
	Kind          string            `json:"kind"`
	StartUnixNano int64             `json:"start_unix_nano"`
	DurationNs    int64             `json:"duration_ns"`
	StatusCode    string            `json:"status_code"`
	StatusMessage string            `json:"status_message"`
	Attributes    map[string]string `json:"attributes"`
}

// ListQuery son los filtros del listado. El cursor (Before*) es exclusivo:
// devuelve trazas estrictamente anteriores a esa posición en el orden
// (StartUnixNano DESC, TraceID DESC).
type ListQuery struct {
	TenantID string
	AgentID  string // opcional; vacío = todos los agentes del tenant
	Limit    int
	// Posición del cursor de paginación; cero = primera página.
	BeforeStartUnixNano int64
	BeforeTraceID       string
}

// Store es el acceso de solo lectura a las trazas de un tenant.
type Store interface {
	// ListTraces devuelve hasta Limit resúmenes, más recientes primero.
	ListTraces(ctx context.Context, q ListQuery) ([]TraceSummary, error)
	// TraceSpans devuelve los spans de una traza en orden temporal. Slice
	// vacío significa que la traza no existe (o no es de ese tenant).
	TraceSpans(ctx context.Context, tenantID, traceID string) ([]Span, error)
}
