"""Tests de la capa adaptadora de convenciones (E1-T09).

Criterio de aceptación: un cambio de versión/variante de semconv no rompe el
contrato del SDK. Lo verificamos comprobando que distintos esquemas de entrada
(legacy "llm.*", "ai.*", versiones previas de gen_ai.*) se normalizan al mismo
esquema canónico, tanto a nivel de función pura como end-to-end por el exporter.
"""
import agentlens
from agentlens.conventions import (
    CONVENTIONS_VERSION,
    GenAI,
    canonical_key,
    normalize_attributes,
)
from opentelemetry.sdk.trace.export.in_memory_span_exporter import InMemorySpanExporter


def test_canonical_key_resolves_known_aliases():
    assert canonical_key("gen_ai.usage.prompt_tokens") == GenAI.USAGE_INPUT_TOKENS
    assert canonical_key("llm.token_count.completion") == GenAI.USAGE_OUTPUT_TOKENS
    assert canonical_key("ai.model.id") == GenAI.REQUEST_MODEL
    # Sin alias -> se devuelve tal cual.
    assert canonical_key("custom.attr") == "custom.attr"


def test_normalize_renames_legacy_token_keys():
    attrs = {
        "gen_ai.usage.prompt_tokens": 120,
        "gen_ai.usage.completion_tokens": 40,
        "llm.request.model": "gpt-4o",
        "unrelated": "x",
    }
    out = normalize_attributes(attrs)
    assert out[GenAI.USAGE_INPUT_TOKENS] == 120
    assert out[GenAI.USAGE_OUTPUT_TOKENS] == 40
    assert out[GenAI.REQUEST_MODEL] == "gpt-4o"
    assert out["unrelated"] == "x"
    # Las claves legacy ya no están.
    assert "gen_ai.usage.prompt_tokens" not in out
    assert "llm.request.model" not in out


def test_normalize_does_not_overwrite_existing_canonical():
    # Si el span trae a la vez la canónica y el alias, gana la canónica.
    attrs = {
        GenAI.USAGE_INPUT_TOKENS: 100,
        "gen_ai.usage.prompt_tokens": 999,
    }
    out = normalize_attributes(attrs)
    assert out[GenAI.USAGE_INPUT_TOKENS] == 100
    assert "gen_ai.usage.prompt_tokens" not in out


def test_different_input_schemas_converge_to_same_contract():
    legacy_llm = normalize_attributes(
        {"llm.token_count.prompt": 10, "llm.token_count.completion": 5}
    )
    vercel_ai = normalize_attributes(
        {"ai.usage.promptTokens": 10, "ai.usage.completionTokens": 5}
    )
    assert legacy_llm == vercel_ai == {
        GenAI.USAGE_INPUT_TOKENS: 10,
        GenAI.USAGE_OUTPUT_TOKENS: 5,
    }


def test_end_to_end_normalization_through_exporter():
    """Un span emitido con claves legacy sale del SDK ya normalizado."""
    mem = InMemorySpanExporter()
    provider = agentlens.instrument(
        tenant_id="acme",
        redact_pii=False,
        payload_mode="inline",
        exporter=mem,
        auto_instrument=False,
    )
    tracer = agentlens.get_tracer()
    with tracer.start_as_current_span("chat gpt-4o") as span:
        # Simula un instrumentador de terceros con esquema legacy.
        span.set_attribute("gen_ai.usage.prompt_tokens", 200)
        span.set_attribute("llm.response.model", "gpt-4o")

    provider.force_flush()
    out = mem.get_finished_spans()[0]
    assert out.attributes[GenAI.USAGE_INPUT_TOKENS] == 200
    assert out.attributes[GenAI.RESPONSE_MODEL] == "gpt-4o"
    assert "gen_ai.usage.prompt_tokens" not in out.attributes
    agentlens.shutdown()


def test_resource_publishes_conventions_version():
    mem = InMemorySpanExporter()
    provider = agentlens.instrument(
        tenant_id="acme", exporter=mem, auto_instrument=False
    )
    with agentlens.tool("x"):
        pass
    provider.force_flush()
    span = mem.get_finished_spans()[0]
    assert (
        span.resource.attributes["agentlens.conventions.version"]
        == CONVENTIONS_VERSION
    )
    agentlens.shutdown()
