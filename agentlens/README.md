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
- **`deploy/`** — Entorno local: OTel Collector (con redacción como segunda
  barrera) + ClickHouse, vía `docker compose`.
- **`examples/`** — Agente de ejemplo end-to-end.

## Estructura objetivo del monorepo

```
agentlens/
├── sdk-python/      # ✅ Core Tracing SDK (Python)
├── sdk-node/        # ⬜ SDK TypeScript (E1-T11)
├── collector/       # ◻️ config base en deploy/; procesadores Go custom (E2-T02)
├── ingestion/       # ⬜ Ingestion Gateway + stream processors (Go) (E2-T05)
├── compliance-engine/ # ⬜ Mapeo regulatorio (Python) (E5)
├── reporter/        # ⬜ Informes firmados (E5-T05)
├── forensics/       # ⬜ Reconstrucción de ejecuciones (E6)
├── frontend/        # ⬜ Dashboard Next.js (E3-T03)
├── policies/        # ⬜ Bundles Rego (E4)
└── deploy/          # ✅ docker-compose local; ◻️ Helm charts (E0-T06)
```

## Mapa con el backlog

Este incremento cubre: E1-T01..T09 (SDK core + redacción + payloads +
enriquecimiento + base de auto-instrumentación + **capa adaptadora de
convenciones**), E0-T03 (gate de latencia), E0-T04 (entorno local) y E2-T01
(Collector base).

Siguiente paso recomendado del backlog: **E2-T05/E2-T07** (Ingestion Gateway en
Go + esquema ClickHouse) para cerrar el camino de ingesta del MVP, y **E1-T10/
T11** (más frameworks + SDK Node), que se apoyan en la capa de convenciones ya
disponible.

## Arranque rápido

```bash
cd deploy && docker compose up -d
cd ../sdk-python && pip install -e ".[dev]" && pytest -q
cd ../examples && python simple_agent.py
```
