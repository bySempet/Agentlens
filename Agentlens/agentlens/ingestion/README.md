# Ingestion Gateway (Go) — E2-T05 · E2-T06

Punto de entrada autenticado del plano de ingesta de AgentLens. Se sitúa entre el
SDK y el procesamiento/almacenamiento:

```
SDK  --OTLP/gRPC + TLS + API key-->  Ingestion Gateway  --OTLP/gRPC-->  Collector
```

Responsabilidades:

- **OTLP/gRPC**: implementa el `TraceService` de OpenTelemetry, así que cualquier
  exportador OTLP estándar (incluido el SDK de AgentLens) habla con él sin cambios.
- **Auth por API key** (E2-T05): cada petición debe traer la cabecera
  `x-agentlens-key` (la misma que emite el SDK). Las claves inválidas se rechazan
  con `Unauthenticated`; las válidas se enrutan al downstream.
- **Resolución de tenant + plan** (E2-T05/T06): cada API key mapea a un tenant y
  a su plan de servicio. El gateway **sella** el atributo de Resource
  `agentlens.tenant.id` con el tenant autoritativo, sobreescribiendo lo que
  declare el cliente (anti-spoofing multi-tenant).
- **Rate limiting por plan** (E2-T06): token bucket por tenant cuyo caudal fija
  el plan; el coste es el número de **spans** (no de RPCs, para que el tamaño de
  batch no cambie lo que se puede ingerir). Al excederlo responde
  `ResourceExhausted`, que los exportadores OTLP tratan como reintentable con
  backoff: el cliente queda limitado al caudal del plan sin perder datos.
- **TLS en el borde**: el canal cliente→gateway se cifra con TLS; el canal
  interno gateway→collector usa la red privada.

## Configuración (variables de entorno)

| Variable | Default | Descripción |
| --- | --- | --- |
| `AGENTLENS_GATEWAY_LISTEN` | `:4317` | Dirección de escucha OTLP/gRPC |
| `AGENTLENS_GATEWAY_DOWNSTREAM` | `localhost:5317` | Endpoint OTLP del Collector |
| `AGENTLENS_GATEWAY_TLS_CERT` | — | Ruta al certificado TLS (habilita TLS) |
| `AGENTLENS_GATEWAY_TLS_KEY` | — | Ruta a la clave privada TLS |
| `AGENTLENS_GATEWAY_API_KEYS` | — | Entradas `key:tenant[:plan]` separadas por coma (plan por defecto: `starter`) |
| `AGENTLENS_GATEWAY_PLAN_LIMITS` | — | Sobreescribe/añade planes: `plan=spans_s[:burst]` separados por coma; `0` = sin límite. Sin burst explícito se usa 2× el caudal |

Planes por defecto: `starter` 100 spans/s (burst 200) · `pro` 1.000 (burst
2.000) · `enterprise` 10.000 (burst 20.000). Un plan desconocido aplica los
límites de `starter` (una key mal dada de alta no abre caudal infinito).

> El `StaticKeyStore` (claves en memoria) y el limitador en memoria son para el
> MVP, tests y despliegues air-gapped. Las interfaces `auth.KeyStore` y
> `gateway.RateLimiter` permiten enchufar Postgres/Redis (E2-T12) sin tocar el
> interceptor ni el servidor.

## Ejecutar localmente

```bash
# El downstream apunta al Collector del docker-compose (deploy/), que escucha en
# 4317; aquí movemos el gateway a otro puerto para no colisionar.
export AGENTLENS_GATEWAY_LISTEN=:4319
export AGENTLENS_GATEWAY_DOWNSTREAM=localhost:4317
export AGENTLENS_GATEWAY_API_KEYS="demo-key:acme-corp"
go run ./cmd/gateway
```

Y apunta el SDK al gateway:

```python
agentlens.instrument(api_key="demo-key", endpoint="http://localhost:4319",
                     tenant_id="acme-corp", agent_id="support-bot")
```

## Tests

```bash
go test -race ./...
```

Cubren: aceptación/rechazo de claves (incl. ausente y vacía), inyección del
tenant+plan en el contexto, enrutado end-to-end sobre un servidor gRPC real
(`bufconn`), el anti-spoofing del tenant, el rechazo con `ResourceExhausted` al
exceder el plan y un test de carga concurrente del limitador (50 goroutines
compitiendo por el mismo bucket; admite exactamente el burst).

## Build de la imagen

```bash
docker build -t agentlens/ingestion-gateway .
```
