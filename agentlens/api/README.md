# API hot path (Go) — E3-T01

API de lectura de trazas para el dashboard y las integraciones. Lee de ClickHouse
(esquema E2-T07) y expone listado y detalle de trazas con paginación.

## Endpoints

| Método | Ruta | Descripción |
| --- | --- | --- |
| `GET` | `/v1/traces?limit=&offset=` | Lista de trazas del tenant (resumen) |
| `GET` | `/v1/traces/{traceId}` | Detalle: spans de la traza (timeline) |
| `GET` | `/healthz` | Liveness |

- El tenant se lee de la cabecera `X-AgentLens-Tenant` (en producción la fija el
  gateway/auth tras validar la sesión). Sin tenant → `400`.
- Paginación: `limit` por defecto 50, máximo 200; `offset` por defecto 0.
  Parámetros inválidos → `400`. Traza inexistente → `404`.
- La lista sale de la vista materializada `agentlens.trace_summary` (un resumen
  por traza), evitando escanear todos los spans.

## Arquitectura

`store.TraceStore` desacopla la API del backend:

- `ClickHouseStore` — producción, sobre `clickhouse-go/v2`. Sus consultas son
  exactamente las verificadas con ClickHouse real en
  `deploy/clickhouse/verify_schema.py`.
- `MemoryStore` — fake en memoria para tests y desarrollo sin ClickHouse.

## Configuración (entorno)

| Variable | Default | Descripción |
| --- | --- | --- |
| `AGENTLENS_API_LISTEN` | `:8080` | Dirección de escucha HTTP |
| `AGENTLENS_CLICKHOUSE_ADDR` | `localhost:9000` | Endpoint nativo de ClickHouse |
| `AGENTLENS_CLICKHOUSE_DB` | `agentlens` | Base de datos |
| `AGENTLENS_CLICKHOUSE_USER` | `agentlens` | Usuario |
| `AGENTLENS_CLICKHOUSE_PASS` | `agentlens` | Contraseña |

## Ejecutar

```bash
# Con el stack local (deploy/) levantado:
go run ./cmd/api
curl -H 'X-AgentLens-Tenant: acme-corp' 'http://localhost:8080/v1/traces?limit=20'
```

## Tests

```bash
go test ./...
```

Cubren paginación (limit/offset, tope máximo), orden por inicio descendente,
aislamiento por tenant, y los errores 400/404. La corrección del SQL contra
ClickHouse real se valida aparte con `deploy/clickhouse/verify_schema.py`.
