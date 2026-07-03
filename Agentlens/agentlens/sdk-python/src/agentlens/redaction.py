"""Redacción de PII en el lado del cliente.

Esta es la primera de las dos barreras de privacidad (la segunda es Presidio en
el Collector). El objetivo es que ningún dato personal salga del proceso del
agente sin mascarar, cumpliendo el principio de minimización (GDPR Art. 5).

El núcleo es puro y testeable: ``Redactor.redact_text`` no tiene efectos
secundarios y es trivial de cubrir con tests.
"""
from __future__ import annotations

import re
from dataclasses import dataclass
from typing import Iterable, List, Tuple

# Orden importa: los patrones más específicos van primero para evitar que un
# patrón genérico (teléfono) "se coma" parte de uno específico (IBAN, tarjeta).
_DEFAULT_PATTERNS: List[Tuple[str, str, str]] = [
    (
        "email",
        r"[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}",
        "[REDACTED_EMAIL]",
    ),
    (
        "iban",
        r"\b[A-Z]{2}\d{2}[A-Z0-9]{11,30}\b",
        "[REDACTED_IBAN]",
    ),
    (
        # 13-19 dígitos, opcionalmente separados por espacios o guiones.
        "card",
        r"\b(?:\d[ \-]?){13,19}\b",
        "[REDACTED_CARD]",
    ),
    (
        # Conservador: requiere prefijo + o paréntesis para reducir falsos
        # positivos sobre números normales.
        "phone",
        r"(?:\+\d{1,3}[ \-]?)?(?:\(\d{1,4}\)[ \-]?)?\d{3,4}[ \-]?\d{3,4}[ \-]?\d{0,4}",
        "[REDACTED_PHONE]",
    ),
]


@dataclass
class Pattern:
    name: str
    regex: "re.Pattern"
    token: str


class Redactor:
    """Aplica un conjunto de patrones de redacción sobre texto y atributos."""

    def __init__(self, patterns: Iterable[Tuple[str, str, str]] = _DEFAULT_PATTERNS,
                 enabled: Iterable[str] | None = None):
        self._patterns: List[Pattern] = []
        enabled_set = set(enabled) if enabled is not None else None
        for name, rgx, token in patterns:
            if enabled_set is not None and name not in enabled_set:
                continue
            self._patterns.append(Pattern(name, re.compile(rgx), token))

    def add_pattern(self, name: str, regex: str, token: str) -> None:
        """Permite a un cliente añadir patrones propios (p.ej. nº de empleado)."""
        self._patterns.append(Pattern(name, re.compile(regex), token))

    def redact_text(self, text: str) -> str:
        for p in self._patterns:
            text = p.regex.sub(p.token, text)
        return text

    def redact_value(self, value):
        """Redacta solo valores de tipo string; otros tipos pasan intactos."""
        if isinstance(value, str):
            return self.redact_text(value)
        if isinstance(value, (list, tuple)):
            return type(value)(self.redact_value(v) for v in value)
        return value

    def redact_attributes(self, attributes: dict) -> dict:
        """Devuelve un nuevo dict con los valores redactados (no muta el original)."""
        return {k: self.redact_value(v) for k, v in attributes.items()}
