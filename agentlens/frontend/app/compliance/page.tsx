import { complianceUsingFixtures, getComplianceMaturity } from "@/lib/api";
import type { FrameworkMaturity } from "@/lib/types";

export const dynamic = "force-dynamic";

function pct(n: number): string {
  return (n * 100).toFixed(0) + "%";
}

function bandClass(band: string): string {
  if (band === "avanzado") return "badge-ok";
  if (band === "gestionado") return "muted";
  return "badge-err";
}

function Bar({ value }: { value: number }) {
  const cls = value >= 0.9 ? "ok" : value >= 0.7 ? "mid" : "low";
  return (
    <div className="cov-track">
      <div className={`cov-bar ${cls}`} style={{ width: `${Math.round(value * 100)}%` }} />
    </div>
  );
}

function FrameworkCard({ f }: { f: FrameworkMaturity }) {
  return (
    <div className="card" data-testid="framework-card">
      <div className="fw-head">
        <div>
          <div className="fw-name">{f.framework}</div>
          <div className="muted mono">v{f.version}</div>
        </div>
        <div className="fw-score">
          <span className="fw-pct">{pct(f.coverage)}</span>
          <span className={`chip ${bandClass(f.band)}`}>{f.band}</span>
        </div>
      </div>
      <Bar value={f.coverage} />
      <table style={{ marginTop: "12px" }}>
        <thead>
          <tr><th>Artículo</th><th className="num">Cubiertos</th><th className="num">Cobertura</th></tr>
        </thead>
        <tbody>
          {f.by_article.map((a) => (
            <tr key={a.article}>
              <td>{a.article}</td>
              <td className="num mono">{a.covered}/{a.total}</td>
              <td className="num mono">{pct(a.coverage)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export default async function CompliancePage() {
  const maturity = await getComplianceMaturity();

  return (
    <main className="container">
      <h1 className="h1">Compliance</h1>
      <p className="sub">Madurez de cumplimiento por marco regulatorio, derivada de las trazas.</p>
      {complianceUsingFixtures && (
        <div className="banner">
          Mostrando datos de ejemplo. Configura <span className="mono">AGENTLENS_COMPLIANCE_URL</span>{" "}
          para conectar con el motor de compliance.
        </div>
      )}

      <div className="kpis">
        <div className="kpi">
          <div className="k">Madurez global</div>
          <div className="v cost" data-testid="overall">{pct(maturity.overall_coverage)}</div>
        </div>
      </div>

      <div className="grid2">
        {maturity.frameworks.map((f) => (
          <FrameworkCard key={f.framework} f={f} />
        ))}
      </div>
    </main>
  );
}
