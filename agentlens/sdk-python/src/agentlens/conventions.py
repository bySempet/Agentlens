"""Capa adaptadora de convenciones (E1-T09).

El esquema OTel GenAI está en estado *Development*: los nombres de atributos
cambian entre versiones de ``opentelemetry-semantic-conventions`` y, peor aún,
los distintos auto-instrumentadores de terceros (OpenAI, LangChain, CrewAI,
Bedrock, librerías legacy "llm.*"/"ai.*") emiten claves diferentes para el mismo
concepto.

Esta capa es el **único** punto del SDK que conoce los nombres concretos de las
convenciones. El resto del código (helpers ``agent``/``tool``, pipeline de
export, externalización, costes) trabaja siempre contra constantes canónicas
definidas aquí. Así, cuando cambie la versión de semconv, sólo se toca este
módulo y el contrato del SDK hacia el cliente no se rompe.

Dos responsabilidades:

1. **Constantes canónicas** (``GenAI``): la versión actual de las claves OTel
   GenAI que emite AgentLens. Un único sitio que actualizar.
2. **Normalización de entrada** (``normalize_attributes``): mapea las variantes
   conocidas (alias legacy o de otras versiones/instrumentadores) a la clave
   canónica, para que toda traza —venga de donde venga— llegue al backend con un
   esquema homogéneo.
"""
from __future__ import annotations

from typing import Dict, Mapping

# Versión del contrato de convenciones que emite este SDK. Se publica como
# atributo de recurso para que el backend sepa con qué esquema interpretar.
CONVENTIONS_VERSION = "genai-1.0"


class GenAI:
    """Claves canónicas OTel GenAI usadas por AgentLens (esquema estable)."""

    # Operación y actores
    OPERATION_NAME = "gen_ai.operation.name"
    SYSTEM = "gen_ai.system"
    AGENT_NAME = "gen_ai.agent.name"
    AGENT_ID = "gen_ai.agent.id"
    TOOL_NAME = "gen_ai.tool.name"
    TOOL_CALL_ID = "gen_ai.tool.call.id"

    # Modelo
    REQUEST_MODEL = "gen_ai.request.model"
    RESPONSE_MODEL = "gen_ai.response.model"

    # Uso de tokens (clave para el dashboard de coste, E3-T06)
    USAGE_INPUT_TOKENS = "gen_ai.usage.input_tokens"
    USAGE_OUTPUT_TOKENS = "gen_ai.usage.output_tokens"

    # Contenido (candidato a redacción + externalización)
    INPUT_MESSAGES = "gen_ai.input.messages"
    OUTPUT_MESSAGES = "gen_ai.output.messages"
    SYSTEM_INSTRUCTIONS = "gen_ai.system_instructions"
    TOOL_CALL_ARGUMENTS = "gen_ai.tool.call.arguments"
    TOOL_CALL_RESULT = "gen_ai.tool.call.result"

    # Operaciones estándar (valores de OPERATION_NAME)
    OP_INVOKE_AGENT = "invoke_agent"
    OP_EXECUTE_TOOL = "execute_tool"
    OP_CHAT = "chat"


# Atributos canónicos que contienen el "contenido grande" (prompts/outputs/
# argumentos). Fuente única para redacción/externalización en lugar de listas
# sueltas repartidas por el SDK.
CONTENT_ATTRS = (
    GenAI.INPUT_MESSAGES,
    GenAI.OUTPUT_MESSAGES,
    GenAI.SYSTEM_INSTRUCTIONS,
    GenAI.TOOL_CALL_ARGUMENTS,
    GenAI.TOOL_CALL_RESULT,
    # Alias legacy frecuentes; se mantienen para externalizar contenido que no
    # haya sido normalizado todavía (defensa en profundidad).
    "gen_ai.prompt",
    "gen_ai.completion",
)


# Mapa de alias -> clave canónica. Cubre:
#   - versiones previas de semconv (prompt/completion_tokens)
#   - instrumentadores legacy estilo OpenLLMetry/OpenInference ("llm.*")
#   - el namespace "ai.*" usado por algunas librerías de Vercel/JS
# Mantener este mapa es mucho más barato que perseguir cambios por todo el SDK.
_ALIAS_MAP: Dict[str, str] = {
    # tokens
    "gen_ai.usage.prompt_tokens": GenAI.USAGE_INPUT_TOKENS,
    "gen_ai.usage.completion_tokens": GenAI.USAGE_OUTPUT_TOKENS,
    "llm.usage.prompt_tokens": GenAI.USAGE_INPUT_TOKENS,
    "llm.usage.completion_tokens": GenAI.USAGE_OUTPUT_TOKENS,
    "llm.token_count.prompt": GenAI.USAGE_INPUT_TOKENS,
    "llm.token_count.completion": GenAI.USAGE_OUTPUT_TOKENS,
    "ai.usage.promptTokens": GenAI.USAGE_INPUT_TOKENS,
    "ai.usage.completionTokens": GenAI.USAGE_OUTPUT_TOKENS,
    # modelo
    "llm.request.model": GenAI.REQUEST_MODEL,
    "llm.response.model": GenAI.RESPONSE_MODEL,
    "ai.model.id": GenAI.REQUEST_MODEL,
    # sistema/proveedor
    "llm.system": GenAI.SYSTEM,
    "llm.vendor": GenAI.SYSTEM,
    # contenido
    "llm.prompts": GenAI.INPUT_MESSAGES,
    "llm.completions": GenAI.OUTPUT_MESSAGES,
    "gen_ai.prompt": GenAI.INPUT_MESSAGES,
    "gen_ai.completion": GenAI.OUTPUT_MESSAGES,
    "ai.prompt": GenAI.INPUT_MESSAGES,
    "ai.response.text": GenAI.OUTPUT_MESSAGES,
    # herramientas
    "llm.tool.name": GenAI.TOOL_NAME,
    "tool.name": GenAI.TOOL_NAME,
}


def canonical_key(key: str) -> str:
    """Devuelve la clave canónica para un atributo, o la propia si no hay alias."""
    return _ALIAS_MAP.get(key, key)


def normalize_attributes(attributes: Mapping) -> dict:
    """Reescribe atributos de un span al esquema canónico de AgentLens.

    - Renombra alias conocidos a su clave canónica.
    - No pisa una clave canónica ya presente: si el instrumentador emitió tanto
      ``gen_ai.usage.input_tokens`` como el alias legacy, gana la canónica y el
      alias se descarta para no duplicar.
    - Atributos sin alias pasan tal cual.
    """
    out: dict = {}
    for key, value in attributes.items():
        target = _ALIAS_MAP.get(key, key)
        if target != key and target in attributes:
            # La canónica ya viene en el span; ignoramos el alias redundante.
            continue
        out[target] = value
    return out
