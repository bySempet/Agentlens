"""CLI: genera un informe de cumplimiento EU AI Act a partir de spans (JSON).

    cat spans.json | python -m agentlens_compliance

Entrada: un array JSON de spans (name, attributes, resource, start_time,
status_code). Salida: el informe en JSON (cobertura + checklist).
"""
import json
import sys

from .evaluator import evaluate
from .frameworks import eu_ai_act


def main() -> int:
    try:
        spans = json.load(sys.stdin)
    except json.JSONDecodeError as exc:
        print(f"entrada JSON inválida: {exc}", file=sys.stderr)
        return 2
    if not isinstance(spans, list):
        print("se esperaba un array JSON de spans", file=sys.stderr)
        return 2

    report = evaluate(spans, eu_ai_act.mapping())
    json.dump(report.to_dict(), sys.stdout, ensure_ascii=False, indent=2)
    print()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
