"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import type { TraceSummary } from "@/lib/types";

// URL pública del WebSocket de la API (p.ej. ws://localhost:8080/v1/stream) y la
// API key. Deben exponerse con prefijo NEXT_PUBLIC_ para llegar al navegador.
const WS_URL = process.env.NEXT_PUBLIC_AGENTLENS_WS_URL;
const API_KEY = process.env.NEXT_PUBLIC_AGENTLENS_API_KEY ?? "";

type Status = "disabled" | "connecting" | "live" | "closed";

// LiveFeed muestra en tiempo real las trazas nuevas (E3-T05). Si no hay WS
// configurado, queda en modo informativo (no rompe el render de la lista).
export default function LiveFeed() {
  const [status, setStatus] = useState<Status>(WS_URL ? "connecting" : "disabled");
  const [traces, setTraces] = useState<TraceSummary[]>([]);

  useEffect(() => {
    if (!WS_URL) return;
    const url = API_KEY ? `${WS_URL}?api_key=${encodeURIComponent(API_KEY)}` : WS_URL;
    const ws = new WebSocket(url);
    ws.onopen = () => setStatus("live");
    ws.onclose = () => setStatus("closed");
    ws.onerror = () => setStatus("closed");
    ws.onmessage = (ev) => {
      try {
        const t = JSON.parse(ev.data) as TraceSummary;
        setTraces((prev) => [t, ...prev].slice(0, 20));
      } catch {
        /* ignora mensajes no-JSON */
      }
    };
    return () => ws.close();
  }, []);

  const dotClass =
    status === "live" ? "badge-ok" : status === "closed" ? "badge-err" : "muted";
  const label =
    status === "live"
      ? "en vivo"
      : status === "connecting"
        ? "conectando…"
        : status === "closed"
          ? "desconectado"
          : "en vivo (configura NEXT_PUBLIC_AGENTLENS_WS_URL)";

  return (
    <div className="livefeed" data-testid="livefeed">
      <div className="live-status">
        <span className={dotClass}>●</span> <span className="muted">{label}</span>
      </div>
      {traces.length > 0 && (
        <div className="live-list">
          {traces.map((t) => (
            <Link key={t.trace_id} href={`/traces/${t.trace_id}`} className="live-item">
              <span className="chip">{t.agent_id}</span>
              <span>{t.root_span_name}</span>
              <span className="mono muted">{t.trace_id}</span>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
