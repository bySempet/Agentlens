# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Qué es

AgentLens: gobernanza y observabilidad para agentes de IA sobre OpenTelemetry GenAI. Monorepo bajo `agentlens/` con tres paquetes implementados (`sdk-python/`, `ingestion/`, `api/`) más `deploy/` (stack local + esquema ClickHouse) y `examples/`. Todo el código, docs y commits se escriben en **español**. Las tareas se identifican con IDs del backlog (`AgentLens_Backlog_Tecnico.md`, formato `Ex-Tyy`, p. ej. E2-T05); los commits y READMEs referencian esos IDs.

## Comandos

```bash
# SDK Python (desde agentlens/sdk-python/)
pip install -e ".[dev]"
pytest -q                          # toda la suite
pytest tests/test_conventions.py   # un archivo
pytest tests/test_redaction.py::test_nombre -q   # un test

# Ingestion Gateway (desde agentlens/ingestion/)
go test -race ./...
go run ./cmd/gateway               # requiere AGENTLENS_GATEWAY_API_KEYS="key:tenant[:plan]"
docker build -t agentlens/ingestion-gateway .

# Trace API (desde agentlens/api/)
go test -race ./...                # +integración: AGENTLENS_TEST_CLICKHOUSE=http://127.0.0.1:8123
go run ./cmd/api                   # requiere AGENTLENS_API_KEYS="key:tenant"

# Stack local (desde agentlens/deploy/): Collector OTLP (:4317) + ClickHouse (:8123/:9000)
docker compose up -d

# Ejemplo end-to-end (sin Docker: AGENTLENS_CONSOLE=1)
python agentlens/examples/simple_agent.py
```

Gate de latencia (E0-T03): el SDK tiene un benchmark en CI que rompe el build si el overhead por span supera 5 ms (medido p99 ≈ 0,1 ms). No introducir trabajo síncrono en el hot path de export.

## Arquitectura

Flujo de datos:

```
Agente + SDK (Python) --OTLP/gRPC + TLS + x-agentlens-key--> Ingestion Gateway (Go)
    --OTLP/gRPC--> OTel Collector (redacción, 2ª barrera) --> ClickHouse
                                       Dashboard --HTTP + X-AgentLens-Key--> Trace API (Go) --HTTP--> ClickHouse
```

### sdk-python — puntos clave

- `instrument.py`: API pública. `instrument()` monta el TracerProvider, activa auto-instrumentadores best-effort (OpenAI, LangChain; si no están instalados se omiten sin fallar) y expone helpers manuales `agent()`/`tool()` (spans `invoke_agent`/`execute_tool`).
- `conventions.py` (E1-T09) es el **único** módulo que conoce los nombres concretos de las claves OTel GenAI (el semconv está en estado *Development* y cambia entre versiones). El resto del SDK usa las constantes canónicas `GenAI.*`; `normalize_attributes()` mapea alias legacy (`llm.*`, `ai.*`, versiones previas de `gen_ai.*`) al esquema canónico. Si cambia el semconv, solo se toca este módulo. La versión del contrato es `CONVENTIONS_VERSION` y se publica como atributo de Resource.
- `processors.py`: `AgentLensSpanExporter` envuelve al exporter OTLP y aplica por span, en este orden: (1) normalización de convenciones, (2) redacción de PII (`redaction.py`), (3) externalización de payloads grandes (`payloads.py`, deja una ref `agentlens://payload/...`). Los atributos de un span finalizado son inmutables, así que reconstruye un `ReadableSpan` nuevo en lugar de mutar.
- Configuración por argumentos de `instrument()` o variables `AGENTLENS_*` (`config.py`).

### ingestion — puntos clave

- Implementa el `TraceService` OTLP estándar, por lo que cualquier exportador OTLP habla con él sin cambios.
- `internal/auth`: interceptor gRPC que valida la cabecera `x-agentlens-key` y resuelve `key → Tenant{ID, Plan}`. `StaticKeyStore` (claves en memoria vía `AGENTLENS_GATEWAY_API_KEYS`, formato `key:tenant[:plan]`) es solo MVP; la interfaz `auth.KeyStore` es el punto de extensión para Postgres/Redis sin tocar interceptor ni servidor.
- `internal/ratelimit` (E2-T06): token bucket por tenant con caudal fijado por plan; el coste es el nº de **spans**, no de RPCs. Plan desconocido cae a `starter`; batch mayor que el burst pasa pero vacía el bucket. El servidor responde `ResourceExhausted` (reintentable con backoff por los exportadores OTLP → throttling sin pérdida). La interfaz `gateway.RateLimiter` permite un limitador distribuido (Redis).
- `internal/gateway`: **sella** el atributo de Resource `agentlens.tenant.id` con el tenant autoritativo de la clave, sobreescribiendo lo que declare el cliente (anti-spoofing multi-tenant). Cualquier cambio aquí debe preservar esa garantía; hay tests end-to-end sobre gRPC real (`bufconn`) que la cubren.
- `internal/forward`: reenvío OTLP al Collector downstream. Config por variables `AGENTLENS_GATEWAY_*` (ver `cmd/gateway/main.go`).

### deploy/clickhouse — esquema explícito (E2-T07)

- El exporter del Collector NO crea tablas (`create_schema: false`): el DDL versionado de `deploy/clickhouse/schema/` es la única fuente de verdad. Archivos aplicados no se editan; los cambios van en un archivo nuevo numerado e idempotente.
- `otel_traces` replica exactamente las columnas del INSERT del exporter (v0.110.0 fijada en el compose) y añade `TenantId`/`AgentId` como `MATERIALIZED` (el exporter no las conoce) con `ORDER BY` que empieza por tenant. Si se sube la versión del Collector, revisar `exporter_traces.go` del exporter.
- El listado de trazas se consulta SIEMPRE vía la vista `agentlens.traces` (re-agrega los parciales de `trace_summaries`, que pueden no estar fusionados por `AggregatingMergeTree`).

### api — puntos clave

- Trace API de lectura (E3-T01), solo stdlib. Habla con ClickHouse por su interfaz **HTTP** con parámetros tipados del servidor (`{name:Type}` + `param_name`): ningún valor del cliente se interpola en SQL.
- Auth por cabecera `X-AgentLens-Key` (misma key que la ingesta); toda consulta queda acotada al tenant de la key y una traza ajena responde 404 (no se revela existencia).
- Paginación por cursor opaco base64 `(start_unix_nano, trace_id)` descendente; el handler pide `limit+1` para detectar si hay página siguiente. El detalle poda por tiempo con `otel_traces_trace_id_ts` antes de leer `otel_traces`.
- `internal/store` define la interfaz; `internal/chstore` la implementa; los tests de `httpapi` usan un fake y los de `chstore` son de integración (se saltan sin `AGENTLENS_TEST_CLICKHOUSE`).

### Convivencia de puertos en local

El Collector del docker-compose ocupa el 4317. Para correr el gateway a la vez: `AGENTLENS_GATEWAY_LISTEN=:4319` y `AGENTLENS_GATEWAY_DOWNSTREAM=localhost:4317`, y apuntar el SDK a `endpoint="http://localhost:4319"`.

### Defensa en profundidad de PII

La redacción ocurre dos veces por diseño: en cliente (SDK, antes de salir del proceso — GDPR Art. 5) y en el Collector (`deploy/otel-collector-config.yaml`) como segunda barrera. No eliminar una asumiendo que la otra basta.
