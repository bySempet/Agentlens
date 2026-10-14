"""Generación de informes de compliance firmados y verificables (E5-T05).

Produce un PDF del informe y un manifiesto firmado (sidecar ``.sig.json``) con el
hash del PDF, la cobertura, un sello de tiempo y la huella de la clave firmante.
La verificación recomputa el hash y valida la firma con una clave pública de
confianza aportada por quien verifica (nunca con la que viaja en el sidecar):
cualquier alteración del PDF o del manifiesto, o una re-firma con otra clave, la
invalida.
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


def _build_manifest(
    report: Report, pdf_sha256: str, signer: str, key_fingerprint: str
) -> Dict[str, Any]:
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
        "signer_key_sha256": key_fingerprint,
    }


def generate_signed_report(
    report: Report,
    pdf_path: str,
    private_pem: Optional[bytes] = None,
    signer: str = "AgentLens Compliance Reporter",
) -> SignedReport:
    """Renderiza el PDF, lo firma y escribe el sidecar ``<pdf>.sig.json``.

    Si no se pasa clave privada, genera un par efímero (solo para demo/tests: un
    informe así solo es verificable conservando ``SignedReport.public_key_pem``).
    La clave pública se deriva siempre de la privada.
    """
    # Imports perezosos: reportlab y cryptography son el extra opcional `pdf`.
    from .report_pdf import render_pdf
    from .signing import (
        generate_keypair,
        public_key_fingerprint,
        public_pem_from_private,
        sha256_file,
        sign_manifest,
    )

    if private_pem is None:
        private_pem, _ = generate_keypair()
    public_pem = public_pem_from_private(private_pem)

    render_pdf(report, pdf_path)
    pdf_sha256 = sha256_file(pdf_path)

    manifest = _build_manifest(
        report, pdf_sha256, signer, public_key_fingerprint(public_pem)
    )
    signature = sign_manifest(manifest, private_pem)

    sidecar_path = pdf_path + ".sig.json"
    with open(sidecar_path, "w", encoding="utf-8") as fh:
        json.dump(
            {"manifest": manifest, "signature": signature,
             "public_key_pem": public_pem.decode("utf-8")},
            fh, ensure_ascii=False, indent=2,
        )

    return SignedReport(pdf_path, sidecar_path, manifest, signature, public_pem.decode("utf-8"))


def verify_signed_report(
    pdf_path: str, sidecar_path: str, trusted_public_pem: bytes
) -> Tuple[bool, str]:
    """Verifica que el PDF no ha cambiado y que lo firmó la clave de confianza.

    ``trusted_public_pem`` debe obtenerse por un canal independiente del informe
    (clave publicada por la organización emisora). La ``public_key_pem`` del
    sidecar es solo informativa: aceptarla permitiría re-firmar un informe falso
    con cualquier clave.
    """
    from .signing import public_key_fingerprint, sha256_file, verify_manifest

    with open(sidecar_path, "r", encoding="utf-8") as fh:
        bundle = json.load(fh)

    manifest = bundle["manifest"]
    signature = bundle["signature"]

    if sha256_file(pdf_path) != manifest.get("pdf_sha256"):
        return False, "el hash del PDF no coincide (documento alterado)"
    if manifest.get("signer_key_sha256") != public_key_fingerprint(trusted_public_pem):
        return False, "el informe no está firmado por la clave de confianza"
    if not verify_manifest(manifest, signature, trusted_public_pem):
        return False, "firma del manifiesto inválida"
    return True, "OK: informe íntegro y firmado por la clave de confianza"
