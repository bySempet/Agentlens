# CLAUDE.md

Guía para Claude Code en este repositorio.

## Qué es

AgentLens: gobernanza y observabilidad para agentes de IA sobre OpenTelemetry GenAI.
Monorepo bajo `agentlens/`. Backlog en `AgentLens_Backlog_Tecnico.md` (IDs `Ex-Tyy`).
Código, docs y commits en **español**.

```
Agente + SDK (Python/Node) --OTLP/gRPC + x-agentlens-key--> Ingestion Gateway (Go)
   --OTLP--> OTel Collector (2ª barrera PII) --> ClickHouse <-- API (Go) <-- Dashboard (Next.js)
                                    PostgreSQL (orgs/agentes/políticas) <-- API /v1/agents
Policy Engine (Go, OPA/Rego -> bundle WASM)    Compliance Engine (Python: mapeos, PDF firmado, madurez)
```

## Ramas

- `main` está obsoleta (solo la subida inicial). **No basar trabajo en `main`.**
- `develop` es la rama de integración; recibe PRs desde `claude/gallant-dirac-74kyr4`.
- Se trabaja y se hace push en `claude/gallant-dirac-74kyr4` (SSH, deploy key con escritura).
- El CI (`.github/workflows/ci.yml`) solo corre en push a `main`/`develop` y en PRs.

## Estado del backlog

| Épica | Hecho | Pendiente relevante |
| --- | --- | --- |
| E0 Fundamentos | T02 (CI), T04 (compose local), T08 (auto-observabilidad OTel) | **T03: figura como hecha pero no existe ningún benchmark/gate de latencia en el repo**; T01/T05–T07/T09 |
| E1 SDK | **Completa** (T01–T14; Python + `@agentlens/node`) | Publicación real en PyPI/npm requiere tag + secretos |
| E2 Ingesta | T01, T05, T06, T07, T08 | T02 (Presidio), T03, T04, T09–T13 |
| E3 API + dashboard | **Completa** (T01–T07) | — |
| E4 Policy Engine | T01, T02 | **T03 (siguiente: enforcement en el borde con `policy.wasm` vía wasmtime)**, T04–T08 |
| E5 Compliance | T01, T02, T03, T05, T06 | T04 (ISO 42001/NIST), T07 (evidencias) |
| E6–E12 | — | Sin empezar |

Huecos conocidos:
- La vista `/compliance` del frontend pide `${AGENTLENS_COMPLIANCE_URL}/v1/compliance`, pero
  **ningún servicio expone ese endpoint**: hoy solo funciona con fixtures.
- `deploy/docker-compose.yml` publica los puertos en `0.0.0.0`. En la EC2, usar un override
  que los ate a `127.0.0.1` (ver "Stack local").
- El sello de tiempo del PDF firmado lo declara el firmante (no es un TSA RFC 3161).
- `reporter/`, `policies/`, `collector/` del README son placeholders: el código vive en
  `compliance-engine/`, `policy-engine/` y `deploy/`.

## Convenciones

- **Commits** en español, formato `tipo(ámbito): descripción (ID)`, con tipo `feat`/`fix`/
  `improve`/`refactor`/`test`. **Un commit por tarea** del backlog. Las mejoras tras una
  revisión van en commits `improve`/`fix` aparte.
- **Verificar ejecutando de verdad**: tests + servicios reales (Docker, CLI, curl), no solo compilar.
- **Revisar el código al cerrar cada épica** (correctitud, seguridad, consistencia).
- **Preguntar antes** de gastar dinero en AWS, hacer algo irreversible o exponer algo a internet.
- Actualizar el README del paquete y el `agentlens/README.md` (mapa del backlog) con cada tarea.
- Invariantes que no se pueden romper:
  - El gateway **sella** `agentlens.tenant.id` con el tenant de la API key (anti-spoofing).
  - La API acota toda consulta al tenant de la key; una traza ajena da 404.
  - La PII se redacta dos veces: en el SDK y en el Collector. No quitar ninguna de las dos.
  - `sdk-python/src/agentlens/conventions.py` es el único módulo que conoce los nombres de
    claves OTel GenAI.
  - El esquema de ClickHouse es de AgentLens (`create_schema: false` en el Collector);
    el listado se consulta vía la vista `agentlens.traces`.
  - El PDF firmado se verifica **contra una clave pública de confianza**, nunca con la que
    trae el sidecar.

## Entorno (EC2 Ubuntu 24.04)

- Go 1.24 en `/usr/local/go/bin`; Python 3.11 en el venv `~/.venvs/agentlens` (con
  `sdk-python` y `compliance-engine` instalados en editable con `[dev]`); Node 22;
  Docker + compose (si el shell no tiene el grupo `docker`, usar `sg docker -c "..."`);
  wasmtime; Chromium de Playwright en `~/.cache/ms-playwright`; `psql` 16.
- `~/.profile` añade Go al PATH y activa el venv.

## Cómo verificar cada componente

