// Pipeline de transformación de spans antes de exportar (paridad con
// sdk-python/processors.py). Orden: normalización de convenciones -> redacción
// de PII -> externalización de payloads.

import type { Attributes } from "@opentelemetry/api";
import type { ReadableSpan, SpanExporter } from "@opentelemetry/sdk-trace-base";
import type { ExportResult } from "@opentelemetry/core";

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

  export(spans: ReadableSpan[], resultCallback: (result: ExportResult) => void): void {
    this.inner.export(spans.map((s) => this.transform(s)), resultCallback);
  }

  shutdown(): Promise<void> {
    return this.inner.shutdown();
  }

  forceFlush(): Promise<void> {
    return this.inner.forceFlush?.() ?? Promise.resolve();
  }
}
