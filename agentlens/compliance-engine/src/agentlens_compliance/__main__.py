"""CLI del motor de compliance.

    cat spans.json | python -m agentlens_compliance [--framework eu-ai-act|gdpr] \\
        [--pdf out.pdf --signing-key firma.key]
    python -m agentlens_compliance --keygen firma        # firma.key + firma.pub
    python -m agentlens_compliance --verify out.pdf --pubkey firma.pub

Sin --pdf: imprime el informe JSON (cobertura + checklist).
Con --pdf: genera además un PDF firmado y su sidecar <out>.sig.json (E5-T05). La
clave privada se lee de --signing-key o de AGENTLENS_SIGNING_KEY_FILE.
"""
import argparse
import json
import os
import sys

from .evaluator import evaluate
from .frameworks import eu_ai_act, gdpr

_FRAMEWORKS = {"eu-ai-act": eu_ai_act, "gdpr": gdpr}


def _keygen(prefix: str) -> int:
    from .signing import generate_keypair, public_key_fingerprint

    priv_pem, pub_pem = generate_keypair()
    key_path, pub_path = prefix + ".key", prefix + ".pub"
    # O_EXCL: nunca sobrescribir una clave existente; 0600: solo el propietario.
    try:
        fd = os.open(key_path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    except FileExistsError:
        print(f"{key_path} ya existe; no se sobrescribe", file=sys.stderr)
        return 1
    with os.fdopen(fd, "wb") as fh:
        fh.write(priv_pem)
    with open(pub_path, "wb") as fh:
        fh.write(pub_pem)
    print(f"clave privada: {key_path}", file=sys.stderr)
    print(f"clave pública: {pub_path}", file=sys.stderr)
    print(f"huella: {public_key_fingerprint(pub_pem)}")
    return 0


def _verify(pdf_path: str, pubkey_path: str) -> int:
    from .reporting import verify_signed_report

    with open(pubkey_path, "rb") as fh:
        trusted = fh.read()
    ok, reason = verify_signed_report(pdf_path, pdf_path + ".sig.json", trusted)
    print(reason)
    return 0 if ok else 1


def main() -> int:
    parser = argparse.ArgumentParser(prog="agentlens_compliance")
    parser.add_argument("--framework", choices=list(_FRAMEWORKS), default="eu-ai-act")
    parser.add_argument("--pdf", help="ruta del PDF firmado a generar")
    parser.add_argument("--signing-key", help="clave privada Ed25519 (PEM) para firmar el PDF")
    parser.add_argument("--keygen", metavar="PREFIX", help="genera PREFIX.key y PREFIX.pub")
    parser.add_argument("--verify", metavar="PDF", help="verifica PDF contra PDF.sig.json")
    parser.add_argument("--pubkey", help="clave pública de confianza (PEM) para --verify")
    args = parser.parse_args()

    if args.keygen:
        return _keygen(args.keygen)
    if args.verify:
        if not args.pubkey:
            parser.error("--verify requiere --pubkey (la clave del sidecar no es de confianza)")
        return _verify(args.verify, args.pubkey)

    private_pem = None
    if args.pdf:
        key_path = args.signing_key or os.environ.get("AGENTLENS_SIGNING_KEY_FILE")
        if not key_path:
            parser.error("--pdf requiere --signing-key o AGENTLENS_SIGNING_KEY_FILE")
        with open(key_path, "rb") as fh:
            private_pem = fh.read()

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
        signed = generate_signed_report(report, args.pdf, private_pem)
        print(f"PDF firmado: {args.pdf}", file=sys.stderr)
        print(f"sidecar: {signed.sidecar_path}", file=sys.stderr)

    json.dump(report.to_dict(), sys.stdout, ensure_ascii=False, indent=2)
    print()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
