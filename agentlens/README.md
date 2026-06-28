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
  canónico estable. Auto-instrumentación resiliente y ampliada (E1-T10): OpenAI,
  LangChain/LangGraph, CrewAI, AutoGen, Pydantic AI, Bedrock. 23 tests en verde;
  gate de latencia p99 ≈ 0,1 ms.
- **`sdk-node/`** — SDK TypeScript `@agentlens/node` (E1-T11), paridad funcional
  con Python: instrument 3 líneas, enriquecimiento, convenciones, redacción PII y
  externalización de payloads. 11 tests (node:test) en verde; typecheck y build.
- **`ingestion/`** — Ingestion Gateway (Go, E2-T05/T06): OTLP/gRPC con TLS, auth
  por API key, resolución de tenant y rate limiting por plan (token-bucket).
  Rechaza claves inválidas, limita el caudal por tier, sella el tenant
  autoritativo en el Resource (anti-spoofing) y reenvía al Collector. 18 tests
  (unit + integración gRPC real, `-race`) en verde; verificado end-to-end
  cross-language SDK Python → gateway → downstream.
- **`deploy/`** — Entorno local: OTel Collector (con redacción como segunda
  barrera) + ClickHouse, vía `docker compose`. **Esquema ClickHouse explícito
  (E2-T07)**: tabla de spans compatible con el exporter OTel + columnas
  materializadas (tenant/agent/tokens), ORDER BY tenant-first, índices de salto
  y vista materializada de resumen de trazas para el dashboard. Verificado con
  ClickHouse real (chDB): inserts, aislamiento por tenant y queries sub-ms.
- **`api/`** — API hot path (Go, E3-T01/T02/T06): lectura de trazas sobre
  ClickHouse. Endpoints de listado (paginado), detalle de traza y **coste agregado
  por agente/modelo** (E3-T06), con **auth por tenant** (API key Bearer → tenant,
  no spoofeable) y **esquema OpenAPI** documentado y servido. `TraceStore`
  desacoplado (ClickHouse + fake en memoria). Tests `-race` de
  auth/paginación/aislamiento/coste, SQL validado contra ClickHouse real y smoke
  del binario.
- **`frontend/`** — Dashboard Next.js (E3-T03/T04/T06): lista de trazas, detalle
  con **vista de conversación GenAI** (chat-style) y **timeline de spans**, y
  **dashboard de coste** por agente/modelo. Capa de datos con fallback a fixtures.
  Build (type-check) y render de las vistas verificados con Chromium real.
- **`examples/`** — Agente de ejemplo end-to-end.

## Estructura objetivo del monorepo

```
agentlens/
├── sdk-python/      # ✅ Core Tracing SDK (Python)
├── sdk-node/        # ✅ SDK TypeScript @agentlens/node (E1-T11)
├── collector/       # ◻️ config base en deploy/; procesadores Go custom (E2-T02)
├── ingestion/       # ✅ Ingestion Gateway (Go) (E2-T05/T06); ⬜ stream processors
├── compliance-engine/ # ⬜ Mapeo regulatorio (Python) (E5)
├── reporter/        # ⬜ Informes firmados (E5-T05)
├── forensics/       # ⬜ Reconstrucción de ejecuciones (E6)
├── api/             # ✅ API de trazas + coste (Go) (E3-T01/T02/T06)
├── frontend/        # ✅ Dashboard Next.js (E3-T03/T04/T06): trazas + conversación + coste
├── policies/        # ⬜ Bundles Rego (E4)
└── deploy/          # ✅ docker-compose + esquema ClickHouse (E2-T07); ◻️ Helm (E0-T06)
```

## Mapa con el backlog

Lo cubierto hasta ahora: E1-T01..T14 (SDK Python core + convenciones +
auto-instrumentación ampliada + **SDK TypeScript** + **docs + publicación**),
E0-T03 (gate de latencia), base de E0-T02 (CI), E0-T04 (entorno local), E2-T01
(Collector base), **E2-T05/T06 (Ingestion Gateway
en Go: auth + rate limiting por plan)**, **E2-T07 (esquema ClickHouse explícito +
resumen de trazas)**, **E3-T01/T02 (API de lectura de trazas: paginación + auth
por tenant + OpenAPI)**, **E3-T06 (coste por agente/modelo)** y **E3-T03/T04
(dashboard Next.js: lista, timeline, conversación GenAI y coste)**.

Con esto, el camino visible del MVP (SDK → ingesta → almacenamiento → API →
dashboard) está completo de extremo a extremo, y ambos SDKs (Python + Node) están
**listos para publicar** (E1-T13/T14): LICENSE Apache-2.0, metadatos completos,
`py.typed`, builds de distribución verificados (`python -m build` / `npm pack`),
tutorial de integración en 5 pasos (`docs/integration.md`) y workflows de CI y
release (PyPI Trusted Publishing + npm) en `.github/workflows/`.

Siguiente paso recomendado: **E2-T08/E3-T07** (PostgreSQL + inventario de
agentes), **E3-T05** (live feed por WebSocket) o saltar a **E4** (policy engine).
La publicación efectiva a PyPI/npm requiere etiquetar un release y configurar los
secretos/Trusted Publishing del repositorio.

## Arranque rápido

```bash
cd deploy && docker compose up -d
cd ../sdk-python && pip install -e ".[dev]" && pytest -q
cd ../examples && python simple_agent.py
```
