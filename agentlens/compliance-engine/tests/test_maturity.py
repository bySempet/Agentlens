"""Tests de la madurez multi-framework (E5-T06)."""
from agentlens_compliance import maturity

from test_eu_ai_act import compliant_spans


def test_summarize_covers_frameworks():
    summary = maturity.summarize(compliant_spans())
    names = {f["framework"] for f in summary["frameworks"]}
    assert names == {"EU AI Act", "GDPR"}
    assert 0.0 <= summary["overall_coverage"] <= 1.0


def test_by_article_and_band():
    summary = maturity.summarize(compliant_spans())
    eu = next(f for f in summary["frameworks"] if f["framework"] == "EU AI Act")
    # Con trazas ricas, EU AI Act queda en banda alta y hay desglose por artículo.
    assert eu["coverage"] >= 0.9 and eu["band"] == "avanzado"
    arts = {a["article"] for a in eu["by_article"]}
    assert {"Art. 12", "Art. 14", "Art. 50", "Art. 86"} <= arts
    for a in eu["by_article"]:
        assert 0.0 <= a["coverage"] <= 1.0


def test_minimal_traces_low_band():
    minimal = [{"name": "chat", "resource": {}, "start_time": 1}]
    summary = maturity.summarize(minimal)
    assert summary["overall_coverage"] < 0.5
    for f in summary["frameworks"]:
        assert f["band"] in ("insuficiente", "inicial")
