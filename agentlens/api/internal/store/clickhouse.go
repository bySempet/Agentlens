package store

import (
	"context"
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// Las consultas reflejan EXACTAMENTE las verificadas con ClickHouse real en
// deploy/clickhouse/verify_schema.py. Mantenerlas en sync con ese script.
const (
	listTracesSQL = `
SELECT
    TraceId,
    argMinMerge(RootSpanName)                       AS root_span_name,
    anyMerge(ServiceName)                           AS service_name,
    anyMerge(AgentId)                               AS agent_id,
    fromUnixTimestamp64Nano(minMerge(StartNs))      AS start_time,
    (maxMerge(EndNs) - minMerge(StartNs)) / 1e6     AS duration_ms,
    countMerge(SpanCount)                           AS span_count,
    sumMerge(ErrorCount)                            AS error_count,
    sumMerge(InputTokens)                           AS input_tokens,
    sumMerge(OutputTokens)                          AS output_tokens
FROM agentlens.trace_summary
WHERE TenantId = ?
GROUP BY TraceId
ORDER BY minMerge(StartNs) DESC
LIMIT ? OFFSET ?`

	getTraceSQL = `
SELECT
    SpanId,
    ParentSpanId,
    SpanName,
    GenAIOperation,
    RequestModel,
    Timestamp        AS start_time,
    Duration / 1e6   AS duration_ms,
    StatusCode,
    StatusMessage
FROM agentlens.otel_traces
WHERE TenantId = ? AND TraceId = ?
ORDER BY Timestamp`
)

// ClickHouseStore implementa TraceStore sobre ClickHouse.
type ClickHouseStore struct {
	conn driver.Conn
}

// NewClickHouseStore abre una conexión nativa a ClickHouse.
func NewClickHouseStore(addr, database, username, password string) (*ClickHouseStore, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{addr},
		Auth: clickhouse.Auth{Database: database, Username: username, Password: password},
	})
	if err != nil {
		return nil, fmt.Errorf("abriendo ClickHouse: %w", err)
	}
	return &ClickHouseStore{conn: conn}, nil
}

// ListTraces consulta la vista de resumen, paginada.
func (s *ClickHouseStore) ListTraces(ctx context.Context, tenantID string, page Page) ([]TraceSummary, error) {
	rows, err := s.conn.Query(ctx, listTracesSQL, tenantID, page.Limit, page.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []TraceSummary{}
	for rows.Next() {
		var t TraceSummary
		if err := rows.Scan(
			&t.TraceID, &t.RootSpanName, &t.ServiceName, &t.AgentID,
			&t.StartTime, &t.DurationMs, &t.SpanCount, &t.ErrorCount,
			&t.InputTokens, &t.OutputTokens,
		); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// GetTrace consulta los spans crudos de una traza.
func (s *ClickHouseStore) GetTrace(ctx context.Context, tenantID, traceID string) ([]Span, error) {
	rows, err := s.conn.Query(ctx, getTraceSQL, tenantID, traceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Span{}
	for rows.Next() {
		var sp Span
		if err := rows.Scan(
			&sp.SpanID, &sp.ParentSpanID, &sp.SpanName, &sp.GenAIOperation,
			&sp.RequestModel, &sp.StartTime, &sp.DurationMs,
			&sp.StatusCode, &sp.StatusMessage,
		); err != nil {
			return nil, err
		}
		out = append(out, sp)
	}
	return out, rows.Err()
}

// Close cierra la conexión.
func (s *ClickHouseStore) Close() error { return s.conn.Close() }
