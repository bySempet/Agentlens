"""Render del informe de compliance a PDF (E5-T05).

reportlab es una dependencia opcional (extra ``pdf``); el import es perezoso para
que el resto del paquete funcione sin ella.
"""
from __future__ import annotations

from typing import Any, Dict, Optional

from .evaluator import Report
from .model import Status


def render_pdf(report: Report, path: str, meta: Optional[Dict[str, Any]] = None) -> None:
    """Escribe el informe en un PDF legible con el checklist y la cobertura."""
    from reportlab.lib import colors
    from reportlab.lib.pagesizes import A4
    from reportlab.lib.styles import getSampleStyleSheet
    from reportlab.lib.units import mm
    from reportlab.platypus import (
        Paragraph,
        SimpleDocTemplate,
        Spacer,
        Table,
        TableStyle,
    )

    styles = getSampleStyleSheet()
    doc = SimpleDocTemplate(path, pagesize=A4, title=f"Compliance — {report.framework}")
    flow = []

    flow.append(Paragraph("AgentLens — Informe de Compliance", styles["Title"]))
    flow.append(Paragraph(f"{report.framework} · versión {report.version}", styles["Heading2"]))
    flow.append(Spacer(1, 6 * mm))

    pct = round(report.coverage() * 100, 1)
    flow.append(Paragraph(
        f"<b>Cobertura: {pct}%</b> — {report.covered} cubiertos, "
        f"{report.partial} parciales, {report.missing} faltantes de {report.total}.",
        styles["Normal"],
    ))
    if meta:
        for k in ("generated_at", "pdf_sha256", "signer"):
            if k in meta:
                flow.append(Paragraph(f"{k}: {meta[k]}", styles["Code"]))
    flow.append(Spacer(1, 6 * mm))

    color_by = {
        Status.COVERED: colors.HexColor("#2ea043"),
        Status.PARTIAL: colors.HexColor("#d29922"),
        Status.MISSING: colors.HexColor("#f85149"),
    }
    data = [["ID", "Artículo", "Requisito", "Estado"]]
    for r in report.results:
        data.append([r.requirement.id, r.requirement.article, r.requirement.title, r.status.value])

    table = Table(data, colWidths=[26 * mm, 26 * mm, 95 * mm, 22 * mm], repeatRows=1)
    style = [
        ("BACKGROUND", (0, 0), (-1, 0), colors.HexColor("#0b0f17")),
        ("TEXTCOLOR", (0, 0), (-1, 0), colors.white),
        ("FONTSIZE", (0, 0), (-1, -1), 7),
        ("VALIGN", (0, 0), (-1, -1), "TOP"),
        ("GRID", (0, 0), (-1, -1), 0.25, colors.HexColor("#d0d7de")),
        ("ROWBACKGROUNDS", (0, 1), (-1, -1), [colors.white, colors.HexColor("#f6f8fa")]),
    ]
    for i, r in enumerate(report.results, start=1):
        style.append(("TEXTCOLOR", (3, i), (3, i), color_by[r.status]))
    table.setStyle(TableStyle(style))
    flow.append(table)

    doc.build(flow)
