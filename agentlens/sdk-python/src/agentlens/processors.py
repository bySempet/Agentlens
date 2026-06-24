"""Pipeline de transformación de spans antes de exportar.

``AgentLensSpanExporter`` envuelve al exporter real (OTLP) y, por cada span,
aplica en este orden:

  1. Normalización de convenciones (alias legacy/variantes -> esquema canónico)
  2. Redacción de PII   (para que no llegue ni al almacén de payloads)
  3. Externalización de payloads grandes

Como los atributos de un span finalizado son inmutables (``BoundedAttributes``
lanza ``TypeError`` al asignar), reconstruimos un ``ReadableSpan`` nuevo con los
atributos transformados. Este enfoque es independiente del orden de los
procesadores y compatible con el exporter OTLP real (verificado).
"""
from __future__ import annotations

from typing import Optional, Sequence

from opentelemetry.sdk.trace import ReadableSpan
from opentelemetry.sdk.trace.export import SpanExporter, SpanExportResult

from .config import AgentLensConfig
from .conventions import normalize_attributes
from .payloads import (
    LocalFilePayloadStore,
    NoopPayloadStore,
    PayloadStore,
    discard_content,
    externalize_attributes,
)
from .redaction import Redactor


class AgentLensSpanExporter(SpanExporter):
    def __init__(
        self,
        inner: SpanExporter,
        config: AgentLensConfig,
        redactor: Optional[Redactor] = None,
        store: Optional[PayloadStore] = None,
    ):
        self._inner = inner
        self._config = config
        self._redactor = redactor if (redactor or config.redact_pii) else None
        if config.redact_pii and self._redactor is None:
            self._redactor = Redactor()
        if store is not None:
            self._store = store
        elif config.payload_mode == "reference":
            self._store = LocalFilePayloadStore()
        else:
            self._store = NoopPayloadStore()

    def _transform(self, span: ReadableSpan) -> ReadableSpan:
        attrs = dict(span.attributes or {})

        # Capa adaptadora de convenciones: homogeneiza el esquema venga de donde
        # venga el span (helpers propios o auto-instrumentación de terceros)
        # antes de redactar/externalizar, que ya trabajan contra claves canónicas.
        attrs = normalize_attributes(attrs)

        if self._redactor is not None:
            attrs = self._redactor.redact_attributes(attrs)

        if self._config.payload_mode == "reference":
            attrs = externalize_attributes(
                attrs, self._store, self._config.content_attrs,
                self._config.payload_threshold_bytes,
            )
        elif self._config.payload_mode == "none":
            attrs = discard_content(attrs, self._config.content_attrs)

        return ReadableSpan(
            name=span.name,
            context=span.context,
            parent=span.parent,
            resource=span.resource,
            attributes=attrs,
            events=span.events,
            links=span.links,
            kind=span.kind,
            instrumentation_scope=span.instrumentation_scope,
            status=span.status,
            start_time=span.start_time,
            end_time=span.end_time,
        )

    def export(self, spans: Sequence[ReadableSpan]) -> SpanExportResult:
        transformed = [self._transform(s) for s in spans]
        return self._inner.export(transformed)

    def shutdown(self) -> None:
        self._inner.shutdown()

    def force_flush(self, timeout_millis: int = 30000) -> bool:
        return self._inner.force_flush(timeout_millis)
