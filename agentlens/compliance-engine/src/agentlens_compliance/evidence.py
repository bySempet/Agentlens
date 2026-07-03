"""Hechos derivados de las trazas para evaluar el cumplimiento.

``Facts`` resume un conjunto de spans (con la forma OTel GenAI que emite el SDK
de AgentLens) en propiedades booleanas/contadores que los requisitos consultan.
Mantener esta capa separada hace que los mappings sean declarativos y testeables.
"""
from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Dict, List


@dataclass
class Span:
    """Span mínimo para evaluación de compliance (subconjunto de OTel)."""

    name: str = ""
    attributes: Dict[str, Any] = field(default_factory=dict)
    resource: Dict[str, Any] = field(default_factory=dict)
    start_time: Any = None
    status_code: str = ""


@dataclass
class Facts:
    spans: List[Span]

    # --- helpers de bajo nivel ---
    def _any_attr(self, key: str) -> bool:
        return any(key in s.attributes for s in self.spans)

    def _any_attr_truthy(self, key: str) -> bool:
        return any(str(s.attributes.get(key, "")).lower() in ("true", "1", "yes")
                   for s in self.spans)

    def _any_resource(self, key: str) -> bool:
        return any(key in s.resource for s in self.spans)

    def _any_attr_contains(self, needle: str) -> bool:
        for s in self.spans:
            for v in s.attributes.values():
                if isinstance(v, str) and needle in v:
                    return True
        return False

    # --- hechos de alto nivel (consumidos por los mappings) ---
    @property
    def has_records(self) -> bool:
        return len(self.spans) > 0

    @property
    def all_timestamped(self) -> bool:
        return self.has_records and all(s.start_time is not None for s in self.spans)

    @property
    def actor_identified(self) -> bool:
        return self._any_resource("agentlens.agent.id")

    @property
    def tenant_isolated(self) -> bool:
        return self._any_resource("agentlens.tenant.id")

    @property
    def model_recorded(self) -> bool:
        return self._any_attr("gen_ai.request.model") or self._any_attr("gen_ai.response.model")

    @property
    def provider_disclosed(self) -> bool:
        return self._any_attr("gen_ai.system")

    @property
    def token_usage_recorded(self) -> bool:
        return self._any_attr("gen_ai.usage.input_tokens") or self._any_attr("gen_ai.usage.output_tokens")

    @property
    def input_recorded(self) -> bool:
        return self._any_attr("gen_ai.input.messages") or self._any_attr("gen_ai.system_instructions")

    @property
    def output_recorded(self) -> bool:
        return self._any_attr("gen_ai.output.messages")

    @property
    def tool_calls_recorded(self) -> bool:
        return any(s.name.startswith("execute_tool") for s in self.spans) or self._any_attr("gen_ai.tool.name")

    @property
    def errors_recorded(self) -> bool:
        return any(s.status_code in ("STATUS_CODE_ERROR", "ERROR") for s in self.spans)

    @property
    def pii_minimized(self) -> bool:
        # Evidencia de redacción en cliente o externalización de payloads.
        return self._any_attr_contains("[REDACTED") or self._any_attr("agentlens.payload.externalized")

    @property
    def conventions_versioned(self) -> bool:
        return self._any_resource("agentlens.conventions.version")

    @property
    def human_oversight(self) -> bool:
        return self._any_attr_truthy("agentlens.human_oversight") or self._any_attr("agentlens.review.by")

    @property
    def escalation_recorded(self) -> bool:
        return self._any_attr_truthy("agentlens.escalated")

    @property
    def ai_interaction_marked(self) -> bool:
        # Art. 50: queda registro de que es una interacción con IA.
        return self._any_attr("gen_ai.operation.name") or self.provider_disclosed

    @property
    def decision_recorded(self) -> bool:
        # Hay entrada y salida -> la decisión del agente es reconstruible.
        return self.input_recorded and self.output_recorded

    @property
    def retention_configured(self) -> bool:
        return self._any_resource("agentlens.retention.days")

    @classmethod
    def from_dicts(cls, spans: List[Dict[str, Any]]) -> "Facts":
        out = []
        for s in spans:
            out.append(Span(
                name=s.get("name", ""),
                attributes=s.get("attributes", {}) or {},
                resource=s.get("resource", {}) or {},
                start_time=s.get("start_time"),
                status_code=s.get("status_code", ""),
            ))
        return cls(out)
