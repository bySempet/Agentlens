# AgentLens SDK (Node / TypeScript) — `@agentlens/node`

Paridad funcional con el SDK de Python (E1-T11): instrumentación OpenTelemetry
GenAI con **redacción de PII en cliente**, **externalización de payloads** y la
**capa adaptadora de convenciones**, agnóstica al framework y al proveedor de LLM.

## Integración en 3 líneas

```ts
import { instrument, agent, tool } from "@agentlens/node";

instrument({ apiKey: "...", tenantId: "acme", agentId: "support-bot" });

agent("support-bot", () => {
  tool("lookup_customer", (span) => {
    span.setAttribute("gen_ai.tool.name", "lookup_customer");
  });
});
```

`agent()`/`tool()` soportan callbacks **síncronos y asíncronos** (el span se
cierra cuando la promesa resuelve).

## Qué hace por defecto (paridad con Python)

- **Camino asíncrono** vía `BatchSpanProcessor` (el agente no espera al backend).
- **Redacción de PII en cliente**: email, IBAN, tarjeta, teléfono; extensible con
  `Redactor.addPattern()`.
- **Externalización de payloads**: prompts/outputs grandes salen del span y dejan
  una referencia `agentlens://payload/...` (modos `reference` | `inline` | `none`).
- **Enriquecimiento** por Resource: `tenant.id`, `agent.id`, `service.name` y
  `conventions.version`.
- **Capa de convenciones**: normaliza alias legacy (`llm.*`, `ai.*`, versiones
  previas de `gen_ai.*`) a un esquema canónico estable.

## Configuración

Por opciones a `instrument()` o por entorno (`AGENTLENS_*`), igual que en Python:
`endpoint` (def. `http://localhost:4317`), `redactPii`, `payloadMode`,
`payloadThresholdBytes`, `flushIntervalMs`, `console`.

## Desarrollo

```bash
npm install
npm run typecheck   # tsc (src + tests)
npm test            # node:test vía tsx
npm run build       # emite dist/ (ESM + .d.ts)
```

Los tests cubren la normalización de convenciones, la redacción y el flujo
end-to-end (enriquecimiento + redacción + externalización + convención) con un
`InMemorySpanExporter`, además del cierre de spans en callbacks async.
