// API pública del SDK (paridad con sdk-python/instrument.py).
//
// Uso mínimo (las "3 líneas"):
//
//   import { instrument } from "@agentlens/node";
//   instrument({ apiKey: "...", tenantId: "acme", agentId: "support-bot" });
//   // ... el código del agente se traza con los helpers agent()/tool()

import { Span, Tracer, trace } from "@opentelemetry/api";
import { Resource } from "@opentelemetry/resources";
import {
  BatchSpanProcessor,
  ConsoleSpanExporter,
  SpanExporter,
} from "@opentelemetry/sdk-trace-base";
import { NodeTracerProvider } from "@opentelemetry/sdk-trace-node";
import { OTLPTraceExporter } from "@opentelemetry/exporter-trace-otlp-grpc";

import { AgentLensConfig, AgentLensOptions, resolveConfig } from "./config.js";
import { CONVENTIONS_VERSION, GenAI } from "./conventions.js";
import { AgentLensSpanExporter } from "./processors.js";
import { Redactor } from "./redaction.js";

export interface InstrumentOptions extends AgentLensOptions {
  /** Exporter propio (p.ej. InMemory en tests); ignora el endpoint OTLP. */
  exporter?: SpanExporter;
  /** Redactor propio (patrones extendidos). */
  redactor?: Redactor;
}

let provider: NodeTracerProvider | undefined;

function buildResource(cfg: AgentLensConfig): Resource {
  return new Resource({
    "service.name": cfg.serviceName,
    "deployment.environment": cfg.environment,
    "agentlens.tenant.id": cfg.tenantId ?? "unknown",
    "agentlens.agent.id": cfg.agentId,
    "agentlens.sdk.language": "node",
    "agentlens.conventions.version": CONVENTIONS_VERSION,
  });
}

function buildInnerExporter(cfg: AgentLensConfig): SpanExporter {
  if (cfg.console) {
    return new ConsoleSpanExporter();
  }
  const headers = cfg.apiKey ? { "x-agentlens-key": cfg.apiKey } : undefined;
  return new OTLPTraceExporter({ url: cfg.endpoint, headers });
}

/** Configura AgentLens y devuelve el TracerProvider (útil en tests). */
export function instrument(options: InstrumentOptions = {}): NodeTracerProvider {
  const cfg = resolveConfig(options);
  const inner = options.exporter ?? buildInnerExporter(cfg);
  const wrapped = new AgentLensSpanExporter(inner, cfg, { redactor: options.redactor });

  const p = new NodeTracerProvider({ resource: buildResource(cfg) });
  // Camino 100% asíncrono: el agente nunca espera al backend.
  p.addSpanProcessor(
    new BatchSpanProcessor(wrapped, {
      scheduledDelayMillis: cfg.flushIntervalMs,
      maxQueueSize: cfg.maxQueueSize,
      maxExportBatchSize: cfg.maxExportBatchSize,
    }),
  );

  // Permite re-inicialización idempotente (sobre todo en tests).
  trace.disable();
  p.register();
  provider = p;
  return p;
}

export function getTracer(name = "agentlens"): Tracer {
  return trace.getTracer(name);
}

function endAfter<T>(span: Span, fn: () => T): T {
  let result: T;
  try {
    result = fn();
  } catch (err) {
    span.end();
    throw err;
  }
  if (result instanceof Promise) {
    return result.finally(() => span.end()) as unknown as T;
  }
  span.end();
  return result;
}

/** Span `invoke_agent` (convención OTel GenAI). Soporta callbacks sync y async. */
export function agent<T>(
  name: string,
  fn: (span: Span) => T,
  opts: { agentId?: string } = {},
): T {
  return getTracer().startActiveSpan(`${GenAI.OP_INVOKE_AGENT} ${name}`, (span) => {
    span.setAttribute(GenAI.OPERATION_NAME, GenAI.OP_INVOKE_AGENT);
    span.setAttribute(GenAI.AGENT_NAME, name);
    if (opts.agentId) span.setAttribute(GenAI.AGENT_ID, opts.agentId);
    return endAfter(span, () => fn(span));
  });
}

/** Span `execute_tool` (convención OTel GenAI). */
export function tool<T>(name: string, fn: (span: Span) => T): T {
  return getTracer().startActiveSpan(`${GenAI.OP_EXECUTE_TOOL} ${name}`, (span) => {
    span.setAttribute(GenAI.OPERATION_NAME, GenAI.OP_EXECUTE_TOOL);
    span.setAttribute(GenAI.TOOL_NAME, name);
    return endAfter(span, () => fn(span));
  });
}

/** Vacía y cierra el provider (importante al final de procesos cortos). */
export async function shutdown(): Promise<void> {
  if (provider) {
    await provider.shutdown();
  }
}
