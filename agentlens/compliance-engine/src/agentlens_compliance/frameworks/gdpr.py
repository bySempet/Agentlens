"""Mapeo GDPR (E5-T03): traza -> evidencia de cumplimiento.

Cubre los principios y obligaciones del RGPD verificables desde las trazas: base
legal, minimización de datos, limitación de la finalidad y del plazo, registro de
accesos, derechos del interesado y trazabilidad.
"""
from __future__ import annotations

from typing import Callable, List, Tuple, Union

from ..evidence import Facts
from ..model import MappedRequirement, Mapping, Requirement, Status

FRAMEWORK = "GDPR"
VERSION = "2016-679"  # Reglamento (UE) 2016/679

FactRef = Union[str, Callable[[Facts], bool]]


def _point(rid: str, article: str, title: str, fact: FactRef, severity: str = "must") -> MappedRequirement:
    req = Requirement(id=rid, framework=FRAMEWORK, article=article, title=title,
                      description=title, severity=severity)

    def check(facts: Facts) -> Tuple[Status, List[str]]:
        ok = fact(facts) if callable(fact) else bool(getattr(facts, fact))
        return (Status.COVERED, [f"OK: {title}"]) if ok else (Status.MISSING, [f"Falta evidencia: {title}"])

    return MappedRequirement(req, check)


_POINTS = [
    # Art. 5 — Principios del tratamiento
    _point("GDPR-5-01", "Art. 5(1)(a)", "Base legal del tratamiento registrada", "lawful_basis_recorded"),
    _point("GDPR-5-02", "Art. 5(1)(b)", "Finalidad del tratamiento registrada", "purpose_recorded"),
    _point("GDPR-5-03", "Art. 5(1)(c)", "Minimización de datos (PII redactada)", "pii_minimized"),
    _point("GDPR-5-04", "Art. 5(1)(e)", "Limitación del plazo de conservación", "retention_configured"),
    _point("GDPR-5-05", "Art. 5(2)", "Responsabilidad proactiva: pista de auditoría", "has_records"),
    _point("GDPR-5-06", "Art. 5(1)(f)", "Integridad: esquema de evidencias estable", "conventions_versioned"),
    # Art. 30 — Registro de actividades de tratamiento
    _point("GDPR-30-01", "Art. 30", "Registro de las actividades de tratamiento", "has_records"),
    _point("GDPR-30-02", "Art. 30", "Identificación del responsable (agente)", "actor_identified"),
    _point("GDPR-30-03", "Art. 30", "Atribución por organización (tenant)", "tenant_isolated"),
    _point("GDPR-30-04", "Art. 30", "Marca temporal de cada tratamiento", "all_timestamped"),
    # Art. 15 — Derecho de acceso / registro de accesos
    _point("GDPR-15-01", "Art. 15", "Registro de accesos a datos", "access_logged"),
    _point("GDPR-15-02", "Art. 15", "Datos de entrada del interesado registrados", "input_recorded"),
    _point("GDPR-15-03", "Art. 15", "Resultados del tratamiento registrados", "output_recorded"),
    # Art. 17 — Derecho de supresión
    _point("GDPR-17-01", "Art. 17", "Soporte de borrado (payloads externalizados)", "erasure_supported"),
    _point("GDPR-17-02", "Art. 17", "Vinculación con el interesado (pseudonimizada)", "data_subject_identified", "should"),
    # Art. 33 — Notificación de brechas
    _point("GDPR-33-01", "Art. 33", "Detección de incidencias para notificación", "errors_recorded", "should"),
    _point("GDPR-33-02", "Art. 33", "Trazabilidad para evaluar el alcance", "has_records"),
]


def mapping() -> Mapping:
    """Devuelve el mapping GDPR versionado."""
    return Mapping(framework=FRAMEWORK, version=VERSION, requirements=list(_POINTS))
