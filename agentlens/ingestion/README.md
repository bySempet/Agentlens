# Ingestion Gateway (Go) — E2-T05 / E2-T06

Punto de entrada autenticado del plano de ingesta de AgentLens. Se sitúa entre el
SDK y el procesamiento/almacenamiento:

```
SDK  --OTLP/gRPC + TLS + API key-->  Ingestion Gateway  --OTLP/gRPC-->  Collector
```

Responsabilidades (E2-T05):

- **OTLP/gRPC**: implementa el `TraceService` de OpenTelemetry, así que cualquier
  exportador OTLP estándar (incluido el SDK de AgentLens) habla con él sin cambios.
- **Auth por API key**: cada petición debe traer la cabecera `x-agentlens-key`
  (la misma que emite el SDK). Las claves inválidas se rechazan con
  `Unauthenticated`; las válidas se enrutan al downstream.
- **Resolución de tenant**: cada API key mapea a un tenant. El gateway **sella**
  el atributo de Resource `agentlens.tenant.id` con el tenant autoritativo,
  sobreescribiendo lo que declare el cliente (anti-spoofing multi-tenant).
- **Rate limiting por plan (E2-T06)**: cada tenant tiene un plan (`free`,
  `starter`, `growth`, `enterprise`) con un caudal sostenido y una ráfaga. Un
  token-bucket por tenant rechaza el exceso con `ResourceExhausted`. Sin
  dependencias externas y seguro para concurrencia.
- **TLS en el borde**: el canal cliente→gateway se cifra con TLS; el canal
  interno gateway→collector usa la red privada.

## Configuración (variables de entorno)

| Variable | Default | Descripción |
| --- | --- | --- |
| `AGENTLENS_GATEWAY_LISTEN` | `:4317` | Dirección de escucha OTLP/gRPC |
| `AGENTLENS_GATEWAY_DOWNSTREAM` | `localhost:5317` | Endpoint OTLP del Collector |
| `AGENTLENS_GATEWAY_TLS_CERT` | — | Ruta al certificado TLS (habilita TLS) |
| `AGENTLENS_GATEWAY_TLS_KEY` | — | Ruta a la clave privada TLS |
| `AGENTLENS_GATEWAY_API_KEYS` | — | Pares `key:tenant` separados por coma |
| `AGENTLENS_GATEWAY_TENANT_PLANS` | — | Pares `tenant:plan` separados por coma |
| `AGENTLENS_GATEWAY_DEFAULT_PLAN` | `free` | Plan para tenants no listados |

> El `StaticKeyStore` (claves en memoria) es para el MVP, tests y despliegues
> air-gapped. La interfaz `auth.KeyStore` permite enchufar Postgres/Redis sin
> tocar el interceptor ni el servidor.

## Ejecutar localmente

```bash
# El downstream apunta al Collector del docker-compose (deploy/), que escucha en
# 4317; aquí movemos el gateway a otro puerto para no colisionar.
export AGENTLENS_GATEWAY_LISTEN=:4319
export AGENTLENS_GATEWAY_DOWNSTREAM=localhost:4317
export AGENTLENS_GATEWAY_API_KEYS="demo-key:acme-corp"
export AGENTLENS_GATEWAY_TENANT_PLANS="acme-corp:growth"   # opcional; def. free
go run ./cmd/gateway
```

Y apunta el SDK al gateway:

```python
agentlens.instrument(api_key="demo-key", endpoint="http://localhost:4319",
                     tenant_id="acme-corp", agent_id="support-bot")
```

## Tests

```bash
go test ./...
```

Cubren: aceptación/rechazo de claves (incl. ausente y vacía), inyección del
tenant en el contexto, enrutado end-to-end sobre un servidor gRPC real
(`bufconn`), anti-spoofing del tenant, límites por plan (ráfaga, recarga,
aislamiento por tenant) y un test de carga concurrente (ejecutar con `-race`):

```bash
go test -race ./...
```

## Build de la imagen

```bash
docker build -t agentlens/ingestion-gateway .
```