Rutas relativas a `agentlens/`.

```bash
# Go (igual que el CI): ingestion, api, policy-engine, shared
for m in ingestion api policy-engine shared; do (cd $m && go build ./... && go vet ./... && go test -race ./...); done

# SDK Python y compliance engine
(cd sdk-python && pytest -q)
(cd compliance-engine && pytest -q)

# SDK Node
(cd sdk-node && npm ci && npm run typecheck && npm test && npm run build)

# Frontend: build + E2E Playwright (Chromium, con fixtures)
(cd frontend && npm ci && npm run build && npm run test:e2e)

# Postgres real: migraciones + verificación de esquema + test de integración del inventario
docker run -d --rm --name al-pg -e POSTGRES_DB=agentlens -e POSTGRES_USER=agentlens \
  -e POSTGRES_PASSWORD=agentlens -p 127.0.0.1:5432:5432 postgres:16
export PGPASSWORD=agentlens
(cd deploy/postgres && PSQL="psql -h localhost -U agentlens -d agentlens" ./migrate.sh \
  && psql -h localhost -U agentlens -d agentlens -v ON_ERROR_STOP=1 -f verify_schema.sql)
(cd api && AGENTLENS_TEST_PG_DSN="postgres://agentlens:agentlens@localhost:5432/agentlens?sslmode=disable" \
  go test ./internal/agents/ -run TestPostgresStore_CRUD -v)
docker stop al-pg

# Esquema ClickHouse sin Docker (chDB)
pip install chdb && python deploy/clickhouse/verify_schema.py

# Policy engine: CLI (exit 3 = denegado) y bundle WASM
(cd policy-engine && echo '{"tool":"shell.exec"}' | go run ./cmd/policyeval; \
  go run ./cmd/buildbundle -out build/bundle.tar.gz)

# Compliance: informe JSON, PDF firmado y verificación con clave de confianza
python -m agentlens_compliance --keygen firma          # firma.key (0600) + firma.pub
cat spans.json | python -m agentlens_compliance --framework gdpr --pdf i.pdf --signing-key firma.key
python -m agentlens_compliance --verify i.pdf --pubkey firma.pub   # exit 0 OK / 1 fallo
```

### Stack local end-to-end (SDK → gateway → Collector → ClickHouse → API)

Override para no exponer puertos fuera de la máquina (`compose.local.yml`, fuera del repo):

```yaml
services:
  clickhouse:     { ports: !override ["127.0.0.1:8123:8123", "127.0.0.1:9000:9000"] }
  postgres:       { ports: !override ["127.0.0.1:5432:5432"] }
  otel-collector: { ports: !override ["127.0.0.1:4317:4317", "127.0.0.1:4318:4318"] }
```

```bash
docker compose -f deploy/docker-compose.yml -f compose.local.yml up -d
# El Collector ocupa :4317; el gateway escucha en :4319 y reenvía al Collector.
# Claves: "key:tenant[,key2:tenant2]"; el plan de cada tenant va en *_TENANT_PLANS.
(cd ingestion && AGENTLENS_GATEWAY_LISTEN=127.0.0.1:4319 AGENTLENS_GATEWAY_DOWNSTREAM=localhost:4317 \
  AGENTLENS_GATEWAY_API_KEYS="demo-key:acme-corp" AGENTLENS_GATEWAY_TENANT_PLANS="acme-corp:growth" \
  go run ./cmd/gateway) &
(cd api && AGENTLENS_API_LISTEN=127.0.0.1:8080 AGENTLENS_API_KEYS="demo-key:acme-corp" \
  AGENTLENS_POSTGRES_DSN="postgres://agentlens:agentlens@localhost:5432/agentlens?sslmode=disable" \
  go run ./cmd/api) &
(cd examples && AGENTLENS_ENDPOINT=http://localhost:4319 python simple_agent.py)

curl -H 'Authorization: Bearer demo-key' 'http://127.0.0.1:8080/v1/traces?limit=5'   # 1 traza, 3 spans
curl -o /dev/null -w '%{http_code}\n' http://127.0.0.1:8080/v1/traces                 # 401
curl -H 'Authorization: Bearer demo-key' http://127.0.0.1:8080/v1/cost
# /v1/agents exige que el tenant exista en Postgres:
psql -h localhost -U agentlens -d agentlens -c \
  "INSERT INTO organizations(slug,name,plan) VALUES('acme-corp','Acme','growth')"
curl -H 'Authorization: Bearer demo-key' -X POST http://127.0.0.1:8080/v1/agents -d '{"name":"bot"}'
docker compose -f deploy/docker-compose.yml down -v
```

Qué comprobar: en ClickHouse (`otel_traces`), los argumentos con email salen como
`[REDACTED_EMAIL]`, el payload grande lleva `agentlens.payload.externalized` y
`gen_ai.usage.prompt_tokens` llega normalizado a `gen_ai.usage.input_tokens`. Con una clave
inválida, el SDK recibe `UNAUTHENTICATED` del gateway.
