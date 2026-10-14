"""Tests del mapeo GDPR (E5-T03)."""
from agentlens_compliance import evaluate
from agentlens_compliance.frameworks import gdpr


RESOURCE = {
    "agentlens.agent.id": "support-bot",
    "agentlens.tenant.id": "acme",
    "agentlens.conventions.version": "genai-1.0",
    "agentlens.retention.days": "365",
    "agentlens.gdpr.lawful_basis": "consent",
    "agentlens.gdpr.purpose": "atención al cliente",
}


def gdpr_compliant_spans():
    return [
        {
            "name": "execute_tool lookup_customer",
            "resource": RESOURCE,
            "start_time": 1,
            "status_code": "STATUS_CODE_ERROR",
            "attributes": {
                "agentlens.data.accessed": "customer_profile",
                "agentlens.gdpr.subject_id": "subj-abc",
                "agentlens.payload.externalized": "gen_ai.output.messages",
                "gen_ai.input.messages": "consulta",
                "gen_ai.output.messages": "respuesta",
                "user.email": "[REDACTED_EMAIL]",
            },
        },
    ]


def test_gdpr_mapping_versioned():
    m = gdpr.mapping()
    assert m.framework == "GDPR" and m.version == "2016-679"
    assert len(m) >= 15


def test_gdpr_full_coverage():
    report = evaluate(gdpr_compliant_spans(), gdpr.mapping())
    assert report.missing == 0, report.to_dict()["totals"]
    assert report.coverage() == 1.0


def test_gdpr_gaps_without_lawful_basis():
    minimal = [{"name": "chat", "resource": {"agentlens.tenant.id": "acme"}, "start_time": 1,
                "attributes": {"gen_ai.output.messages": "x"}}]
    report = evaluate(minimal, gdpr.mapping())
    gap_ids = {r.requirement.id for r in report.gaps()}
    assert "GDPR-5-01" in gap_ids   # sin base legal
    assert "GDPR-5-04" in gap_ids   # sin retención
    assert "GDPR-5-03" in gap_ids   # sin minimización (no hay PII redactada)
