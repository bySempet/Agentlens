-- Esquema ClickHouse explícito de AgentLens (E2-T07).
--
-- AgentLens posee el esquema (el exporter del Collector se configura con
-- create_schema: false). Así está versionado, optimizado para consultas
-- multi-tenant sub-segundo y enriquecido con columnas materializadas y una
-- vista de resumen de trazas para el dashboard.
--
-- `otel_traces` mantiene el CONTRATO DE COLUMNAS del exporter ClickHouse de
-- opentelemetry-collector-contrib (v0.110): el exporter inserta por nombre de
-- columna, así que basta con que existan esas columnas con tipos compatibles.
-- Las columnas MATERIALIZED extra las calcula ClickHouse y el exporter las
-- ignora.

CREATE DATABASE IF NOT EXISTS agentlens;

-- ---------------------------------------------------------------------------
-- Spans crudos
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS agentlens.otel_traces
(
    -- Columnas del contrato del exporter OTel.
    Timestamp          DateTime64(9) CODEC(Delta(8), ZSTD(1)),
    TraceId            String CODEC(ZSTD(1)),
    SpanId             String CODEC(ZSTD(1)),
    ParentSpanId       String CODEC(ZSTD(1)),
    TraceState         String CODEC(ZSTD(1)),
    SpanName           LowCardinality(String) CODEC(ZSTD(1)),
    SpanKind           LowCardinality(String) CODEC(ZSTD(1)),
    ServiceName        LowCardinality(String) CODEC(ZSTD(1)),
    ResourceAttributes Map(LowCardinality(String), String) CODEC(ZSTD(1)),
    ScopeName          String CODEC(ZSTD(1)),
    ScopeVersion       String CODEC(ZSTD(1)),
    SpanAttributes     Map(LowCardinality(String), String) CODEC(ZSTD(1)),
    Duration           Int64 CODEC(ZSTD(1)),    -- nanosegundos (tipo del exporter)
    StatusCode         LowCardinality(String) CODEC(ZSTD(1)),
    StatusMessage      String CODEC(ZSTD(1)),
    Events Nested
    (
        Timestamp  DateTime64(9),
        Name       LowCardinality(String),
        Attributes Map(LowCardinality(String), String)
    ) CODEC(ZSTD(1)),
    Links Nested
    (
        TraceId    String,
        SpanId     String,
        TraceState String,
        Attributes Map(LowCardinality(String), String)
    ) CODEC(ZSTD(1)),

    -- Columnas materializadas AgentLens: extracción tipada para filtrar rápido
    -- sin parsear mapas en cada consulta.
    TenantId       LowCardinality(String) MATERIALIZED ResourceAttributes['agentlens.tenant.id'],
    AgentId        LowCardinality(String) MATERIALIZED ResourceAttributes['agentlens.agent.id'],
    GenAIOperation LowCardinality(String) MATERIALIZED SpanAttributes['gen_ai.operation.name'],
    RequestModel   LowCardinality(String) MATERIALIZED SpanAttributes['gen_ai.request.model'],
    InputTokens    UInt32 MATERIALIZED toUInt32OrZero(SpanAttributes['gen_ai.usage.input_tokens']),
    OutputTokens   UInt32 MATERIALIZED toUInt32OrZero(SpanAttributes['gen_ai.usage.output_tokens']),
    IsError        UInt8  MATERIALIZED StatusCode = 'STATUS_CODE_ERROR',

    -- Índices de salto: aceleran búsquedas por traza y por contenido de atributos.
    INDEX idx_trace_id TraceId TYPE bloom_filter(0.001) GRANULARITY 1,
    INDEX idx_agent AgentId TYPE bloom_filter(0.01) GRANULARITY 1,
    INDEX idx_duration Duration TYPE minmax GRANULARITY 1
)
ENGINE = MergeTree
PARTITION BY toDate(Timestamp)
-- Tenant primero: las consultas del dashboard filtran siempre por tenant.
ORDER BY (TenantId, ServiceName, SpanName, Timestamp)
TTL toDateTime(Timestamp) + INTERVAL 30 DAY
SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1;

-- ---------------------------------------------------------------------------
-- Resumen por traza (alimenta la lista de trazas del dashboard, E3-T01/T03)
-- ---------------------------------------------------------------------------
-- AggregatingMergeTree con estados parciales; se consulta con *Merge + GROUP BY.
-- Evita escanear todos los spans para pintar la lista de trazas.
CREATE TABLE IF NOT EXISTS agentlens.trace_summary
(
    TenantId     LowCardinality(String),
    TraceId      String,
    ServiceName  AggregateFunction(any, LowCardinality(String)),
    AgentId      AggregateFunction(any, LowCardinality(String)),
    RootSpanName AggregateFunction(argMin, String, UInt64),
    StartNs      AggregateFunction(min, Int64),
    EndNs        AggregateFunction(max, Int64),
    SpanCount    AggregateFunction(count),
    ErrorCount   AggregateFunction(sum, UInt8),
    InputTokens  AggregateFunction(sum, UInt32),
    OutputTokens AggregateFunction(sum, UInt32)
)
ENGINE = AggregatingMergeTree
PARTITION BY tuple()
ORDER BY (TenantId, TraceId);

-- La MV transforma cada bloque insertado en otel_traces en estados parciales.
-- El span raíz es el de ParentSpanId vacío (argMin por longitud del parent).
CREATE MATERIALIZED VIEW IF NOT EXISTS agentlens.trace_summary_mv
TO agentlens.trace_summary
AS
SELECT
    ResourceAttributes['agentlens.tenant.id']                         AS TenantId,
    TraceId,
    anyState(ServiceName)                                             AS ServiceName,
    anyState(ResourceAttributes['agentlens.agent.id'])               AS AgentId,
    argMinState(toString(SpanName), length(ParentSpanId))            AS RootSpanName,
    minState(toUnixTimestamp64Nano(Timestamp))                       AS StartNs,
    maxState(toInt64(toUnixTimestamp64Nano(Timestamp)) + toInt64(Duration)) AS EndNs,
    countState()                                                      AS SpanCount,
    sumState(toUInt8(StatusCode = 'STATUS_CODE_ERROR'))              AS ErrorCount,
    sumState(toUInt32OrZero(SpanAttributes['gen_ai.usage.input_tokens']))  AS InputTokens,
    sumState(toUInt32OrZero(SpanAttributes['gen_ai.usage.output_tokens'])) AS OutputTokens
FROM agentlens.otel_traces
GROUP BY TenantId, TraceId;
