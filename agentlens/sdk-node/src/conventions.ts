// Capa adaptadora de convenciones (paridad con sdk-python/conventions.py, E1-T09).
// Único punto que conoce los nombres concretos del esquema OTel GenAI; el resto
// del SDK trabaja contra estas constantes canónicas.

import type { Attributes } from "@opentelemetry/api";

export const CONVENTIONS_VERSION = "genai-1.0";

export const GenAI = {
  OPERATION_NAME: "gen_ai.operation.name",
  SYSTEM: "gen_ai.system",
  AGENT_NAME: "gen_ai.agent.name",
  AGENT_ID: "gen_ai.agent.id",
  TOOL_NAME: "gen_ai.tool.name",
  TOOL_CALL_ID: "gen_ai.tool.call.id",
  REQUEST_MODEL: "gen_ai.request.model",
  RESPONSE_MODEL: "gen_ai.response.model",
  USAGE_INPUT_TOKENS: "gen_ai.usage.input_tokens",
  USAGE_OUTPUT_TOKENS: "gen_ai.usage.output_tokens",
  INPUT_MESSAGES: "gen_ai.input.messages",
  OUTPUT_MESSAGES: "gen_ai.output.messages",
  SYSTEM_INSTRUCTIONS: "gen_ai.system_instructions",
  TOOL_CALL_ARGUMENTS: "gen_ai.tool.call.arguments",
  TOOL_CALL_RESULT: "gen_ai.tool.call.result",
  OP_INVOKE_AGENT: "invoke_agent",
  OP_EXECUTE_TOOL: "execute_tool",
  OP_CHAT: "chat",
} as const;

// Atributos que contienen "contenido grande" (candidatos a redacción/externalización).
export const CONTENT_ATTRS: string[] = [
  GenAI.INPUT_MESSAGES,
  GenAI.OUTPUT_MESSAGES,
  GenAI.SYSTEM_INSTRUCTIONS,
  GenAI.TOOL_CALL_ARGUMENTS,
  GenAI.TOOL_CALL_RESULT,
  "gen_ai.prompt",
  "gen_ai.completion",
];

// Alias legacy/variantes -> clave canónica (versiones previas, llm.*, ai.*).
const ALIAS_MAP: Record<string, string> = {
  "gen_ai.usage.prompt_tokens": GenAI.USAGE_INPUT_TOKENS,
  "gen_ai.usage.completion_tokens": GenAI.USAGE_OUTPUT_TOKENS,
  "llm.usage.prompt_tokens": GenAI.USAGE_INPUT_TOKENS,
  "llm.usage.completion_tokens": GenAI.USAGE_OUTPUT_TOKENS,
  "llm.token_count.prompt": GenAI.USAGE_INPUT_TOKENS,
  "llm.token_count.completion": GenAI.USAGE_OUTPUT_TOKENS,
  "ai.usage.promptTokens": GenAI.USAGE_INPUT_TOKENS,
  "ai.usage.completionTokens": GenAI.USAGE_OUTPUT_TOKENS,
  "llm.request.model": GenAI.REQUEST_MODEL,
  "llm.response.model": GenAI.RESPONSE_MODEL,
  "ai.model.id": GenAI.REQUEST_MODEL,
  "llm.system": GenAI.SYSTEM,
  "llm.vendor": GenAI.SYSTEM,
  "llm.prompts": GenAI.INPUT_MESSAGES,
  "llm.completions": GenAI.OUTPUT_MESSAGES,
  "gen_ai.prompt": GenAI.INPUT_MESSAGES,
  "gen_ai.completion": GenAI.OUTPUT_MESSAGES,
  "ai.prompt": GenAI.INPUT_MESSAGES,
  "ai.response.text": GenAI.OUTPUT_MESSAGES,
  "llm.tool.name": GenAI.TOOL_NAME,
  "tool.name": GenAI.TOOL_NAME,
};

export function canonicalKey(key: string): string {
  return ALIAS_MAP[key] ?? key;
}

/** Reescribe atributos al esquema canónico; la clave canónica gana sobre el alias. */
export function normalizeAttributes(attributes: Attributes): Attributes {
  const out: Attributes = {};
  for (const [key, value] of Object.entries(attributes)) {
    const target = ALIAS_MAP[key] ?? key;
    if (target !== key && target in attributes) {
      continue; // canónica ya presente: descartamos el alias redundante
    }
    out[target] = value;
  }
  return out;
}
