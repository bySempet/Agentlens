// Configuración del SDK (paridad con sdk-python/config.py).
import { CONTENT_ATTRS } from "./conventions.js";

export type PayloadMode = "reference" | "inline" | "none";

export interface AgentLensOptions {
  apiKey?: string;
  endpoint?: string;
  tenantId?: string;
  agentId?: string;
  serviceName?: string;
  environment?: string;
  redactPii?: boolean;
  payloadMode?: PayloadMode;
  payloadThresholdBytes?: number;
  contentAttrs?: string[];
  flushIntervalMs?: number;
  maxQueueSize?: number;
  maxExportBatchSize?: number;
  console?: boolean;
}

export interface AgentLensConfig extends Required<Omit<AgentLensOptions, "apiKey" | "tenantId">> {
  apiKey?: string;
  tenantId?: string;
}

function envBool(name: string, def: boolean): boolean {
  const v = process.env[name];
  if (v === undefined) return def;
  return ["1", "true", "yes", "on"].includes(v.trim().toLowerCase());
}

export function resolveConfig(options: AgentLensOptions = {}): AgentLensConfig {
  const cfg: AgentLensConfig = {
    apiKey: options.apiKey ?? process.env.AGENTLENS_API_KEY,
    endpoint: options.endpoint ?? process.env.AGENTLENS_ENDPOINT ?? "http://localhost:4317",
    tenantId: options.tenantId ?? process.env.AGENTLENS_TENANT_ID,
    agentId: options.agentId ?? process.env.AGENTLENS_AGENT_ID ?? "default",
    serviceName: options.serviceName ?? process.env.AGENTLENS_SERVICE_NAME ?? "agentlens-agent",
    environment: options.environment ?? process.env.AGENTLENS_ENV ?? "development",
    redactPii: options.redactPii ?? envBool("AGENTLENS_REDACT_PII", true),
    payloadMode: (options.payloadMode ?? (process.env.AGENTLENS_PAYLOAD_MODE as PayloadMode) ?? "reference"),
    payloadThresholdBytes: options.payloadThresholdBytes ?? 4096,
    contentAttrs: options.contentAttrs ?? CONTENT_ATTRS,
    flushIntervalMs: options.flushIntervalMs ?? 100,
    maxQueueSize: options.maxQueueSize ?? 2048,
    maxExportBatchSize: options.maxExportBatchSize ?? 512,
    console: options.console ?? envBool("AGENTLENS_CONSOLE", false),
  };
  validate(cfg);
  return cfg;
}

function validate(cfg: AgentLensConfig): void {
  if (!["reference", "inline", "none"].includes(cfg.payloadMode)) {
    throw new Error(`payloadMode inválido: ${cfg.payloadMode}`);
  }
  if (cfg.flushIntervalMs <= 0) {
    throw new Error("flushIntervalMs debe ser > 0");
  }
}
