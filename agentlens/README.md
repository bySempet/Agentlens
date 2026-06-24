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
- **`ingestion/`** — Ingestion Gateway (Go, E2-T05): OTLP/gRPC con TLS, auth por
  API key y resolución de tenant. Rechaza claves inválidas, sella el tenant
  autoritativo en el Resource (anti-spoofing) y reenvía al Collector. Tests de
  auth y de enrutado end-to-end sobre gRPC real en verde.
- **`deploy/`** — Entorno local: OTel Collector (con redacción como segunda
  barrera) + ClickHouse, vía `docker compose`.
- **`examples/`** — Agente de ejemplo end-to-end.

## Estructura objetivo del monorepo

```
agentlens/
├── sdk-python/      # ✅ Core Tracing SDK (Python)
├── sdk-node/        # ⬜ SDK TypeScript (E1-T11)
├── collector/       # ◻️ config base en deploy/; procesadores Go custom (E2-T02)
├── ingestion/       # ✅ Ingestion Gateway (Go) (E2-T05); ⬜ stream processors
├── compliance-engine/ # ⬜ Mapeo regulatorio (Python) (E5)
├── reporter/        # ⬜ Informes firmados (E5-T05)
├── forensics/       # ⬜ Reconstrucción de ejecuciones (E6)
├── frontend/        # ⬜ Dashboard Next.js (E3-T03)
├── policies/        # ⬜ Bundles Rego (E4)
└── deploy/          # ✅ docker-compose local; ◻️ Helm charts (E0-T06)
```

## Mapa con el backlog

Lo cubierto hasta ahora: E1-T01..T09 (SDK core + redacción + payloads +
enriquecimiento + base de auto-instrumentación + **capa adaptadora de
convenciones**), E0-T03 (gate de latencia), E0-T04 (entorno local), E2-T01
(Collector base) y **E2-T05 (Ingestion Gateway en Go)**.

Siguiente paso recomendado del backlog: **E2-T07** (esquema ClickHouse explícito
+ ingestión) y **E2-T06** (resolución de tenant por plan + rate limiting, que se
apoya en la auth del gateway ya disponible). En paralelo, **E1-T10/T11** (más
frameworks + SDK Node) se apoyan en la capa de convenciones.

## Arranque rápido

```bash
cd deploy && docker compose up -d
cd ../sdk-python && pip install -e ".[dev]" && pytest -q
cd ../examples && python simple_agent.py
```
