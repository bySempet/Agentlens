// Extrae una conversación legible (chat-style) a partir de los atributos GenAI
// de los spans (E3-T04). Tolera distintos formatos del contenido gen_ai.* y
// degrada con elegancia: contenido vacío, redactado o externalizado.
import type { Span } from "./types";

export type TurnKind = "system" | "user" | "assistant" | "tool";

export interface ChatTurn {
  kind: TurnKind;
  role: string;
  text: string;
  spanId: string;
}

const REFERENCE_PREFIX = "agentlens://payload/";

function isReference(s: string): boolean {
  return s.startsWith(REFERENCE_PREFIX);
}

/** Extrae el texto de un mensaje en los formatos habituales de OTel GenAI. */
function messageText(msg: unknown): string {
  if (typeof msg === "string") return msg;
  if (msg && typeof msg === "object") {
    const m = msg as Record<string, unknown>;
    if (typeof m.content === "string") return m.content;
    // content como lista de partes [{type:'text', text:'...'}] o [{content:'...'}]
    const parts = (m.content ?? m.parts) as unknown;
    if (Array.isArray(parts)) {
      return parts
        .map((p) => {
          if (typeof p === "string") return p;
          const po = p as Record<string, unknown>;
          return (po.text as string) ?? (po.content as string) ?? "";
        })
        .filter(Boolean)
        .join("\n");
    }
  }
  return "";
}

function roleToKind(role: string, fallback: TurnKind): TurnKind {
  const r = role.toLowerCase();
  if (r === "system") return "system";
  if (r === "user" || r === "human") return "user";
  if (r === "assistant" || r === "ai" || r === "model") return "assistant";
  if (r === "tool" || r === "function") return "tool";
  return fallback;
}

/** Parsea un atributo de mensajes (JSON array, objeto o texto plano). */
function parseMessages(raw: string | undefined, fallbackKind: TurnKind, spanId: string): ChatTurn[] {
  if (!raw) return [];
  if (isReference(raw)) {
    return [{ kind: fallbackKind, role: fallbackKind, text: "[contenido externalizado]", spanId }];
  }
  try {
    const data = JSON.parse(raw);
    const arr = Array.isArray(data) ? data : [data];
    return arr
      .map((m): ChatTurn => {
        const role = (m && typeof m === "object" && (m as any).role) || fallbackKind;
        return { kind: roleToKind(String(role), fallbackKind), role: String(role), text: messageText(m), spanId };
      })
      .filter((t) => t.text);
  } catch {
    return [{ kind: fallbackKind, role: fallbackKind, text: raw, spanId }];
  }
}

/** Construye la conversación recorriendo los spans en orden temporal. */
export function buildConversation(spans: Span[]): ChatTurn[] {
  const ordered = spans
    .slice()
    .sort((a, b) => new Date(a.start_time).getTime() - new Date(b.start_time).getTime());

  const turns: ChatTurn[] = [];
  for (const s of ordered) {
    if (s.system_instructions) {
      turns.push(...parseMessages(s.system_instructions, "system", s.span_id));
    }
    turns.push(...parseMessages(s.input_messages, "user", s.span_id));
    turns.push(...parseMessages(s.output_messages, "assistant", s.span_id));
    if (s.tool_arguments) {
      turns.push({ kind: "tool", role: `tool · ${s.span_name}`, text: s.tool_arguments, spanId: s.span_id });
    }
    if (s.tool_result) {
      turns.push({ kind: "tool", role: `resultado · ${s.span_name}`, text: s.tool_result, spanId: s.span_id });
    }
  }
  return turns;
}
