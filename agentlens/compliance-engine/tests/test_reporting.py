"""Tests del informe PDF firmado (E5-T05)."""
import json
import os

from agentlens_compliance import evaluate, generate_signed_report, verify_signed_report
from agentlens_compliance.frameworks import eu_ai_act

from test_eu_ai_act import compliant_spans


def test_generate_and_verify_signed_report(tmp_path):
    report = evaluate(compliant_spans(), eu_ai_act.mapping())
    pdf = str(tmp_path / "informe.pdf")

    signed = generate_signed_report(report, pdf)

    # El PDF existe y es un PDF real.
    assert os.path.exists(pdf)
    with open(pdf, "rb") as fh:
        assert fh.read(5) == b"%PDF-"
    assert os.path.exists(signed.sidecar_path)
    assert signed.manifest["framework"] == "EU AI Act"
    assert "generated_at" in signed.manifest and "pdf_sha256" in signed.manifest

    ok, reason = verify_signed_report(pdf, signed.sidecar_path, signed.public_key_pem.encode())
    assert ok, reason


def test_tampered_pdf_fails_verification(tmp_path):
    report = evaluate(compliant_spans(), eu_ai_act.mapping())
    pdf = str(tmp_path / "informe.pdf")
    signed = generate_signed_report(report, pdf)

    # Alteramos el PDF -> la verificación debe fallar (documento inalterable).
    with open(pdf, "ab") as fh:
        fh.write(b"%% tampered %%")

    ok, reason = verify_signed_report(pdf, signed.sidecar_path, signed.public_key_pem.encode())
    assert not ok
    assert "hash" in reason.lower()


def test_tampered_manifest_fails_verification(tmp_path):
    report = evaluate(compliant_spans(), eu_ai_act.mapping())
    pdf = str(tmp_path / "informe.pdf")
    signed = generate_signed_report(report, pdf)

    with open(signed.sidecar_path, "r+", encoding="utf-8") as fh:
        bundle = json.load(fh)
        bundle["manifest"]["coverage"] = 0.0  # manipula la cobertura
        fh.seek(0)
        json.dump(bundle, fh)
        fh.truncate()

    ok, _ = verify_signed_report(pdf, signed.sidecar_path, signed.public_key_pem.encode())
    assert not ok


def test_reforged_report_with_other_key_fails_verification(tmp_path):
    """Un atacante regenera PDF + sidecar con su propia clave: debe fallar.

    El sidecar trae una clave pública válida para su firma, por eso la
    verificación tiene que usar la clave de confianza y no la del sidecar.
    """
    from agentlens_compliance.signing import generate_keypair

    trusted_priv, trusted_pub = generate_keypair()
    pdf = str(tmp_path / "informe.pdf")
    generate_signed_report(evaluate(compliant_spans(), eu_ai_act.mapping()), pdf, trusted_priv)
    assert verify_signed_report(pdf, pdf + ".sig.json", trusted_pub)[0]

    forged = evaluate([], eu_ai_act.mapping())
    generate_signed_report(forged, pdf)  # re-firma con una clave efímera ajena

    ok, reason = verify_signed_report(pdf, pdf + ".sig.json", trusted_pub)
    assert not ok
    assert "clave de confianza" in reason


def test_signed_manifest_pins_signer_key(tmp_path):
    """Cambiar la huella del manifiesto para apuntar a otra clave rompe la firma."""
    from agentlens_compliance.signing import generate_keypair, public_key_fingerprint

    priv, pub = generate_keypair()
    _, other_pub = generate_keypair()
    pdf = str(tmp_path / "informe.pdf")
    generate_signed_report(evaluate(compliant_spans(), eu_ai_act.mapping()), pdf, priv)

    sidecar = pdf + ".sig.json"
    with open(sidecar, "r+", encoding="utf-8") as fh:
        bundle = json.load(fh)
        bundle["manifest"]["signer_key_sha256"] = public_key_fingerprint(other_pub)
        fh.seek(0)
        json.dump(bundle, fh)
        fh.truncate()

    ok, reason = verify_signed_report(pdf, sidecar, other_pub)
    assert not ok
    assert "firma" in reason
