// Externalización de payloads (paridad con sdk-python/payloads.py, E1-T05).
// Los prompts/outputs grandes salen del span y dejan una referencia.

import { createHash, randomUUID } from "node:crypto";
import type { Attributes } from "@opentelemetry/api";

export interface PayloadStore {
  /** Almacena el payload y devuelve una URI de referencia. */
  put(payload: string): string;
}

/** Descarta el payload (modo 'none'). */
export class NoopPayloadStore implements PayloadStore {
  put(): string {
    return "agentlens://payload/discarded";
  }
}

/** Almacén en memoria para desarrollo/tests (sustituible por S3 en prod). */
export class InMemoryPayloadStore implements PayloadStore {
  readonly items = new Map<string, string>();
  put(payload: string): string {
    const id = randomUUID().replace(/-/g, "");
    this.items.set(id, payload);
    return `agentlens://payload/${id}`;
  }
}

function byteLength(s: string): number {
  return Buffer.byteLength(s, "utf8");
}

/** Externaliza los atributos de contenido que superen el umbral. */
export function externalizeAttributes(
  attributes: Attributes,
  store: PayloadStore,
  contentAttrs: string[],
  thresholdBytes: number,
): Attributes {
  const out: Attributes = { ...attributes };
  const externalized: string[] = [];
  for (const key of contentAttrs) {
    const value = out[key];
    if (typeof value === "string" && byteLength(value) > thresholdBytes) {
      out[key] = store.put(value);
      out[`agentlens.payload.${key}.sha256`] = createHash("sha256").update(value).digest("hex");
      externalized.push(key);
    }
  }
  if (externalized.length > 0) {
    out["agentlens.payload.externalized"] = externalized.join(",");
  }
  return out;
}

/** Modo 'none': elimina por completo los atributos de contenido. */
export function discardContent(attributes: Attributes, contentAttrs: string[]): Attributes {
  const out: Attributes = { ...attributes };
  for (const key of contentAttrs) {
    delete out[key];
  }
  return out;
}
