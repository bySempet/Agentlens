// API pública de @agentlens/node.
export { instrument, agent, tool, getTracer, shutdown } from "./instrument.js";
export type { InstrumentOptions } from "./instrument.js";
export {
  GenAI,
  CONVENTIONS_VERSION,
  CONTENT_ATTRS,
  canonicalKey,
  normalizeAttributes,
} from "./conventions.js";
export { Redactor } from "./redaction.js";
export {
  NoopPayloadStore,
  InMemoryPayloadStore,
  externalizeAttributes,
  discardContent,
} from "./payloads.js";
export type { PayloadStore } from "./payloads.js";
export { resolveConfig } from "./config.js";
export type { AgentLensOptions, AgentLensConfig, PayloadMode } from "./config.js";

export const VERSION = "0.1.0";
