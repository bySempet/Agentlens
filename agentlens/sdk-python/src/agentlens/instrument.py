"""API pública del SDK de AgentLens.

Uso mínimo (las "3 líneas" del brief):

    import agentlens
    agentlens.instrument(api_key="...", tenant_id="acme", agent_id="support-bot")
    # ... el resto del código del agente se instrumenta automáticamente

También expone helpers de instrumentación manual (``agent`` y ``tool``) que
siguen las convenciones OTel GenAI (spans ``invoke_agent`` y ``execute_tool``),
útiles cuando no se usa un framework con auto-instrumentación.
"""
from __future__ import annotations

import logging
from contextlib import contextmanager
from typing import Optional

from opentelemetry import trace
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import (
    BatchSpanProcessor,
    ConsoleSpanExporter,
    SpanExporter,
)

from .config import AgentLensConfig
from .conventions import CONVENTIONS_VERSION, GenAI
from .instrumentation import activate as activate_instrumentation
from .processors import AgentLensSpanExporter
from .redaction import Redactor

logger = logging.getLogger("agentlens")

_PROVIDER: Optional[TracerProvider] = None


def _build_resource(cfg: AgentLensConfig) -> Resource:
    """Enriquecimiento: todos los spans heredan tenant/agente/servicio."""
    return Resource.create(
        {
            "service.name": cfg.service_name,
            "deployment.environment": cfg.environment,
            "agentlens.tenant.id": cfg.tenant_id or "unknown",
            "agentlens.agent.id": cfg.agent_id,
            "agentlens.sdk.language": "python",
            # Versión del contrato de convenciones con que se emiten los spans.
            # El backend la usa para interpretar el esquema de forma estable.
            "agentlens.conventions.version": CONVENTIONS_VERSION,
        }
    )


def _build_inner_exporter(cfg: AgentLensConfig) -> SpanExporter:
    if cfg.console:
        return ConsoleSpanExporter()
    # Import perezoso para no exigir gRPC en escenarios de solo-consola/tests.
    from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter

    headers = (("x-agentlens-key", cfg.api_key),) if cfg.api_key else None
    return OTLPSpanExporter(endpoint=cfg.endpoint, headers=headers, insecure=True)




def instrument(
    api_key: Optional[str] = None,
    *,
    endpoint: Optional[str] = None,
    tenant_id: Optional[str] = None,
    agent_id: Optional[str] = None,
    service_name: Optional[str] = None,
    redact_pii: Optional[bool] = None,
    payload_mode: Optional[str] = None,
    console: Optional[bool] = None,
    exporter: Optional[SpanExporter] = None,
    redactor: Optional[Redactor] = None,
    auto_instrument: bool = True,
) -> TracerProvider:
    """Configura AgentLens. Devuelve el ``TracerProvider`` (útil en tests).

    ``exporter`` permite inyectar un exporter propio (p.ej. InMemory en tests),
    en cuyo caso se ignora el endpoint OTLP.
    """
    global _PROVIDER

    cfg = AgentLensConfig.from_env(
        api_key=api_key,
        endpoint=endpoint,
        tenant_id=tenant_id,
        agent_id=agent_id,
        service_name=service_name,
        redact_pii=redact_pii,
        payload_mode=payload_mode,
        console=console,
    )
    cfg.validate()

    provider = TracerProvider(resource=_build_resource(cfg))

    inner = exporter if exporter is not None else _build_inner_exporter(cfg)
    wrapped = AgentLensSpanExporter(inner, cfg, redactor=redactor)

    # Camino 100% asíncrono: el agente nunca espera al backend. El batch se
    # vacía cada flush_interval_ms en un hilo aparte.
    provider.add_span_processor(
        BatchSpanProcessor(
            wrapped,
            schedule_delay_millis=cfg.flush_interval_ms,
            max_queue_size=cfg.max_queue_size,
            max_export_batch_size=cfg.max_export_batch_size,
        )
    )

    trace.set_tracer_provider(provider)
    _PROVIDER = provider

    if auto_instrument:
        result = activate_instrumentation()
        logger.info("AgentLens auto-instrumentación -> %s", result.summary())

    logger.info(
        "AgentLens activo (tenant=%s, agent=%s, redact_pii=%s, payload_mode=%s)",
        cfg.tenant_id, cfg.agent_id, cfg.redact_pii, cfg.payload_mode,
    )
    return provider


def get_tracer(name: str = "agentlens"):
    return trace.get_tracer(name)


@contextmanager
def agent(name: str, *, agent_id: Optional[str] = None):
    """Span ``invoke_agent`` (convención OTel GenAI) para instrumentación manual.

    Usa las constantes de la capa de convenciones; no hardcodea claves ``gen_ai.*``.
    """
    tracer = get_tracer()
    with tracer.start_as_current_span(f"{GenAI.OP_INVOKE_AGENT} {name}") as span:
        span.set_attribute(GenAI.OPERATION_NAME, GenAI.OP_INVOKE_AGENT)
        span.set_attribute(GenAI.AGENT_NAME, name)
        if agent_id:
            span.set_attribute(GenAI.AGENT_ID, agent_id)
        yield span


@contextmanager
def tool(name: str):
    """Span ``execute_tool`` (convención OTel GenAI) para instrumentación manual."""
    tracer = get_tracer()
    with tracer.start_as_current_span(f"{GenAI.OP_EXECUTE_TOOL} {name}") as span:
        span.set_attribute(GenAI.OPERATION_NAME, GenAI.OP_EXECUTE_TOOL)
        span.set_attribute(GenAI.TOOL_NAME, name)
        yield span


def shutdown() -> None:
    """Vacía y cierra el provider (importante al final de procesos cortos)."""
    if _PROVIDER is not None:
        _PROVIDER.shutdown()
