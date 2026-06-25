# API hot path (Go) — E3-T01 / E3-T02

API de lectura de trazas para el dashboard y las integraciones. Lee de ClickHouse
(esquema E2-T07) y expone listado y detalle de trazas con paginación, autenticada
por tenant y con esquema OpenAPI documentado.

## Endpoints

| Método | Ruta | Auth | Descripción |
| --- | --- | --- | --- |
| `GET` | `/v1/traces?limit=&offset=` | sí | Lista de trazas del tenant (resumen) |
| `GET` | `/v1/traces/{traceId}` | sí | Detalle: spans de la traza (timeline) |
| `GET` | `/openapi.yaml` | no | Especificación OpenAPI 3.0 (servida embebida) |
| `GET` | `/healthz` | no | Liveness |

- **Auth por tenant (E3-T02)**: cabecera `Authorization: Bearer <api-key>` (o
  `X-AgentLens-Key`). El tenant se **deriva de la key**, no de una cabecera
  spoofeable, así que cada cliente solo ve sus trazas. Sin key o inválida → `401`.
- Paginación: `limit` por defecto 50, máximo 200; `offset` por defecto 0.
  Parámetros inválidos → `400`. Traza inexistente → `404`.
- La lista sale de la vista materializada `agentlens.trace_summary` (un resumen
  por traza), evitando escanear todos los spans.
- El contrato REST está documentado en `spec/openapi.yaml` y se sirve en vivo.

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
| `AGENTLENS_API_KEYS` | — | Pares `key:tenant` separados por coma (auth) |

## Ejecutar

```bash
# Con el stack local (deploy/) levantado:
export AGENTLENS_API_KEYS="demo-key:acme-corp"
go run ./cmd/api
curl -H 'Authorization: Bearer demo-key' 'http://localhost:8080/v1/traces?limit=20'
curl http://localhost:8080/openapi.yaml          # contrato REST (público)
```

## Tests

```bash
go test -race ./...
```

Cubren auth por API key (Bearer / X-AgentLens-Key, 401 sin/ inválida),
aislamiento de tenant derivado de la key, rutas públicas, servicio de la OpenAPI,
paginación (limit/offset, tope máximo), orden por inicio descendente y errores
400/404. La corrección del SQL contra ClickHouse real se valida aparte con
`deploy/clickhouse/verify_schema.py`.
