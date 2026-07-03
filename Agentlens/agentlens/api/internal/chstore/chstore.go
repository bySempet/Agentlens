// Package chstore implementa store.Store sobre la interfaz HTTP de ClickHouse
// (E3-T01). Usa parámetros tipados del servidor ({name:Type} + param_name), de
// modo que ningún valor del cliente se interpola en el SQL (sin inyección), y
// FORMAT JSON para decodificar con la stdlib: el hot path no necesita driver.
package chstore

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/bysempet/agentlens/api/internal/store"
)

// Client habla con un servidor ClickHouse por HTTP.
type Client struct {
	baseURL  string
	user     string
	password string
	http     *http.Client
}

// New crea el cliente. baseURL es el endpoint HTTP de ClickHouse
// (p. ej. http://localhost:8123).
func New(baseURL, user, password string) *Client {
	return &Client{
		baseURL:  strings.TrimRight(baseURL, "/"),
		user:     user,
		password: password,
		http:     &http.Client{},
	}
}

// query ejecuta el SQL con los parámetros dados y decodifica el campo `data`
// del FORMAT JSON de ClickHouse sobre `out` (puntero a slice de structs).
func (c *Client) query(ctx context.Context, sql string, params map[string]string, out any) error {
	q := url.Values{}
	// Los enteros de 64 bits llegan como números JSON, no como strings.
	q.Set("output_format_json_quote_64bit_integers", "0")
	q.Set("default_format", "JSON")
	for name, value := range params {
		q.Set("param_"+name, value)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/?"+q.Encode(), strings.NewReader(sql))
	if err != nil {
		return err
	}
	if c.user != "" {
		req.Header.Set("X-ClickHouse-User", c.user)
		req.Header.Set("X-ClickHouse-Key", c.password)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("clickhouse no accesible: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("clickhouse HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return fmt.Errorf("respuesta de clickhouse ilegible: %w", err)
	}
	return json.Unmarshal(envelope.Data, out)
}

const listTracesSQL = `
SELECT TraceId                             AS trace_id,
       AgentId                             AS agent_id,
       ServiceName                         AS service_name,
       RootSpanName                        AS root_span_name,
       toUnixTimestamp64Nano(StartTime)    AS start_unix_nano,
       DurationNs                          AS duration_ns,
       SpanCount                           AS span_count,
       ErrorCount                          AS error_count,
       InputTokens                         AS input_tokens,
       OutputTokens                        AS output_tokens,
       Model                               AS model
FROM agentlens.traces
WHERE TenantId = {tenant:String}
`

// ListTraces implementa store.Store.
func (c *Client) ListTraces(ctx context.Context, q store.ListQuery) ([]store.TraceSummary, error) {
	sql := listTracesSQL
	params := map[string]string{
		"tenant": q.TenantID,
		"limit":  strconv.Itoa(q.Limit),
	}
	if q.AgentID != "" {
		sql += "  AND AgentId = {agent:String}\n"
		params["agent"] = q.AgentID
	}
	if q.BeforeTraceID != "" {
		sql += "  AND (toUnixTimestamp64Nano(StartTime), TraceId) < ({before_ts:Int64}, {before_id:String})\n"
		params["before_ts"] = strconv.FormatInt(q.BeforeStartUnixNano, 10)
		params["before_id"] = q.BeforeTraceID
	}
	sql += "ORDER BY start_unix_nano DESC, trace_id DESC\nLIMIT {limit:UInt32}"

	out := []store.TraceSummary{}
	if err := c.query(ctx, sql, params, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// traceWindowSQL acota la ventana temporal de la traza vía el índice
// otel_traces_trace_id_ts, para que el detalle pode particiones por Timestamp
// en vez de escanear toda la tabla.
const traceWindowSQL = `
SELECT toUnixTimestamp64Nano(min(Start)) AS s,
       toUnixTimestamp64Nano(max(End))   AS e
FROM agentlens.otel_traces_trace_id_ts
WHERE TraceId = {trace_id:String}
`

const traceSpansSQL = `
SELECT SpanId                              AS span_id,
       ParentSpanId                        AS parent_span_id,
       SpanName                            AS name,
       SpanKind                            AS kind,
       toUnixTimestamp64Nano(Timestamp)    AS start_unix_nano,
       Duration                            AS duration_ns,
       StatusCode                          AS status_code,
       StatusMessage                       AS status_message,
       SpanAttributes                      AS attributes
FROM agentlens.otel_traces
WHERE TenantId = {tenant:String}
  AND TraceId  = {trace_id:String}
  AND Timestamp >= fromUnixTimestamp64Nano({win_s:Int64})
  AND Timestamp <= fromUnixTimestamp64Nano({win_e:Int64})
ORDER BY start_unix_nano
`

// TraceSpans implementa store.Store.
func (c *Client) TraceSpans(ctx context.Context, tenantID, traceID string) ([]store.Span, error) {
	var window []struct {
		S int64 `json:"s"`
		E int64 `json:"e"`
	}
	if err := c.query(ctx, traceWindowSQL, map[string]string{"trace_id": traceID}, &window); err != nil {
		return nil, err
	}
	// min/max sobre cero filas devuelven la época (0): traza desconocida.
	if len(window) == 0 || window[0].E == 0 {
		return []store.Span{}, nil
	}

	out := []store.Span{}
	err := c.query(ctx, traceSpansSQL, map[string]string{
		"tenant":   tenantID,
		"trace_id": traceID,
		"win_s":    strconv.FormatInt(window[0].S, 10),
		"win_e":    strconv.FormatInt(window[0].E, 10),
	}, &out)
	if err != nil {
		return nil, err
	}
	return out, nil
}
