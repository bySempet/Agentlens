import { getTraces, usingFixtures } from "@/lib/api";
import TraceRow from "./TraceRow";

export const dynamic = "force-dynamic";

export default async function TracesPage() {
  const traces = await getTraces();

  return (
    <main className="container">
      <h1 className="h1">Trazas</h1>
      <p className="sub">Actividad reciente de agentes, por traza.</p>
      {usingFixtures && (
        <div className="banner">
          Mostrando datos de ejemplo. Configura <span className="mono">AGENTLENS_API_URL</span>{" "}
          y <span className="mono">AGENTLENS_API_KEY</span> para conectar con la API de trazas.
        </div>
      )}

      <table>
        <thead>
          <tr>
            <th>Traza</th>
            <th>Agente</th>
            <th>Servicio</th>
            <th>Inicio</th>
            <th className="num">Duración</th>
            <th className="num">Spans</th>
            <th className="num">Tokens</th>
            <th>Estado</th>
          </tr>
        </thead>
        <tbody>
          {traces.map((t) => (
            <TraceRow key={t.trace_id} t={t} />
          ))}
        </tbody>
      </table>
    </main>
  );
}
