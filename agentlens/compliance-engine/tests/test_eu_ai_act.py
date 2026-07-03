"""Tests del mapeo EU AI Act (E5-T02) y del modelo/evaluador (E5-T01)."""
from agentlens_compliance import Status, evaluate
from agentlens_compliance.frameworks import eu_ai_act


# Resource común que identifica agente/tenant, versiona el esquema y fija retención.
RESOURCE = {
    "agentlens.agent.id": "support-bot",
    "agentlens.tenant.id": "acme",
    "agentlens.conventions.version": "genai-1.0",
    "agentlens.retention.days": "365",
}


def compliant_spans():
    """Trazas ricas que aportan evidencia para los 40 puntos."""
    return [
        {
            "name": "invoke_agent support-bot",
            "resource": RESOURCE,
            "start_time": 1,
            "status_code": "STATUS_CODE_OK",
            "attributes": {
                "gen_ai.operation.name": "invoke_agent",
                "gen_ai.system": "openai",
                "agentlens.human_oversight": "true",
                "agentlens.escalated": "true",
                "gen_ai.system_instructions": "Eres un asistente.",
            },
        },
        {
            "name": "chat gpt-4o",
            "resource": RESOURCE,
            "start_time": 2,
            "status_code": "STATUS_CODE_OK",
            "attributes": {
                "gen_ai.operation.name": "chat",
                "gen_ai.request.model": "gpt-4o",
                "gen_ai.usage.input_tokens": "120",
                "gen_ai.usage.output_tokens": "40",
                "gen_ai.input.messages": "hola",
                "gen_ai.output.messages": "buenas",
                "user.email": "[REDACTED_EMAIL]",
            },
        },
        {
            "name": "execute_tool lookup",
            "resource": RESOURCE,
            "start_time": 3,
            "status_code": "STATUS_CODE_ERROR",
            "attributes": {
                "gen_ai.operation.name": "execute_tool",
                "gen_ai.tool.name": "lookup",
            },
        },
    ]


def test_mapping_has_40_points_and_version():
    m = eu_ai_act.mapping()
    assert len(m) == 40, f"esperados 40 puntos, hay {len(m)}"
    assert m.version == "2024-1689"
    assert m.framework == "EU AI Act"


def test_full_coverage_with_rich_traces():
    report = evaluate(compliant_spans(), eu_ai_act.mapping())
    assert report.total == 40
    # Con evidencia para todos los puntos, cobertura total.
    assert report.coverage() == 1.0, report.to_dict()["totals"]
    assert report.missing == 0


def test_articles_present():
    report = evaluate(compliant_spans(), eu_ai_act.mapping())
    articles = {r.requirement.article for r in report.results}
    for art in ("Art. 12", "Art. 14", "Art. 50", "Art. 86"):
        assert art in articles


def test_gaps_detected_with_minimal_traces():
    minimal = [{"name": "invoke_agent x", "resource": {"agentlens.tenant.id": "acme"}, "start_time": 1}]
    report = evaluate(minimal, eu_ai_act.mapping())
    assert report.coverage() < 0.5
    gap_ids = {r.requirement.id for r in report.gaps()}
    # Sin supervisión, escalado, modelo ni retención -> deben aparecer como gaps.
    for rid in ("AIA-14-01", "AIA-14-02", "AIA-12-08", "AIA-12-12"):
        assert rid in gap_ids
    # El registro básico y el tenant sí están cubiertos.
    covered = {r.requirement.id for r in report.results if r.status == Status.COVERED}
    assert "AIA-12-01" in covered and "AIA-12-04" in covered


def test_report_serializes():
    report = evaluate(compliant_spans(), eu_ai_act.mapping())
    d = report.to_dict()
    assert d["framework"] == "EU AI Act"
    assert d["coverage"] == 1.0
    assert len(d["requirements"]) == 40
    assert {"id", "article", "status", "evidence"} <= set(d["requirements"][0])
