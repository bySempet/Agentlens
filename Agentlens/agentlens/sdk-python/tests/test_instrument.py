"""Test de integración end-to-end del SDK.

Usa un InMemorySpanExporter inyectado para verificar que, a través de
``instrument()``, los spans salen con:
  - enriquecimiento de recurso (tenant/agente)
  - redacción de PII
  - externalización de payloads grandes
sin necesidad de un Collector ni red.
"""
import agentlens
from opentelemetry.sdk.trace.export.in_memory_span_exporter import InMemorySpanExporter


def test_end_to_end_redaction_enrichment_and_payload():
    mem = InMemorySpanExporter()
    provider = agentlens.instrument(
        tenant_id="acme",
        agent_id="support-bot",
        service_name="test-svc",
        redact_pii=True,
        payload_mode="reference",
        exporter=mem,
        auto_instrument=False,
    )

    with agentlens.agent("support-bot"):
        with agentlens.tool("lookup_customer") as t:
            t.set_attribute("user.email", "alice@example.com")
            t.set_attribute("gen_ai.output.messages", "respuesta " + "y" * 5000)

    provider.force_flush()
    spans = mem.get_finished_spans()
    assert len(spans) == 2  # invoke_agent + execute_tool

    tool_span = next(s for s in spans if s.name.startswith("execute_tool"))

    # Enriquecimiento de recurso
    assert tool_span.resource.attributes["agentlens.tenant.id"] == "acme"
    assert tool_span.resource.attributes["agentlens.agent.id"] == "support-bot"

    # Redacción de PII
    assert tool_span.attributes["user.email"] == "[REDACTED_EMAIL]"

    # Externalización de payload grande
    assert tool_span.attributes["gen_ai.output.messages"].startswith(
        "agentlens://payload/"
    )
    assert "agentlens.payload.externalized" in tool_span.attributes

    # Convención OTel GenAI en los spans manuales
    assert tool_span.attributes["gen_ai.operation.name"] == "execute_tool"
    assert tool_span.attributes["gen_ai.tool.name"] == "lookup_customer"

    agentlens.shutdown()


def test_payload_mode_none_discards_content():
    mem = InMemorySpanExporter()
    provider = agentlens.instrument(
        tenant_id="acme",
        payload_mode="none",
        exporter=mem,
        auto_instrument=False,
    )
    with agentlens.tool("x") as t:
        t.set_attribute("gen_ai.output.messages", "contenido sensible")
        t.set_attribute("gen_ai.tool.name", "x")
    provider.force_flush()
    span = mem.get_finished_spans()[0]
    assert "gen_ai.output.messages" not in span.attributes
    assert span.attributes["gen_ai.tool.name"] == "x"
    agentlens.shutdown()
