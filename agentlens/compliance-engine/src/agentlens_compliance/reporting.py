"""Generación de informes de compliance firmados y verificables (E5-T05).

Produce un PDF del informe y un manifiesto firmado (sidecar ``.sig.json``) con el
hash del PDF, la cobertura y un sello de tiempo. La verificación recomputa el
hash y valida la firma: cualquier alteración del PDF o del manifiesto la invalida.
"""
from __future__ import annotations

import json
from dataclasses import dataclass
from datetime import datetime, timezone
from typing import Any, Dict, Optional, Tuple

from .evaluator import Report


@dataclass
class SignedReport:
    pdf_path: str
    sidecar_path: str
    manifest: Dict[str, Any]
    signature: str
    public_key_pem: str


def _build_manifest(report: Report, pdf_sha256: str, signer: str) -> Dict[str, Any]:
    return {
        "framework": report.framework,
        "version": report.version,
        "coverage": report.coverage(),
        "totals": {
            "total": report.total, "covered": report.covered,
            "partial": report.partial, "missing": report.missing,
        },
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "pdf_sha256": pdf_sha256,
        "signer": signer,
    }


def generate_signed_report(
    report: Report,
    pdf_path: str,
    private_pem: Optional[bytes] = None,
    public_pem: Optional[bytes] = None,
    signer: str = "AgentLens Compliance Reporter",
) -> SignedReport:
    """Renderiza el PDF, lo firma y escribe el sidecar ``<pdf>.sig.json``.

    Si no se pasan claves, genera un par efímero (para demo/tests).
    """
    # Imports perezosos: reportlab y cryptography son el extra opcional `pdf`.
    from .report_pdf import render_pdf
    from .signing import generate_keypair, sha256_file, sign_manifest

    if private_pem is None or public_pem is None:
        private_pem, public_pem = generate_keypair()

    render_pdf(report, pdf_path)
    pdf_sha256 = sha256_file(pdf_path)

    manifest = _build_manifest(report, pdf_sha256, signer)
    signature = sign_manifest(manifest, private_pem)

    sidecar_path = pdf_path + ".sig.json"
    with open(sidecar_path, "w", encoding="utf-8") as fh:
        json.dump(
            {"manifest": manifest, "signature": signature,
             "public_key_pem": public_pem.decode("utf-8")},
            fh, ensure_ascii=False, indent=2,
        )

    return SignedReport(pdf_path, sidecar_path, manifest, signature, public_pem.decode("utf-8"))


def verify_signed_report(pdf_path: str, sidecar_path: str) -> Tuple[bool, str]:
    """Verifica que el PDF no ha cambiado y que la firma del manifiesto es válida."""
    from .signing import sha256_file, verify_manifest

    with open(sidecar_path, "r", encoding="utf-8") as fh:
        bundle = json.load(fh)

    manifest = bundle["manifest"]
    signature = bundle["signature"]
    public_pem = bundle["public_key_pem"].encode("utf-8")

    if sha256_file(pdf_path) != manifest.get("pdf_sha256"):
        return False, "el hash del PDF no coincide (documento alterado)"
    if not verify_manifest(manifest, signature, public_pem):
        return False, "firma del manifiesto inválida"
    return True, "OK: informe íntegro y firma válida"
