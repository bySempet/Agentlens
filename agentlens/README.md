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
  cross-language SDK Python → gateway → downstream. **Auto-observable** (E0-T08):
  emite sus propias trazas OTel (otelgrpc) si se configura `AGENTLENS_OTLP_ENDPOINT`.
- **`deploy/`** — Entorno local: OTel Collector (con redacción como segunda
  barrera) + ClickHouse, vía `docker compose`. **Esquema ClickHouse explícito
  (E2-T07)**: tabla de spans compatible con el exporter OTel + columnas
  materializadas (tenant/agent/tokens), ORDER BY tenant-first, índices de salto
  y vista materializada de resumen de trazas para el dashboard. Verificado con
  ClickHouse real (chDB): inserts, aislamiento por tenant y queries sub-ms.
  **Esquema PostgreSQL del plano de control (E2-T08)**: orgs, usuarios, agentes,
  API keys y políticas, con migraciones versionadas (`migrate.sh`). Verificado
  contra PostgreSQL real: migraciones idempotentes, CHECK/UNIQUE y FK CASCADE.
- **`api/`** — API hot path (Go, E3-T01/T02/T05/T06/T07): lectura de trazas sobre
  ClickHouse (listado, detalle, **coste** E3-T06, **live feed WebSocket** E3-T05) e
  **inventario de agentes CRUD** sobre Postgres (E3-T07), con **auth por tenant**
  (API key Bearer/query → tenant, no spoofeable) y **esquema OpenAPI** documentado.
  Stores desacoplados (ClickHouse/Postgres + fakes en memoria). Tests `-race`
  (auth, paginación, aislamiento, coste, WS, CRUD), SQL validado contra ClickHouse
  real e **inventario verificado contra Postgres real**. **Auto-observable**
  (E0-T08): trazas OTel propias (otelhttp) si se configura `AGENTLENS_OTLP_ENDPOINT`.
- **`frontend/`** — Dashboard Next.js (E3-T03..T06 + E5-T06): lista de trazas con
  **live feed**, detalle con **conversación GenAI** y **timeline**, **dashboard de
  coste** y **dashboard de madurez de compliance** (EU AI Act/GDPR). Fallback a
  fixtures. Build + **4 tests E2E Playwright** verificados con Chromium real.
- **`policy-engine/`** — Policy Engine (Go + OPA/Rego, E4-T01/T02): evalúa decisiones
  de gobernanza sobre acciones de agentes (herramientas bloqueadas, PII→externo,
  límite de tokens), data-driven y compilado una vez. **Latencia ~60 µs** por
  evaluación (criterio < 5 ms). Compila la política a **bundle WASM** (E4-T02) para
  enforcement en el borde. Librería + CLIs `policyeval`/`buildbundle`; 9 tests.
- **`compliance-engine/`** — Mapeo regulatorio (Python, E5-T01..T06): traduce las
  trazas en evidencia de cumplimiento. Modelo versionado + **checklist EU AI Act
  de 40 puntos** (Art. 12/14/50/86) y **mapeo GDPR** (Art. 5/15/17/30/33). Informe
  con cobertura/gaps, **PDF firmado (Ed25519) verificable** (E5-T05) y **madurez
  por framework/artículo** (E5-T06). CLI. 14 tests (cobertura, gaps, PDF firmado +
  detección de manipulación, madurez).
- **`examples/`** — Agente de ejemplo end-to-end.

## Estructura objetivo del monorepo

```
agentlens/
├── sdk-python/      # ✅ Core Tracing SDK (Python)
├── sdk-node/        # ✅ SDK TypeScript @agentlens/node (E1-T11)
├── shared/          # ✅ módulo Go compartido: keystore (auth) + observability (OTel)
├── collector/       # ◻️ config base en deploy/; procesadores Go custom (E2-T02)
├── ingestion/       # ✅ Ingestion Gateway (Go) (E2-T05/T06); ⬜ stream processors
├── compliance-engine/ # ✅ EU AI Act + GDPR + PDF firmado + madurez (E5-T01..T06)
├── reporter/        # ◻️ informes firmados en compliance-engine (E5-T05)
├── forensics/       # ⬜ Reconstrucción de ejecuciones (E6)
├── api/             # ✅ API trazas/coste/feed/agentes (Go) (E3-T01/02/05/06/07)
├── frontend/        # ✅ Dashboard Next.js: trazas/coste/live feed + compliance (E5-T06)
├── policy-engine/   # ✅ Motor OPA/Rego + bundle WASM (Go) (E4-T01/T02)
├── policies/        # ◻️ política base + bundle WASM en policy-engine/ (E4-T02)
└── deploy/          # ✅ docker-compose + ClickHouse (E2-T07) + PostgreSQL (E2-T08); ◻️ Helm
```

## Mapa con el backlog

Lo cubierto hasta ahora: E1-T01..T14 (SDK Python core + convenciones +
auto-instrumentación ampliada + **SDK TypeScript** + **docs + publicación**),
E0-T03 (gate de latencia), base de E0-T02 (CI), E0-T04 (entorno local), E2-T01
(Collector base), **E2-T05/T06 (Ingestion Gateway
en Go: auth + rate limiting por plan)**, **E2-T07 (esquema ClickHouse explícito +
resumen de trazas)**, **E2-T08 (esquema PostgreSQL del plano de control)**,
**E3-T01/T02 (API de lectura de trazas: paginación + auth
por tenant + OpenAPI)**, **E3-T05 (live feed WebSocket)**, **E3-T06 (coste por
agente/modelo)**, **E3-T07 (inventario de agentes CRUD sobre Postgres)** y
**E3-T03/T04 (dashboard Next.js: lista, timeline, conversación GenAI, coste y
live feed)**. **Épica E3 completa.**

Con esto, el camino visible del MVP (SDK → ingesta → almacenamiento → API →
dashboard) está completo de extremo a extremo, y ambos SDKs (Python + Node) están
**listos para publicar** (E1-T13/T14): LICENSE Apache-2.0, metadatos completos,
`py.typed`, builds de distribución verificados (`python -m build` / `npm pack`),
tutorial de integración en 5 pasos (`docs/integration.md`) y workflows de CI y
release (PyPI Trusted Publishing + npm) en `.github/workflows/`.

**E3 completa** (T01–T07). Avanzan además **E4 (Policy Engine): E4-T01/T02** y
**E5 (Compliance): E5-T01/T02/T03 + E5-T05/T06** (mapeo EU AI Act + GDPR, **informe
PDF firmado y verificable** y **dashboard de madurez**). Siguiente paso: **E5-T07**
(gestión de evidencias), **E5-T04** (ISO 42001 / NIST AI RMF), **E4-T03**
(enforcement en el borde) y **E4-T06** (UI de políticas). La publicación a
PyPI/npm requiere etiquetar un release y configurar los secretos/Trusted
Publishing del repositorio.

## Arranque rápido

```bash
cd deploy && docker compose up -d
cd ../sdk-python && pip install -e ".[dev]" && pytest -q
cd ../examples && python simple_agent.py
```
