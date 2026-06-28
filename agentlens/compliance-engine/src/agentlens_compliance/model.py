"""Modelo de mapeo regulatorio versionado (E5-T01).

Un *mapping* asocia, para un framework regulatorio y una versión concretos, una
lista de requisitos con la forma de comprobar (a partir de las trazas) si hay
evidencia de cumplimiento. La estructura es editable (datos + funciones puras) y
versionada, de modo que un cambio normativo es una versión nueva del mapping.
"""
from __future__ import annotations

from dataclasses import dataclass, field
from enum import Enum
from typing import Callable, List, Tuple


class Status(str, Enum):
    COVERED = "covered"   # hay evidencia suficiente en las trazas
    PARTIAL = "partial"   # evidencia parcial
    MISSING = "missing"   # sin evidencia


@dataclass(frozen=True)
class Requirement:
    """Un requisito regulatorio concreto (un punto del checklist)."""

    id: str
    framework: str
    article: str
    title: str
    description: str
    severity: str = "must"  # "must" | "should"


# Una comprobación recibe los hechos derivados de las trazas y devuelve el estado
# y una lista de evidencias (textos explicativos).
Check = Callable[["object"], Tuple[Status, List[str]]]


@dataclass(frozen=True)
class MappedRequirement:
    requirement: Requirement
    check: Check


@dataclass
class Mapping:
    """Conjunto versionado de requisitos de un framework."""

    framework: str
    version: str
    requirements: List[MappedRequirement] = field(default_factory=list)

    def __len__(self) -> int:
        return len(self.requirements)
