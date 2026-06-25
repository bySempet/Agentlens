"""Auto-instrumentación de frameworks de agentes (E1-T07/T08/T10).

AgentLens no reimplementa la instrumentación de cada framework: se apoya en los
instrumentadores OpenTelemetry de cada ecosistema y, encima, aplica su capa de
convenciones (E1-T09) y su pipeline de privacidad. Este módulo solo se encarga de
**activar de forma resiliente** los instrumentadores disponibles:

- Si el framework o su instrumentación no están instalados, se omite (no rompe).
- Si un instrumentador falla al activarse, se aísla y los demás siguen.
- El resultado es estructurado (activados / omitidos / fallidos) para poder
  loguearlo y testearlo.

El ``importer`` es inyectable para poder testear la matriz de frameworks sin
tener instalado ninguno (se inyectan instrumentadores falsos).
"""
from __future__ import annotations

from dataclasses import dataclass, field
from typing import Callable, List, Optional


@dataclass(frozen=True)
class Instrumentor:
    """Especificación de un instrumentador: de dónde importarlo."""

    name: str          # nombre legible (p.ej. "OpenAI")
    module: str        # módulo a importar
    cls: str           # clase Instrumentor dentro del módulo


# Frameworks soportados. P0 del backlog: OpenAI, LangChain/LangGraph, CrewAI,
# AutoGen, Pydantic AI y Bedrock Agents. Cada uno se activa best-effort.
DEFAULT_INSTRUMENTORS: List[Instrumentor] = [
    Instrumentor("OpenAI", "opentelemetry.instrumentation.openai_v2", "OpenAIInstrumentor"),
    Instrumentor("LangChain", "opentelemetry.instrumentation.langchain", "LangchainInstrumentor"),
    Instrumentor("CrewAI", "opentelemetry.instrumentation.crewai", "CrewAIInstrumentor"),
    Instrumentor("AutoGen", "opentelemetry.instrumentation.autogen", "AutoGenInstrumentor"),
    Instrumentor("PydanticAI", "opentelemetry.instrumentation.pydantic_ai", "PydanticAIInstrumentor"),
    Instrumentor("Bedrock", "opentelemetry.instrumentation.bedrock", "BedrockInstrumentor"),
]


@dataclass
class ActivationResult:
    """Resultado de activar la matriz de instrumentadores."""

    activated: List[str] = field(default_factory=list)
    skipped: List[str] = field(default_factory=list)        # no instalados
    failed: List[tuple] = field(default_factory=list)        # (name, error)

    def summary(self) -> str:
        parts = []
        if self.activated:
            parts.append("activados: " + ", ".join(self.activated))
        if self.skipped:
            parts.append(f"omitidos: {len(self.skipped)}")
        if self.failed:
            parts.append("fallidos: " + ", ".join(n for n, _ in self.failed))
        return "; ".join(parts) or "ninguno"


# Tipo del importador: (module, fromlist) -> module. Por defecto, el builtin.
Importer = Callable[[str, List[str]], object]


def _default_importer(module: str, fromlist: List[str]):
    return __import__(module, fromlist=fromlist)


def activate(
    instrumentors: Optional[List[Instrumentor]] = None,
    importer: Importer = _default_importer,
) -> ActivationResult:
    """Activa los instrumentadores disponibles y devuelve el resultado.

    - ``ImportError``/``ModuleNotFoundError`` -> se considera "no instalado" (skip).
    - Cualquier otro error al instanciar/activar -> se registra como "fallido"
      pero no detiene al resto.
    """
    specs = instrumentors if instrumentors is not None else DEFAULT_INSTRUMENTORS
    result = ActivationResult()
    for spec in specs:
        try:
            module = importer(spec.module, [spec.cls])
        except ImportError:
            result.skipped.append(spec.name)
            continue
        try:
            instrumentor_cls = getattr(module, spec.cls)
            instrumentor_cls().instrument()
            result.activated.append(spec.name)
        except Exception as exc:  # noqa: BLE001 - aislamos fallos por instrumentador
            result.failed.append((spec.name, str(exc)))
    return result
