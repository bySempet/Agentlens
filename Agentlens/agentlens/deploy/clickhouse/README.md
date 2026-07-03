# Esquema ClickHouse de AgentLens (E2-T07)

Esquema **explícito y versionado** del almacén de trazas. El exporter del
Collector ya no crea tablas (`create_schema: false`): el DDL de este directorio
es la única fuente de verdad.

## Archivos

| Archivo | Contenido |
| --- | --- |
| `schema/000_database.sql` | Base de datos `agentlens` |
| `schema/001_traces.sql` | `otel_traces`: spans, compatible con el INSERT del exporter, con `TenantId`/`AgentId` materializadas y orden primario por tenant |
| `schema/002_trace_id_ts.sql` | Índice traza → ventana temporal (para el detalle de traza) |
| `schema/003_trace_summaries.sql` | Resumen incremental por traza + vista `agentlens.traces` (para el listado del dashboard, E3-T01) |

En docker-compose el directorio `schema/` se monta en
`/docker-entrypoint-initdb.d` y se aplica en orden alfabético al primer
arranque del volumen. Fuera de Docker: `clickhouse client --multiquery < schema/00X_*.sql`
en orden.

## Reglas de evolución

- Los archivos aplicados no se editan: los cambios de esquema van en un archivo
  nuevo numerado (`004_...sql`), idempotente (`IF NOT EXISTS` / `ALTER`).
- Las columnas y tipos que espera el INSERT del exporter ClickHouse
  (`exporter/clickhouseexporter/exporter_traces.go` de la versión fijada en
  `deploy/docker-compose.yml`) no pueden cambiar sin actualizar el Collector a
  la vez. Las columnas propias van como `MATERIALIZED` (el exporter no las
  menciona y ClickHouse las calcula en cada inserción).
- Consultar el listado de trazas SIEMPRE vía `agentlens.traces` (re-agrega los
  parciales de `trace_summaries`, que pueden no estar fusionados).

## Consultas de referencia

```sql
-- Listado por tenant (dashboard)
SELECT TraceId, RootSpanName, StartTime, DurationNs, SpanCount,
       ErrorCount, InputTokens, OutputTokens, Model
FROM agentlens.traces
WHERE TenantId = 'acme-corp'
ORDER BY StartTime DESC
LIMIT 20;

-- Detalle de una traza (spans)
SELECT Timestamp, SpanName, SpanKind, Duration, StatusCode, SpanAttributes
FROM agentlens.otel_traces
WHERE TenantId = 'acme-corp' AND TraceId = '...'
ORDER BY Timestamp;
```
