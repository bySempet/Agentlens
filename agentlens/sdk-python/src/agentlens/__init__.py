"""AgentLens SDK — Gobernanza y observabilidad para agentes de IA.

Instrumentación basada en OpenTelemetry GenAI, agnóstica al framework y al
proveedor de LLM, con redacción de PII en cliente y externalización de payloads.
"""
from .config import AgentLensConfig
from .conventions import CONVENTIONS_VERSION, GenAI, normalize_attributes
from .instrument import (
    agent,
    check,
    enforce,
    get_tracer,
    instrument,
    shutdown,
    tool,
)
from .instrumentation import ActivationResult, Instrumentor, activate
from .policy import PolicyDecision, PolicyError, PolicyViolation, WasmPolicy
from .redaction import Redactor

__version__ = "0.1.0"

__all__ = [
    "instrument",
    "shutdown",
    "agent",
    "tool",
    "check",
    "enforce",
    "PolicyDecision",
    "PolicyViolation",
    "PolicyError",
    "WasmPolicy",
    "get_tracer",
    "Redactor",
    "AgentLensConfig",
    "GenAI",
    "CONVENTIONS_VERSION",
    "normalize_attributes",
    "activate",
    "Instrumentor",
    "ActivationResult",
    "__version__",
]
