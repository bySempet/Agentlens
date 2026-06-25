"""Matriz de tests de la auto-instrumentación (E1-T10).

No requiere tener instalado ningún framework: se inyecta un ``importer`` falso
que simula la presencia/ausencia/fallo de cada instrumentador. Se verifica:
  - activación de los presentes
  - omisión limpia de los no instalados (ImportError)
  - aislamiento de fallos (un instrumentador roto no tumba a los demás)
  - que los spans de cualquier instrumentador pasan por el pipeline de AgentLens
    (enriquecimiento de recurso + normalización de convenciones)
"""
import agentlens
from agentlens.instrumentation import (
    DEFAULT_INSTRUMENTORS,
    Instrumentor,
    activate,
)
from opentelemetry.sdk.trace.export.in_memory_span_exporter import InMemorySpanExporter


def make_importer(modules: dict):
    """Construye un importer falso a partir de {module: {cls: obj|Exception}}.

    Si el módulo no está en el dict -> ImportError (no instalado). Si el valor es
    una excepción, importar el módulo la lanza.
    """
    def _importer(module: str, fromlist):
        if module not in modules:
            raise ImportError(f"no module {module}")
        attrs = modules[module]
        if isinstance(attrs, Exception):
            raise attrs
        ns = type("FakeModule", (), {})()
        for cls_name, obj in attrs.items():
            setattr(ns, cls_name, obj)
        return ns
    return _importer


def fake_instrumentor(record, name, raise_on_instrument=False):
    """Devuelve una clase Instrumentor falsa que registra su activación."""
    class _Fake:
        def instrument(self):
            if raise_on_instrument:
                raise RuntimeError("boom")
            record.append(name)
    return _Fake


def test_all_present_are_activated():
    record = []
    specs = [
        Instrumentor("A", "mod.a", "AInstr"),
        Instrumentor("B", "mod.b", "BInstr"),
    ]
    importer = make_importer({
        "mod.a": {"AInstr": fake_instrumentor(record, "A")},
        "mod.b": {"BInstr": fake_instrumentor(record, "B")},
    })
    result = activate(specs, importer=importer)
    assert result.activated == ["A", "B"]
    assert result.skipped == [] and result.failed == []
    assert record == ["A", "B"]  # ambos .instrument() ejecutados


def test_missing_framework_is_skipped():
    specs = [
        Instrumentor("A", "mod.a", "AInstr"),
        Instrumentor("Missing", "mod.missing", "X"),
    ]
    importer = make_importer({"mod.a": {"AInstr": fake_instrumentor([], "A")}})
    result = activate(specs, importer=importer)
    assert result.activated == ["A"]
    assert result.skipped == ["Missing"]
    assert result.failed == []


def test_failing_instrumentor_is_isolated():
    record = []
    specs = [
        Instrumentor("Broken", "mod.broken", "BrokenInstr"),
        Instrumentor("Good", "mod.good", "GoodInstr"),
    ]
    importer = make_importer({
        "mod.broken": {"BrokenInstr": fake_instrumentor(record, "Broken", raise_on_instrument=True)},
        "mod.good": {"GoodInstr": fake_instrumentor(record, "Good")},
    })
    result = activate(specs, importer=importer)
    assert result.activated == ["Good"]
    assert [n for n, _ in result.failed] == ["Broken"]
    assert record == ["Good"]  # el bueno se activó pese al fallo del otro


def test_default_registry_covers_p0_frameworks():
    names = {i.name for i in DEFAULT_INSTRUMENTORS}
    for expected in {"OpenAI", "LangChain", "CrewAI", "AutoGen", "PydanticAI", "Bedrock"}:
        assert expected in names, f"falta {expected} en la matriz por defecto"


def test_activate_with_real_defaults_never_raises():
    # En el entorno de test no hay frameworks instalados: todo debe omitirse sin
    # lanzar excepción.
    result = activate()
    assert result.failed == []
    assert len(result.skipped) == len(DEFAULT_INSTRUMENTORS)


def test_instrumentor_spans_flow_through_agentlens_pipeline():
    """Un span emitido por cualquier instrumentador hereda recurso AgentLens y
    se normaliza (alias legacy -> canónico)."""
    mem = InMemorySpanExporter()
    provider = agentlens.instrument(
        tenant_id="acme",
        redact_pii=False,
        payload_mode="inline",
        exporter=mem,
        auto_instrument=False,  # activamos manualmente nuestro instrumentador falso
    )

    class Emitter:
        def instrument(self):
            from opentelemetry import trace
            tracer = trace.get_tracer("fake-framework")
            with tracer.start_as_current_span("chat gpt-4o") as s:
                # clave legacy que la capa de convenciones debe normalizar
                s.set_attribute("gen_ai.usage.prompt_tokens", 11)

    importer = make_importer({"mod.e": {"Emitter": Emitter}})
    result = activate([Instrumentor("Emitter", "mod.e", "Emitter")], importer=importer)
    assert result.activated == ["Emitter"]

    provider.force_flush()
    span = mem.get_finished_spans()[0]
    # Enriquecimiento de recurso
    assert span.resource.attributes["agentlens.tenant.id"] == "acme"
    # Normalización de convenciones
    assert span.attributes[agentlens.GenAI.USAGE_INPUT_TOKENS] == 11
    assert "gen_ai.usage.prompt_tokens" not in span.attributes
    agentlens.shutdown()
