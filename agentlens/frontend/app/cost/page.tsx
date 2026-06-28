import { getCost, usingFixtures } from "@/lib/api";

export const dynamic = "force-dynamic";

function usd(n: number): string {
  return "$" + n.toFixed(2);
}

function tokens(n: number): string {
  return n.toLocaleString("es-ES");
}

export default async function CostPage() {
  const cost = await getCost();

  return (
    <main className="container">
      <h1 className="h1">Coste</h1>
      <p className="sub">Coste estimado de tokens por agente y por modelo.</p>
      {usingFixtures && (
        <div className="banner">
          Mostrando datos de ejemplo. Configura <span className="mono">AGENTLENS_API_URL</span>{" "}
          y <span className="mono">AGENTLENS_API_KEY</span> para conectar con la API.
        </div>
      )}

      <div className="kpis">
        <div className="kpi">
          <div className="k">Coste total</div>
          <div className="v cost">{usd(cost.total_cost_usd)}</div>
        </div>
        <div className="kpi">
          <div className="k">Tokens de entrada</div>
          <div className="v mono">{tokens(cost.total_input_tokens)}</div>
        </div>
        <div className="kpi">
          <div className="k">Tokens de salida</div>
          <div className="v mono">{tokens(cost.total_output_tokens)}</div>
        </div>
      </div>

      <div className="grid2">
        <div className="card">
          <p className="section-title">Por agente</p>
          <table>
            <thead>
              <tr><th>Agente</th><th className="num">Tokens</th><th className="num">Coste</th></tr>
            </thead>
            <tbody>
              {cost.by_agent.map((a) => (
                <tr key={a.agent_id} data-testid="agent-cost">
                  <td><span className="chip">{a.agent_id}</span></td>
                  <td className="num mono">{tokens(a.input_tokens + a.output_tokens)}</td>
                  <td className="num mono">{usd(a.cost_usd)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        <div className="card">
          <p className="section-title">Por modelo</p>
          <table>
            <thead>
              <tr><th>Modelo</th><th className="num">Tokens</th><th className="num">Coste</th></tr>
            </thead>
            <tbody>
              {cost.by_model.map((m) => (
                <tr key={m.model} data-testid="model-cost">
                  <td className="mono">{m.model || "(desconocido)"}</td>
                  <td className="num mono">{tokens(m.input_tokens + m.output_tokens)}</td>
                  <td className="num mono">{usd(m.cost_usd)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </main>
  );
}
