// Redacción de PII en cliente (paridad con sdk-python/redaction.py, E1-T04).
// Primera barrera de privacidad: nada de PII sale del proceso sin mascarar.

import type { Attributes, AttributeValue } from "@opentelemetry/api";

interface Pattern {
  name: string;
  regex: RegExp;
  token: string;
}

// Orden: patrones más específicos primero (IBAN/tarjeta antes que teléfono).
const DEFAULT_PATTERNS: Array<[string, RegExp, string]> = [
  ["email", /[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}/g, "[REDACTED_EMAIL]"],
  ["iban", /\b[A-Z]{2}\d{2}[A-Z0-9]{11,30}\b/g, "[REDACTED_IBAN]"],
  ["card", /\b(?:\d[ \-]?){13,19}\b/g, "[REDACTED_CARD]"],
  [
    // Conservador para evitar falsos positivos sobre números sueltos: exige
    // prefijo internacional, área entre paréntesis o tres grupos separados.
    "phone",
    /(?:\+\d{1,3}[ \-]?\d{2,4}(?:[ \-]?\d{2,4}){1,4}|\(\d{1,4}\)[ \-]?\d{3,4}[ \-]?\d{3,4}|\d{3}[ \-]\d{3}[ \-]\d{2,4})/g,
    "[REDACTED_PHONE]",
  ],
];

export class Redactor {
  private patterns: Pattern[];

  constructor(enabled?: string[]) {
    this.patterns = DEFAULT_PATTERNS.filter(
      ([name]) => !enabled || enabled.includes(name),
    ).map(([name, regex, token]) => ({ name, regex, token }));
  }

  /** Añade un patrón propio (p.ej. nº de empleado). */
  addPattern(name: string, regex: RegExp, token: string): void {
    this.patterns.push({ name, regex, token });
  }

  redactText(text: string): string {
    let out = text;
    for (const p of this.patterns) {
      out = out.replace(p.regex, p.token);
    }
    return out;
  }

  redactValue(value: AttributeValue): AttributeValue {
    if (typeof value === "string") {
      return this.redactText(value);
    }
    if (Array.isArray(value)) {
      return value.map((v) => (typeof v === "string" ? this.redactText(v) : v)) as AttributeValue;
    }
    return value;
  }

  redactAttributes(attributes: Attributes): Attributes {
    const out: Attributes = {};
    for (const [k, v] of Object.entries(attributes)) {
      out[k] = v === undefined ? v : this.redactValue(v);
    }
    return out;
  }
}
