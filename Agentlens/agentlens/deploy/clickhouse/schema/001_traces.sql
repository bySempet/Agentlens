-- E2-T07 · Esquema explícito de spans/trazas de AgentLens (v1).
--
-- Sustituye a la tabla auto-creada por el exporter ClickHouse del Collector
-- (create_schema: false en la config). La lista y tipos de las columnas que
-- inserta el exporter (v0.110.0) se conservan EXACTAMENTE; lo que cambia:
--
--   * TenantId / AgentId como columnas MATERIALIZED desde ResourceAttributes:
--     el exporter no las conoce (no van en su INSERT) y ClickHouse las calcula
--     en cada inserción. Son las claves de filtrado de todo el producto.
--   * ORDER BY empieza por TenantId: toda query de producto filtra por tenant
--     (multi-tenancy), así el índice primario poda por tenant antes que nada.
--   * Índices secundarios del exporter conservados (trace_id, mapas, duración).
--
-- Si se actualiza la versión del Collector, revisar que el INSERT del exporter
-- siga siendo compatible (exporter/clickhouseexporter/exporter_traces.go).

CREATE TABLE IF NOT EXISTS agentlens.otel_traces
(
    Timestamp          DateTime64(9) CODEC (Delta, ZSTD(1)),
    TraceId            String CODEC (ZSTD(1)),
    SpanId             String CODEC (ZSTD(1)),
    ParentSpanId       String CODEC (ZSTD(1)),
    TraceState         String CODEC (ZSTD(1)),
    SpanName           LowCardinality(String) CODEC (ZSTD(1)),
    SpanKind           LowCardinality(String) CODEC (ZSTD(1)),
    ServiceName        LowCardinality(String) CODEC (ZSTD(1)),
    ResourceAttributes Map(LowCardinality(String), String) CODEC (ZSTD(1)),
    ScopeName          String CODEC (ZSTD(1)),
    ScopeVersion       String CODEC (ZSTD(1)),
    SpanAttributes     Map(LowCardinality(String), String) CODEC (ZSTD(1)),
    Duration           Int64 CODEC (ZSTD(1)),
    StatusCode         LowCardinality(String) CODEC (ZSTD(1)),
    StatusMessage      String CODEC (ZSTD(1)),
    Events Nested (
        Timestamp DateTime64(9),
        Name LowCardinality(String),
        Attributes Map(LowCardinality(String), String)
    ) CODEC (ZSTD(1)),
    Links Nested (
        TraceId String,
        SpanId String,
        TraceState String,
        Attributes Map(LowCardinality(String), String)
    ) CODEC (ZSTD(1)),

    -- Claves de producto, selladas por el Ingestion Gateway (anti-spoofing).
    TenantId           LowCardinality(String) MATERIALIZED ResourceAttributes['agentlens.tenant.id'] CODEC (ZSTD(1)),
    AgentId            LowCardinality(String) MATERIALIZED ResourceAttributes['agentlens.agent.id'] CODEC (ZSTD(1)),

    INDEX idx_trace_id TraceId TYPE bloom_filter(0.001) GRANULARITY 1,
    INDEX idx_agent_id AgentId TYPE bloom_filter(0.01) GRANULARITY 1,
    INDEX idx_res_attr_key mapKeys(ResourceAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
    INDEX idx_res_attr_value mapValues(ResourceAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
    INDEX idx_span_attr_key mapKeys(SpanAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
    INDEX idx_span_attr_value mapValues(SpanAttributes) TYPE bloom_filter(0.01) GRANULARITY 1,
    INDEX idx_duration Duration TYPE minmax GRANULARITY 1
)
ENGINE = MergeTree
PARTITION BY toDate(Timestamp)
ORDER BY (TenantId, ServiceName, toUnixTimestamp(Timestamp), TraceId)
TTL toDateTime(Timestamp) + toIntervalDay(30)  -- retención del plan Starter
SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1;
