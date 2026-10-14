// Pipeline de transformación de spans antes de exportar (paridad con
// sdk-python/processors.py). Orden: normalización de convenciones -> redacción
// de PII -> externalización de payloads.

import type { Attributes } from "@opentelemetry/api";
import type { ReadableSpan, SpanExporter } from "@opentelemetry/sdk-trace-base";
import { ExportResultCode, type ExportResult } from "@opentelemetry/core";

import type { AgentLensConfig } from "./config.js";
import { normalizeAttributes } from "./conventions.js";
import {
  InMemoryPayloadStore,
  NoopPayloadStore,
  PayloadStore,
  discardContent,
  externalizeAttributes,
} from "./payloads.js";
import { Redactor } from "./redaction.js";

/** Spans transformados por tick del event loop (ver AgentLensSpanExporter.export). */
const TRANSFORM_CHUNK = 64;

/** Envuelve un span exponiendo unos atributos transformados, preservando el resto. */
function spanWithAttributes(span: ReadableSpan, attributes: Attributes): ReadableSpan {
  return new Proxy(span, {
    get(target, prop, receiver) {
      if (prop === "attributes") return attributes;
      const value = Reflect.get(target, prop, receiver);
      return typeof value === "function" ? value.bind(target) : value;
    },
  });
}

export class AgentLensSpanExporter implements SpanExporter {
  private readonly redactor?: Redactor;
  private readonly store: PayloadStore;

  constructor(
    private readonly inner: SpanExporter,
    private readonly config: AgentLensConfig,
    opts: { redactor?: Redactor; store?: PayloadStore } = {},
  ) {
    this.redactor = opts.redactor ?? (config.redactPii ? new Redactor() : undefined);
    this.store =
      opts.store ??
      (config.payloadMode === "reference" ? new InMemoryPayloadStore() : new NoopPayloadStore());
  }

  private transform(span: ReadableSpan): ReadableSpan {
    let attrs: Attributes = normalizeAttributes(span.attributes);
    if (this.redactor) {
      attrs = this.redactor.redactAttributes(attrs);
    }
    if (this.config.payloadMode === "reference") {
      attrs = externalizeAttributes(
        attrs,
        this.store,
        this.config.contentAttrs,
        this.config.payloadThresholdBytes,
      );
    } else if (this.config.payloadMode === "none") {
      attrs = discardContent(attrs, this.config.contentAttrs);
    }
    return spanWithAttributes(span, attrs);
  }

  /**
   * El BatchSpanProcessor de OTel JS llama a export() de forma síncrona dentro de
   * span.end() cuando se llena un lote: transformar ahí (redacción + payloads)
   * cargaría decenas de ms sobre la acción del agente (E0-T03). Por eso la
   * transformación se difiere fuera del hot path y se hace por trozos, cediendo
   * el event loop entre ellos.
   */
  export(spans: ReadableSpan[], resultCallback: (result: ExportResult) => void): void {
    const out: ReadableSpan[] = [];
    const step = (from: number) => {
      try {
        const to = Math.min(from + TRANSFORM_CHUNK, spans.length);
        for (let i = from; i < to; i++) out.push(this.transform(spans[i]));
        if (to < spans.length) {
          setImmediate(step, to);
        } else {
          this.inner.export(out, resultCallback);
        }
      } catch (error) {
        resultCallback({ code: ExportResultCode.FAILED, error: error as Error });
      }
    };
    setImmediate(step, 0);
  }

  shutdown(): Promise<void> {
    return this.inner.shutdown();
  }

  forceFlush(): Promise<void> {
    return this.inner.forceFlush?.() ?? Promise.resolve();
  }
}
