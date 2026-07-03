# Integración con AgentLens en 5 pasos

Esta guía es reproducible por alguien externo: en menos de 10 minutos verás la
traza de un agente de extremo a extremo (SDK → Collector → ClickHouse → API →
dashboard). Hay variante **Python** y **Node/TypeScript**.

## Paso 1 — Levanta el stack local

```bash
git clone https://github.com/bysempet/agentlens
cd agentlens/deploy
docker compose up -d        # OTel Collector (OTLP :4317) + ClickHouse
```

El Collector recibe OTLP y escribe en el esquema ClickHouse de AgentLens
(`deploy/clickhouse/init.sql`).

## Paso 2 — Instala el SDK

**Python**
```bash
pip install agentlens
```

**Node**
```bash
npm install @agentlens/node
```

## Paso 3 — Instrumenta tu agente (3 líneas)

**Python**
```python
import agentlens
agentlens.instrument(tenant_id="acme", agent_id="support-bot")
# A partir de aquí, OpenAI / LangChain / CrewAI / AutoGen / Pydantic AI / Bedrock
# se auto-instrumentan si están instalados.
```

**Node**
```ts
import { instrument } from "@agentlens/node";
instrument({ tenantId: "acme", agentId: "support-bot" });
```

> Por defecto el SDK exporta a `http://localhost:4317` (el Collector del paso 1),
> redacta PII en cliente y externaliza payloads grandes.

## Paso 4 — Ejecuta el agente

Si no usas un framework auto-instrumentado, traza a mano con los helpers:

**Python**
```python
with agentlens.agent("support-bot"):
    with agentlens.tool("lookup_customer") as span:
        span.set_attribute("gen_ai.tool.name", "lookup_customer")
agentlens.shutdown()
```

**Node**
```ts
import { agent, tool, shutdown } from "@agentlens/node";
agent("support-bot", () => {
  tool("lookup_customer", (span) => {
    span.setAttribute("gen_ai.tool.name", "lookup_customer");
  });
});
await shutdown();
```

¿Sin entorno propio? Usa el ejemplo incluido:
```bash
cd examples && AGENTLENS_CONSOLE=1 python simple_agent.py
```

## Paso 5 — Mira las trazas en el dashboard

Arranca la API de lectura y el dashboard:

```bash
# API hot path (lee de ClickHouse)
cd api
AGENTLENS_API_KEYS="demo-key:acme" go run ./cmd/api      # :8080

# Dashboard
cd ../frontend
AGENTLENS_API_URL=http://localhost:8080 AGENTLENS_API_KEY=demo-key npm run dev
```

Abre <http://localhost:3000>: verás la lista de trazas y, al entrar en una, el
**timeline de spans** paso a paso del agente.

---

## Configuración útil

| Variable (`AGENTLENS_*`) | Efecto |
| --- | --- |
| `ENDPOINT` | Endpoint OTLP (def. `http://localhost:4317`) |
| `API_KEY` | API key del tenant (auth en gateway/API) |
| `REDACT_PII` | Activa/desactiva la redacción en cliente (def. on) |
| `PAYLOAD_MODE` | `reference` \| `inline` \| `none` |
| `CONSOLE` | Exporta a consola (dev sin Collector) |

## Siguiente

- Pon delante el **Ingestion Gateway** (`ingestion/`) para auth por API key,
  resolución de tenant y rate limiting por plan.
- Contrato de la API: `api/spec/openapi.yaml` (servido en `/openapi.yaml`).
