-- E2-T07 · Índice traza -> ventana temporal (v1).
--
-- Dado un TraceId, acota en qué rango de tiempo (y por tanto en qué particiones)
-- viven sus spans, para que el detalle de traza no escanee toda la tabla.
-- Mismo diseño que genera el exporter con create_schema: true; lo declaramos
-- explícito porque el esquema ahora es responsabilidad nuestra.

CREATE TABLE IF NOT EXISTS agentlens.otel_traces_trace_id_ts
(
    TraceId String CODEC (ZSTD(1)),
    Start   DateTime64(9) CODEC (Delta, ZSTD(1)),
    End     DateTime64(9) CODEC (Delta, ZSTD(1)),
    INDEX idx_trace_id TraceId TYPE bloom_filter(0.01) GRANULARITY 1
)
ENGINE = MergeTree
TTL toDateTime(Start) + toIntervalDay(30)
ORDER BY (TraceId, toUnixTimestamp(Start))
SETTINGS index_granularity = 8192;

CREATE MATERIALIZED VIEW IF NOT EXISTS agentlens.otel_traces_trace_id_ts_mv
    TO agentlens.otel_traces_trace_id_ts
AS
SELECT TraceId,
       min(Timestamp) AS Start,
       max(Timestamp) AS End
FROM agentlens.otel_traces
WHERE TraceId != ''
GROUP BY TraceId;
