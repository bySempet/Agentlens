# AgentLens — Monorepo

Gobernanza y observabilidad para agentes de IA.

## Estado actual (primer incremento — E1 + base de E2/E0)

Implementado y verificado:

- **`sdk-python/`** — Core Tracing SDK (v0.1). `instrument()` de 3 líneas,
  export OTLP asíncrono, redacción de PII en cliente, externalización de
  payloads, enriquecimiento de spans y helpers de agente/herramienta según las
  convenciones OTel GenAI. Incluye la **capa adaptadora de convenciones**
  (E1-T09): aísla el esquema interno del OTel GenAI *Development* y normaliza
  alias legacy (`llm.*`, `ai.*`, versiones previas de `gen_ai.*`) a un esquema
  canónico estable. 17 tests en verde; gate de latencia p99 ≈ 0,1 ms.
- **`ingestion/`** — Ingestion Gateway (Go, E2-T05 + E2-T06): OTLP/gRPC con TLS,
  auth por API key y resolución de tenant + plan. Rechaza claves inválidas,
  sella el tenant autoritativo en el Resource (anti-spoofing), aplica **rate
  limiting por plan** (token bucket por tenant, coste en spans, respuesta
  `ResourceExhausted` reintentable) y reenvía al Collector. Tests de auth,
  enrutado end-to-end sobre gRPC real y carga concurrente del limitador en
  verde (`-race`); throttling verificado E2E (plan de 5 spans/s ingiere 30
  spans en ~6 s; plan pro, en 0,3 s).
- **`deploy/`** — Entorno local: OTel Collector (con redacción como segunda
  barrera) + ClickHouse, vía `docker compose`. Incluye el **esquema ClickHouse
  explícito y versionado** (E2-T07, `deploy/clickhouse/schema/`): `otel_traces`
  con `TenantId`/`AgentId` materializadas y orden primario por tenant, índice
  traza→tiempo y vista `agentlens.traces` de resúmenes para el listado del
  dashboard. Verificado end-to-end (SDK → Gateway → Collector → ClickHouse) con
  consultas de listado/detalle en ~10-15 ms.
- **`api/`** — Trace API (Go, E3-T01): lectura de trazas sobre ClickHouse para
  el dashboard. Listado paginado por cursor (vista `agentlens.traces`) y detalle
  de spans (podado por el índice traza→tiempo), auth por API key con datos
  acotados al tenant (una traza ajena responde 404). Solo stdlib; consultas con
  parámetros tipados de ClickHouse (sin inyección). Tests de handlers +
  integración real en verde; listado/detalle en ~10-16 ms.

## Estructura objetivo del monorepo

```
agentlens/
├── sdk-python/      # ✅ Core Tracing SDK (Python)
├── sdk-node/        # ⬜ SDK TypeScript (E1-T11)
├── collector/       # ◻️ config base en deploy/; procesadores Go custom (E2-T02)
├── ingestion/       # ✅ Ingestion Gateway (Go) (E2-T05 + rate limiting E2-T06)
├── api/             # ✅ Trace API de lectura (Go) (E3-T01)
├── compliance-engine/ # ⬜ Mapeo regulatorio (Python) (E5)
├── reporter/        # ⬜ Informes firmados (E5-T05)
├── forensics/       # ⬜ Reconstrucción de ejecuciones (E6)
├── frontend/        # ⬜ Dashboard Next.js (E3-T03)
├── policies/        # ⬜ Bundles Rego (E4)
└── deploy/          # ✅ docker-compose + esquema ClickHouse (E2-T07); ◻️ Helm (E0-T06)
```

## Mapa con el backlog

Lo cubierto hasta ahora: E1-T01..T09 (SDK core + redacción + payloads +
enriquecimiento + base de auto-instrumentación + **capa adaptadora de
convenciones**), E0-T03 (gate de latencia), E0-T04 (entorno local), E2-T01
(Collector base), E2-T05 (Ingestion Gateway en Go), E2-T07 (esquema ClickHouse
explícito + ingestión), E2-T06 (tenant por plan + rate limiting) y **E3-T01
(Trace API de lectura)**.

Siguiente paso recomendado del backlog: **E2-T08** (esquema PostgreSQL +
migraciones: agentes, políticas, orgs y usuarios, del que dependen E3-T07 y E4)
o **E3-T03** (dashboard Next.js, que ya tiene API que consumir vía E3-T02). En
paralelo, **E1-T10/T11** (más frameworks + SDK Node) se apoyan en la capa de
convenciones.

## Arranque rápido

```bash
cd deploy && docker compose up -d
cd ../sdk-python && pip install -e ".[dev]" && pytest -q
cd ../examples && python simple_agent.py
```
