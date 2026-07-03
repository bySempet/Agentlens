"use client";

import { useRouter } from "next/navigation";
import type { TraceSummary } from "@/lib/types";

function fmtTime(iso: string): string {
  return new Date(iso).toLocaleString("es-ES", {
    hour12: false,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}

// Fila de traza clicable por completo (navega al detalle). Es un client component
// porque toda la fila <tr> necesita un manejador de navegación.
export default function TraceRow({ t }: { t: TraceSummary }) {
  const router = useRouter();
  return (
    <tr data-testid="trace-row" onClick={() => router.push(`/traces/${t.trace_id}`)}>
      <td>
        <div>{t.root_span_name}</div>
        <div className="mono muted">{t.trace_id}</div>
      </td>
      <td><span className="chip">{t.agent_id}</span></td>
      <td className="muted">{t.service_name}</td>
      <td className="muted mono">{fmtTime(t.start_time)}</td>
      <td className="num mono">{t.duration_ms.toFixed(1)} ms</td>
      <td className="num mono">{t.span_count}</td>
      <td className="num mono">{t.input_tokens + t.output_tokens}</td>
      <td>
        {t.error_count > 0 ? (
          <span className="badge-err">● {t.error_count} error{t.error_count > 1 ? "es" : ""}</span>
        ) : (
          <span className="badge-ok">● ok</span>
        )}
      </td>
    </tr>
  );
}
