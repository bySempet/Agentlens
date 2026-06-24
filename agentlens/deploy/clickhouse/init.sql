-- Inicialización de ClickHouse para AgentLens (entorno local).
--
-- El exporter ClickHouse del Collector crea automáticamente la tabla
-- `otel_traces` (con create_schema: true). Aquí solo aseguramos la base de
-- datos y añadimos una vista de conveniencia para inspección rápida.

CREATE DATABASE IF NOT EXISTS agentlens;

-- Vista de conveniencia: las acciones de agentes y herramientas, con el tenant
-- y el agente que las generó, ordenadas por tiempo. Se materializa al primer
-- uso tras la creación de `otel_traces` por el Collector.
--
-- Consulta de ejemplo una vez haya datos:
--
--   SELECT Timestamp, SpanName,
--          ResourceAttributes['agentlens.tenant.id'] AS tenant,
--          ResourceAttributes['agentlens.agent.id']  AS agent,
--          SpanAttributes['gen_ai.tool.name']         AS tool,
--          SpanAttributes['agentlens.payload.externalized'] AS externalized
--   FROM agentlens.otel_traces
--   ORDER BY Timestamp DESC
--   LIMIT 50;
