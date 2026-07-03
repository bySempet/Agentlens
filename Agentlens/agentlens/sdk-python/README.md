# AgentLens SDK (Python)

Gobernanza y observabilidad para agentes de IA. Instrumentación basada en
OpenTelemetry GenAI, agnóstica al framework y al proveedor de LLM, con
**redacción de PII en cliente** y **externalización de payloads** integradas.

## Integración en 3 líneas

```python
import agentlens
agentlens.instrument(api_key="...", tenant_id="acme", agent_id="support-bot")
# A partir de aquí, OpenAI / LangChain / CrewAI se auto-instrumentan.
```

## Qué hace por defecto

- **Camino 100% asíncrono.** El agente nunca espera al backend. Overhead medido
  p99 ≈ 0,1 ms por span (gate de CI: < 5 ms).
- **Redacción de PII en cliente.** Email, IBAN, tarjeta y teléfono se mascaran
  *antes* de salir del proceso (minimización de datos, GDPR Art. 5). Extensible
  con patrones propios.
- **Externalización de payloads.** Prompts y outputs grandes salen del span y
  dejan una referencia (`agentlens://payload/...`), resolviendo coste,
  privacidad y retención a la vez.
- **Enriquecimiento.** Todos los spans heredan `tenant.id`, `agent.id` y
  `service.name` vía Resource.
- **Capa adaptadora de convenciones.** El esquema OTel GenAI está en estado
  *Development* y cambia entre versiones. AgentLens normaliza alias legacy
  (`llm.*`, `ai.*`, `gen_ai.usage.prompt_tokens`, …) a un esquema canónico
  estable, de modo que un cambio de semconv no rompe el contrato del SDK. Las
  claves se centralizan en `agentlens.GenAI` y la versión del contrato se publica
  en `agentlens.conventions.version`.

## Instrumentación manual (sin framework)

```python
with agentlens.agent("support-bot"):          # span invoke_agent
    with agentlens.tool("lookup_customer") as s:  # span execute_tool
        s.set_attribute("gen_ai.tool.name", "lookup_customer")
```

## Configuración

Por argumentos a `instrument()` o por entorno (`AGENTLENS_*`):

| Opción | Default | Descripción |
| --- | --- | --- |
| `endpoint` | `http://localhost:4317` | OTLP/gRPC del Collector |
| `redact_pii` | `True` | Redacción de PII en cliente |
| `payload_mode` | `reference` | `reference` \| `inline` \| `none` |
| `payload_threshold_bytes` | `4096` | Tamaño a partir del cual se externaliza |
| `flush_interval_ms` | `100` | Periodo de vaciado del batch |
| `console` | `False` | Exporta a consola (dev sin Collector) |

## Probarlo localmente

```bash
# 1. Levantar el stack (Collector + ClickHouse)
cd ../deploy && docker compose up -d

# 2. Ejecutar el agente de ejemplo
cd ../examples && python simple_agent.py

# 3. Consultar las trazas en ClickHouse
#    Listado:  SELECT * FROM agentlens.traces ORDER BY StartTime DESC LIMIT 20;
#    Spans:    SELECT * FROM agentlens.otel_traces ORDER BY Timestamp DESC LIMIT 20;
```

Sin Docker: `AGENTLENS_CONSOLE=1 python examples/simple_agent.py`.

## Tests

```bash
pip install -e ".[dev]"
pytest -q
```
