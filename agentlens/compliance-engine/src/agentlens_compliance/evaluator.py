"""Evaluación de cumplimiento: trazas + mapping -> informe (E5).

Calcula, para cada requisito del mapping, su estado (covered/partial/missing) y
las evidencias, además de una puntuación de cobertura agregada.
"""
from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Dict, List

from .evidence import Facts, Span
from .model import Mapping, Requirement, Status


@dataclass
class RequirementResult:
    requirement: Requirement
    status: Status
    evidence: List[str] = field(default_factory=list)


@dataclass
class Report:
    framework: str
    version: str
    results: List[RequirementResult]

    @property
    def total(self) -> int:
        return len(self.results)

    @property
    def covered(self) -> int:
        return sum(1 for r in self.results if r.status == Status.COVERED)

    @property
    def partial(self) -> int:
        return sum(1 for r in self.results if r.status == Status.PARTIAL)

    @property
    def missing(self) -> int:
        return sum(1 for r in self.results if r.status == Status.MISSING)

    def coverage(self) -> float:
        """Puntuación 0..1: covered cuenta 1, partial 0.5."""
        if not self.results:
            return 0.0
        score = self.covered + 0.5 * self.partial
        return round(score / self.total, 4)

    def gaps(self) -> List[RequirementResult]:
        return [r for r in self.results if r.status != Status.COVERED]

    def to_dict(self) -> Dict[str, Any]:
        return {
            "framework": self.framework,
            "version": self.version,
            "coverage": self.coverage(),
            "totals": {"total": self.total, "covered": self.covered,
                       "partial": self.partial, "missing": self.missing},
            "requirements": [
                {
                    "id": r.requirement.id,
                    "article": r.requirement.article,
                    "title": r.requirement.title,
                    "severity": r.requirement.severity,
                    "status": r.status.value,
                    "evidence": r.evidence,
                }
                for r in self.results
            ],
        }


def evaluate(spans, mapping: Mapping) -> Report:
    """Evalúa un mapping contra una lista de spans (dicts o ``Span``)."""
    if spans and isinstance(spans[0], Span):
        facts = Facts(list(spans))
    else:
        facts = Facts.from_dicts(list(spans))

    results: List[RequirementResult] = []
    for mr in mapping.requirements:
        status, evidence = mr.check(facts)
        results.append(RequirementResult(mr.requirement, status, evidence))
    return Report(mapping.framework, mapping.version, results)
