// Capa de datos del dashboard. Si AGENTLENS_API_URL está configurada, consulta
// la API hot path (E3-T01/T02) con la API key; si no, cae a fixtures para poder
// desarrollar/demostrar el dashboard sin backend.
import { fixtureSpans, fixtureTraces } from "./fixtures";
import type { Span, TraceSummary } from "./types";

const API_URL = process.env.AGENTLENS_API_URL;
const API_KEY = process.env.AGENTLENS_API_KEY ?? "";

export const usingFixtures = !API_URL;

function authHeaders(): HeadersInit {
  return API_KEY ? { Authorization: `Bearer ${API_KEY}` } : {};
}

export async function getTraces(limit = 50, offset = 0): Promise<TraceSummary[]> {
  if (!API_URL) {
    return fixtureTraces;
  }
  const res = await fetch(
    `${API_URL}/v1/traces?limit=${limit}&offset=${offset}`,
    { headers: authHeaders(), cache: "no-store" },
  );
  if (!res.ok) {
    throw new Error(`API /v1/traces -> ${res.status}`);
  }
  const body = await res.json();
  return body.traces as TraceSummary[];
}

export async function getTrace(traceId: string): Promise<Span[]> {
  if (!API_URL) {
    return fixtureSpans[traceId] ?? [];
  }
  const res = await fetch(`${API_URL}/v1/traces/${encodeURIComponent(traceId)}`, {
    headers: authHeaders(),
    cache: "no-store",
  });
  if (res.status === 404) {
    return [];
  }
  if (!res.ok) {
    throw new Error(`API /v1/traces/${traceId} -> ${res.status}`);
  }
  const body = await res.json();
  return body.spans as Span[];
}
