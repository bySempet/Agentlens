"""Madurez de compliance multi-framework (E5-T06).

Evalúa las trazas contra varios frameworks y produce una puntuación de madurez
por framework (cobertura global + desglose por artículo), lista para el dashboard.
"""
from __future__ import annotations

from typing import Any, Dict, List

from .evaluator import evaluate
from .frameworks import eu_ai_act, gdpr

# Frameworks incluidos en el dashboard de madurez.
_MAPPINGS = [eu_ai_act.mapping, gdpr.mapping]


def _band(coverage: float) -> str:
    """Nivel de madurez cualitativo a partir de la cobertura."""
    if coverage >= 0.9:
        return "avanzado"
    if coverage >= 0.7:
        return "gestionado"
    if coverage >= 0.4:
        return "inicial"
    return "insuficiente"


def summarize(spans) -> Dict[str, Any]:
    """Devuelve la madurez por framework para un conjunto de trazas."""
    frameworks: List[Dict[str, Any]] = []
    for make in _MAPPINGS:
        report = evaluate(spans, make())
        frameworks.append({
            "framework": report.framework,
            "version": report.version,
            "coverage": report.coverage(),
            "band": _band(report.coverage()),
            "totals": {"total": report.total, "covered": report.covered,
                       "partial": report.partial, "missing": report.missing},
            "by_article": report.by_article(),
        })
    overall = round(sum(f["coverage"] for f in frameworks) / len(frameworks), 4) if frameworks else 0.0
    return {"overall_coverage": overall, "frameworks": frameworks}
