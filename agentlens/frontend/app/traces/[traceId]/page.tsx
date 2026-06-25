import Link from "next/link";
import { notFound } from "next/navigation";
import { getTrace } from "@/lib/api";
import type { Span } from "@/lib/types";

export const dynamic = "force-dynamic";

function tMs(iso: string): number {
  return new Date(iso).getTime();
}

interface Positioned {
  span: Span;
  leftPct: number;
  widthPct: number;
}

// Calcula la posición de cada barra en la ventana temporal de la traza.
function layout(spans: Span[]): { rows: Positioned[]; totalMs: number; start: number } {
  const start = Math.min(...spans.map((s) => tMs(s.start_time)));
  const end = Math.max(...spans.map((s) => tMs(s.start_time) + s.duration_ms));
  const totalMs = Math.max(end - start, 0.001);
  const rows = spans
    .slice()
    .sort((a, b) => tMs(a.start_time) - tMs(b.start_time))
    .map((span) => {
      const offset = tMs(span.start_time) - start;
      return {
        span,
        leftPct: (offset / totalMs) * 100,
        widthPct: Math.max((span.duration_ms / totalMs) * 100, 1.5),
      };
    });
  return { rows, totalMs, start };
}

export default async function TraceDetailPage({
  params,
}: {
  params: { traceId: string };
}) {
  const spans = await getTrace(params.traceId);
  if (spans.length === 0) {
    notFound();
  }

  const { rows, totalMs } = layout(spans);
  const errors = spans.filter((s) => s.status_code === "STATUS_CODE_ERROR").length;

  return (
    <main className="container">
      <Link href="/" className="back">← Trazas</Link>
      <h1 className="h1">Detalle de traza</h1>
      <p className="sub mono">{params.traceId}</p>

      <div className="card">
        <div className="meta">
          <div>
            <div className="k">Spans</div>
            <div className="v">{spans.length}</div>
          </div>
          <div>
            <div className="k">Duración total</div>
            <div className="v mono">{totalMs.toFixed(1)} ms</div>
          </div>
          <div>
            <div className="k">Estado</div>
            <div className="v">
              {errors > 0 ? (
                <span className="badge-err">{errors} error(es)</span>
              ) : (
                <span className="badge-ok">ok</span>
              )}
            </div>
          </div>
        </div>
      </div>

      <div className="card">
        <div className="timeline" data-testid="timeline">
          {rows.map(({ span, leftPct, widthPct }) => {
            const isErr = span.status_code === "STATUS_CODE_ERROR";
            return (
              <div className="row" key={span.span_id}>
                <div className="label">
                  <div>{span.span_name}</div>
                  <div className="op">
                    {span.genai_operation}
                    {span.request_model ? ` · ${span.request_model}` : ""}
                  </div>
                </div>
                <div className="track">
                  <div
                    className={`bar${isErr ? " err" : ""}`}
                    style={{ left: `${leftPct}%`, width: `${widthPct}%` }}
                    title={`${span.duration_ms.toFixed(2)} ms`}
                  />
                </div>
                <div className="num mono">{span.duration_ms.toFixed(1)} ms</div>
              </div>
            );
          })}
        </div>
      </div>
    </main>
  );
}
