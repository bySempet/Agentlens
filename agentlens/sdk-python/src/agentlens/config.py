"""Configuración del SDK de AgentLens.

Toda la configuración tiene defaults seguros y puede sobreescribirse por
variables de entorno (prefijo AGENTLENS_) o por argumentos a ``instrument()``.
"""
from __future__ import annotations

import os
from dataclasses import dataclass, field
from typing import Optional

# Atributos que típicamente contienen el contenido grande (prompts/outputs/
# argumentos de herramientas). Son los candidatos a externalización y los que
# con mayor probabilidad contienen PII.
DEFAULT_CONTENT_ATTRS = (
    "gen_ai.input.messages",
    "gen_ai.output.messages",
    "gen_ai.system_instructions",
    "gen_ai.tool.call.arguments",
    "gen_ai.tool.call.result",
    "gen_ai.prompt",
    "gen_ai.completion",
)


def _env_bool(name: str, default: bool) -> bool:
    val = os.getenv(name)
    if val is None:
        return default
    return val.strip().lower() in ("1", "true", "yes", "on")


@dataclass
class AgentLensConfig:
    """Configuración inmutable del SDK resuelta en el arranque."""

    # Identidad / routing
    api_key: Optional[str] = None
    endpoint: str = "http://localhost:4317"  # OTel Collector OTLP/gRPC
    tenant_id: Optional[str] = None
    agent_id: str = "default"
    service_name: str = "agentlens-agent"
    environment: str = "development"

    # Privacidad
    redact_pii: bool = True

    # Externalización de payloads: "reference" | "inline" | "none"
    #   reference -> payloads grandes salen del span y dejan una referencia
    #   inline    -> se mantienen en el span (no recomendado en producción)
    #   none      -> se descartan (solo metadatos)
    payload_mode: str = "reference"
    payload_threshold_bytes: int = 4096
    content_attrs: tuple = DEFAULT_CONTENT_ATTRS

    # Camino asíncrono (no negociable: el agente nunca espera al backend)
    flush_interval_ms: int = 100
    max_queue_size: int = 2048
    max_export_batch_size: int = 512

    # Dev
    console: bool = False  # exporta a consola en lugar de OTLP (útil sin Collector)

    @classmethod
    def from_env(cls, **overrides) -> "AgentLensConfig":
        """Construye la config combinando defaults < entorno < argumentos."""
        base = dict(
            api_key=os.getenv("AGENTLENS_API_KEY"),
            endpoint=os.getenv("AGENTLENS_ENDPOINT", cls.endpoint),
            tenant_id=os.getenv("AGENTLENS_TENANT_ID"),
            agent_id=os.getenv("AGENTLENS_AGENT_ID", cls.agent_id),
            service_name=os.getenv("AGENTLENS_SERVICE_NAME", cls.service_name),
            environment=os.getenv("AGENTLENS_ENV", cls.environment),
            redact_pii=_env_bool("AGENTLENS_REDACT_PII", cls.redact_pii),
            payload_mode=os.getenv("AGENTLENS_PAYLOAD_MODE", cls.payload_mode),
            console=_env_bool("AGENTLENS_CONSOLE", cls.console),
        )
        # Los overrides explícitos (no-None) ganan sobre el entorno.
        for k, v in overrides.items():
            if v is not None:
                base[k] = v
        return cls(**base)

    def validate(self) -> None:
        if self.payload_mode not in ("reference", "inline", "none"):
            raise ValueError(f"payload_mode inválido: {self.payload_mode!r}")
        if self.flush_interval_ms <= 0:
            raise ValueError("flush_interval_ms debe ser > 0")
