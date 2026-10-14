"""CLI del motor de compliance.

    cat spans.json | python -m agentlens_compliance [--framework eu-ai-act|gdpr] [--pdf out.pdf]

Sin --pdf: imprime el informe JSON (cobertura + checklist).
Con --pdf: genera además un PDF firmado y su sidecar <out>.sig.json (E5-T05).
"""
import argparse
import json
import sys

from .evaluator import evaluate
from .frameworks import eu_ai_act, gdpr

_FRAMEWORKS = {"eu-ai-act": eu_ai_act, "gdpr": gdpr}


def main() -> int:
    parser = argparse.ArgumentParser(prog="agentlens_compliance")
    parser.add_argument("--framework", choices=list(_FRAMEWORKS), default="eu-ai-act")
    parser.add_argument("--pdf", help="ruta del PDF firmado a generar")
    args = parser.parse_args()

    try:
        spans = json.load(sys.stdin)
    except json.JSONDecodeError as exc:
        print(f"entrada JSON inválida: {exc}", file=sys.stderr)
        return 2
    if not isinstance(spans, list):
        print("se esperaba un array JSON de spans", file=sys.stderr)
        return 2

    report = evaluate(spans, _FRAMEWORKS[args.framework].mapping())

    if args.pdf:
        from .reporting import generate_signed_report
        signed = generate_signed_report(report, args.pdf)
        print(f"PDF firmado: {args.pdf}", file=sys.stderr)
        print(f"sidecar: {signed.sidecar_path}", file=sys.stderr)

    json.dump(report.to_dict(), sys.stdout, ensure_ascii=False, indent=2)
    print()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
