// Package store define el acceso de lectura a trazas (E3-T01). La interfaz
// TraceStore desacopla la API HTTP del backend concreto (ClickHouse en prod,
// fake en memoria en tests).
package store

import (
	"context"
	"time"
)

// TraceSummary es una fila de la lista de trazas del dashboard. Procede de la
// vista materializada agentlens.trace_summary (resumen por traza).
type TraceSummary struct {
	TraceID      string    `json:"trace_id"`
	RootSpanName string    `json:"root_span_name"`
	ServiceName  string    `json:"service_name"`
	AgentID      string    `json:"agent_id"`
	StartTime    time.Time `json:"start_time"`
	DurationMs   float64   `json:"duration_ms"`
	SpanCount    uint64    `json:"span_count"`
	ErrorCount   uint64    `json:"error_count"`
	InputTokens  uint64    `json:"input_tokens"`
	OutputTokens uint64    `json:"output_tokens"`
}

// Span es un span dentro del detalle de una traza (timeline + conversación).
type Span struct {
	SpanID         string    `json:"span_id"`
	ParentSpanID   string    `json:"parent_span_id"`
	SpanName       string    `json:"span_name"`
	GenAIOperation string    `json:"genai_operation"`
	RequestModel   string    `json:"request_model"`
	StartTime      time.Time `json:"start_time"`
	DurationMs     float64   `json:"duration_ms"`
	StatusCode     string    `json:"status_code"`
	StatusMessage  string    `json:"status_message"`

	// Contenido GenAI para la vista de conversación (E3-T04). Pueden venir
	// vacíos, redactados o como referencia (agentlens://payload/...) según la
	// política de privacidad del SDK.
	SystemInstructions string `json:"system_instructions"`
	InputMessages      string `json:"input_messages"`
	OutputMessages     string `json:"output_messages"`
	ToolArguments      string `json:"tool_arguments"`
	ToolResult         string `json:"tool_result"`
}

// Page describe una petición de paginación ya validada.
type Page struct {
	Limit  int
	Offset int
}

// CostRow agrega tokens por (agente, modelo) para el cálculo de coste (E3-T06).
type CostRow struct {
	AgentID      string `json:"agent_id"`
	Model        string `json:"model"`
	InputTokens  uint64 `json:"input_tokens"`
	OutputTokens uint64 `json:"output_tokens"`
}

// TraceStore es el contrato de lectura sobre el almacén de trazas.
type TraceStore interface {
	// ListTraces devuelve el resumen de las trazas de un tenant, paginado y
	// ordenado por inicio descendente (más recientes primero).
	ListTraces(ctx context.Context, tenantID string, page Page) ([]TraceSummary, error)
	// GetTrace devuelve los spans de una traza concreta de un tenant, ordenados
	// por tiempo. Lista vacía si la traza no existe para ese tenant.
	GetTrace(ctx context.Context, tenantID, traceID string) ([]Span, error)
	// ListTracesSince devuelve las trazas de un tenant que comenzaron después de
	// `since`, en orden ascendente (para el live feed, E3-T05).
	ListTracesSince(ctx context.Context, tenantID string, since time.Time, limit int) ([]TraceSummary, error)
	// CostRows agrega el uso de tokens por agente y modelo de un tenant.
	CostRows(ctx context.Context, tenantID string) ([]CostRow, error)
}
