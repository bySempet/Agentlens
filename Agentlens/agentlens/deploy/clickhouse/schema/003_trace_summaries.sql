-- E2-T07 · Resumen por traza para el listado del dashboard (v1).
--
-- El listado de trazas (E3-T01) no puede agregar sobre agentlens.otel_traces en
-- cada petición. Esta vista materializada mantiene un resumen incremental por
-- (TenantId, TraceId) con AggregatingMergeTree: cada inserción del Collector
-- aporta parciales y los merges los combinan.
--
-- Consultar SIEMPRE a través de la vista agentlens.traces (abajo), que
-- re-agrega: los parciales de un mismo TraceId pueden no estar fusionados aún.

CREATE TABLE IF NOT EXISTS agentlens.trace_summaries
(
    TenantId     LowCardinality(String),
    TraceId      String,
    AgentId      SimpleAggregateFunction(anyLast, String),
    ServiceName  SimpleAggregateFunction(anyLast, String),
    -- max() con '' para los bloques sin span raíz: cualquier nombre real gana.
    RootSpanName SimpleAggregateFunction(max, String),
    Start        SimpleAggregateFunction(min, DateTime64(9)),
    End          SimpleAggregateFunction(max, DateTime64(9)),
    SpanCount    SimpleAggregateFunction(sum, UInt64),
    ErrorCount   SimpleAggregateFunction(sum, UInt64),
    InputTokens  SimpleAggregateFunction(sum, UInt64),
    OutputTokens SimpleAggregateFunction(sum, UInt64),
    Model        SimpleAggregateFunction(max, String)
)
ENGINE = AggregatingMergeTree
TTL toDateTime(Start) + toIntervalDay(30)
ORDER BY (TenantId, TraceId)
SETTINGS index_granularity = 8192;

CREATE MATERIALIZED VIEW IF NOT EXISTS agentlens.trace_summaries_mv
    TO agentlens.trace_summaries
AS
SELECT TenantId,
       TraceId,
       toString(anyLast(AgentId))                                     AS AgentId,
       toString(anyLast(ServiceName))                                 AS ServiceName,
       maxIf(toString(SpanName), ParentSpanId = '')                   AS RootSpanName,
       min(Timestamp)                                                 AS Start,
       max(Timestamp)                                                 AS End,
       count()                                                        AS SpanCount,
       countIf(StatusCode = 'Error')                                  AS ErrorCount,
       sum(toUInt64OrZero(SpanAttributes['gen_ai.usage.input_tokens']))  AS InputTokens,
       sum(toUInt64OrZero(SpanAttributes['gen_ai.usage.output_tokens'])) AS OutputTokens,
       max(SpanAttributes['gen_ai.request.model'])                    AS Model
FROM agentlens.otel_traces
WHERE TraceId != ''
GROUP BY TenantId, TraceId;

-- Interfaz de consulta estable para la API (E3-T01): re-agrega los parciales.
CREATE VIEW IF NOT EXISTS agentlens.traces AS
SELECT TenantId,
       TraceId,
       anyLast(AgentId)                       AS AgentId,
       anyLast(ServiceName)                   AS ServiceName,
       max(RootSpanName)                      AS RootSpanName,
       -- StartTime/EndTime (y no Start/End) para no colisionar con las columnas
       -- fuente al calcular DurationNs (ILLEGAL_AGGREGATION con el alias igual).
       min(Start)                             AS StartTime,
       max(End)                               AS EndTime,
       toUnixTimestamp64Nano(EndTime) - toUnixTimestamp64Nano(StartTime) AS DurationNs,
       sum(SpanCount)                         AS SpanCount,
       sum(ErrorCount)                        AS ErrorCount,
       sum(InputTokens)                       AS InputTokens,
       sum(OutputTokens)                      AS OutputTokens,
       max(Model)                             AS Model
FROM agentlens.trace_summaries
GROUP BY TenantId, TraceId;
