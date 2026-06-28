"""Mapeo EU AI Act (E5-T02): traza -> requisito.

Checklist de 40 puntos sobre los artículos clave para sistemas de IA basados en
agentes: Art. 12 (registro/logs), Art. 14 (supervisión humana), Art. 50
(transparencia) y Art. 86 (derecho a explicación), más buenas prácticas
transversales de minimización y trazabilidad.

Cada punto se cubre si las trazas contienen la evidencia correspondiente. El
mapping está versionado: un cambio normativo es una versión nueva.
"""
from __future__ import annotations

from typing import Callable, List, Tuple, Union

from ..evidence import Facts
from ..model import MappedRequirement, Mapping, Requirement, Status

FRAMEWORK = "EU AI Act"
VERSION = "2024-1689"  # Reglamento (UE) 2024/1689

FactRef = Union[str, Callable[[Facts], bool]]


def _point(rid: str, article: str, title: str, fact: FactRef, severity: str = "must") -> MappedRequirement:
    req = Requirement(id=rid, framework=FRAMEWORK, article=article, title=title,
                      description=title, severity=severity)

    def check(facts: Facts) -> Tuple[Status, List[str]]:
        ok = fact(facts) if callable(fact) else bool(getattr(facts, fact))
        if ok:
            return Status.COVERED, [f"OK: {title}"]
        return Status.MISSING, [f"Falta evidencia: {title}"]

    return MappedRequirement(req, check)


_POINTS = [
    # --- Art. 12 — Conservación de registros (logs automáticos) ---
    _point("AIA-12-01", "Art. 12", "Registro automático de eventos del sistema", "has_records"),
    _point("AIA-12-02", "Art. 12", "Marca temporal en cada evento", "all_timestamped"),
    _point("AIA-12-03", "Art. 12", "Identificación del agente responsable", "actor_identified"),
    _point("AIA-12-04", "Art. 12", "Atribución por organización (tenant)", "tenant_isolated"),
    _point("AIA-12-05", "Art. 12", "Registro de las entradas (prompts/contexto)", "input_recorded"),
    _point("AIA-12-06", "Art. 12", "Registro de las salidas del sistema", "output_recorded"),
    _point("AIA-12-07", "Art. 12", "Registro de acciones/herramientas ejecutadas", "tool_calls_recorded"),
    _point("AIA-12-08", "Art. 12", "Registro del modelo/versión utilizado", "model_recorded"),
    _point("AIA-12-09", "Art. 12", "Registro del consumo de recursos (tokens)", "token_usage_recorded"),
    _point("AIA-12-10", "Art. 12", "Registro de errores y anomalías", "errors_recorded", "should"),
    _point("AIA-12-11", "Art. 12", "Esquema de logs versionado y estable", "conventions_versioned"),
    _point("AIA-12-12", "Art. 12", "Periodo de conservación de logs definido", "retention_configured", "should"),

    # --- Art. 14 — Supervisión humana ---
    _point("AIA-14-01", "Art. 14", "Constancia de supervisión humana", "human_oversight"),
    _point("AIA-14-02", "Art. 14", "Mecanismo de escalado a intervención humana", "escalation_recorded"),
    _point("AIA-14-03", "Art. 14", "Decisión reconstruible para revisión", "decision_recorded"),
    _point("AIA-14-04", "Art. 14", "Detección de fallos para intervención", "errors_recorded"),
    _point("AIA-14-05", "Art. 14", "Identificación del agente supervisado", "actor_identified"),
    _point("AIA-14-06", "Art. 14", "Visibilidad de las acciones a supervisar", "tool_calls_recorded"),
    _point("AIA-14-07", "Art. 14", "Contexto disponible para el supervisor", "input_recorded"),
    _point("AIA-14-08", "Art. 14", "Resultado disponible para el supervisor", "output_recorded"),

    # --- Art. 50 — Transparencia ---
    _point("AIA-50-01", "Art. 50", "Se registra que es una interacción con IA", "ai_interaction_marked"),
    _point("AIA-50-02", "Art. 50", "Proveedor/sistema de IA identificado", "provider_disclosed", "should"),
    _point("AIA-50-03", "Art. 50", "Contenido generado registrado", "output_recorded"),
    _point("AIA-50-04", "Art. 50", "Modelo divulgado en el registro", "model_recorded"),
    _point("AIA-50-05", "Art. 50", "Instrucciones del sistema registradas", "input_recorded"),
    _point("AIA-50-06", "Art. 50", "Trazabilidad por organización para informar", "tenant_isolated"),
    _point("AIA-50-07", "Art. 50", "Momento de la interacción registrado", "all_timestamped"),
    _point("AIA-50-08", "Art. 50", "Herramientas/datos accedidos registrados", "tool_calls_recorded", "should"),

    # --- Art. 86 — Derecho a explicación de decisiones individuales ---
    _point("AIA-86-01", "Art. 86", "Decisión explicable (entrada + salida)", "decision_recorded"),
    _point("AIA-86-02", "Art. 86", "Factores de entrada de la decisión", "input_recorded"),
    _point("AIA-86-03", "Art. 86", "Resultado de la decisión", "output_recorded"),
    _point("AIA-86-04", "Art. 86", "Pasos/herramientas hacia la decisión", "tool_calls_recorded"),
    _point("AIA-86-05", "Art. 86", "Modelo responsable de la decisión", "model_recorded"),
    _point("AIA-86-06", "Art. 86", "Agente responsable identificado", "actor_identified"),
    _point("AIA-86-07", "Art. 86", "Secuencia temporal de la decisión", "all_timestamped"),

    # --- Transversal: minimización y trazabilidad (buenas prácticas) ---
    _point("AIA-GEN-01", "Transversal", "Minimización de PII (redacción en cliente)", "pii_minimized"),
    _point("AIA-GEN-02", "Transversal", "Aislamiento multi-tenant de los registros", "tenant_isolated"),
    _point("AIA-GEN-03", "Transversal", "Integridad/estabilidad del esquema de evidencias", "conventions_versioned"),
    _point("AIA-GEN-04", "Transversal", "Disponibilidad de la pista de auditoría", "has_records"),
    _point("AIA-GEN-05", "Transversal", "Monitorización de incidencias", "errors_recorded", "should"),
]


def mapping() -> Mapping:
    """Devuelve el mapping EU AI Act versionado (40 puntos)."""
    return Mapping(framework=FRAMEWORK, version=VERSION, requirements=list(_POINTS))
