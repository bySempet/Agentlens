# Trace API (Go) — E3-T01

API hot path de **lectura** de trazas sobre ClickHouse. Es la que consumirá el
dashboard (E3-T03); no ingiere nada (eso es del Ingestion Gateway).

```
Dashboard / integraciones  --HTTP + API key-->  Trace API  --HTTP-->  ClickHouse
```

- **Auth por API key** (cabecera `X-AgentLens-Key`, misma key que la ingesta):
  la key resuelve al tenant y **toda** consulta queda acotada a él. Una traza de
  otro tenant responde `404` (no se revela su existencia).
- **Solo stdlib**: habla con ClickHouse por su interfaz HTTP con parámetros
  tipados del servidor (`{name:Type}` + `param_name`), así que ningún valor del
  cliente se interpola en el SQL.
- Lee el esquema de E2-T07: el listado sale de la vista `agentlens.traces`
  (resúmenes pre-agregados) y el detalle poda por tiempo con
  `otel_traces_trace_id_ts` antes de tocar `otel_traces`.

## Endpoints

| Método y ruta | Descripción |
| --- | --- |
| `GET /healthz` | Liveness (sin auth) |
| `GET /v1/traces` | Listado paginado, más recientes primero. Query: `limit` (def. 50, máx. 200), `cursor` (opaco, de la respuesta anterior), `agent_id` (filtro opcional) |
| `GET /v1/traces/{trace_id}` | Spans de la traza en orden temporal |

Respuesta del listado: `{"traces": [...], "next_cursor": "..."}`; `next_cursor`
solo aparece si hay más páginas. Los tiempos van en epoch-nanos
(`start_unix_nano`, `duration_ns`).

## Configuración (variables de entorno)

| Variable | Default | Descripción |
| --- | --- | --- |
| `AGENTLENS_API_LISTEN` | `:8080` | Dirección de escucha HTTP |
| `AGENTLENS_API_CLICKHOUSE` | `http://localhost:8123` | Endpoint HTTP de ClickHouse |
| `AGENTLENS_API_CLICKHOUSE_USER` | `agentlens` | Usuario de ClickHouse |
| `AGENTLENS_API_CLICKHOUSE_PASSWORD` | — | Contraseña (el docker-compose local usa `agentlens`) |
| `AGENTLENS_API_KEYS` | — | Entradas `key:tenant[:plan]` separadas por coma (mismo formato que el gateway; el plan se ignora aquí) |

## Ejecutar localmente

```bash
# Contra el stack del docker-compose (deploy/)
export AGENTLENS_API_CLICKHOUSE_PASSWORD=agentlens
export AGENTLENS_API_KEYS="demo-key:acme-corp"
go run ./cmd/api

curl -H 'X-AgentLens-Key: demo-key' 'localhost:8080/v1/traces?limit=20'
```

## Tests

```bash
go test -race ./...

# Con integración contra un ClickHouse real (esquema E2-T07 aplicado):
AGENTLENS_TEST_CLICKHOUSE=http://127.0.0.1:8123 go test -race ./...
```

Los de `httpapi` cubren auth, scoping por tenant, paginación por cursor,
parámetros inválidos y 404; los de integración verifican orden, cursor sin
duplicados y aislamiento de tenant contra ClickHouse de verdad.

## Build de la imagen

```bash
docker build -t agentlens/trace-api .
```
