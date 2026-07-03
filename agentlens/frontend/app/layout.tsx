import type { Metadata } from "next";
import Link from "next/link";
import "./globals.css";

export const metadata: Metadata = {
  title: "AgentLens — Trazas",
  description: "Observabilidad y gobernanza de agentes de IA",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="es">
      <body>
        <header className="header">
          <Link href="/" className="logo">
            Agent<span>Lens</span>
          </Link>
          <nav className="nav">
            <Link href="/">Trazas</Link>
            <Link href="/cost">Coste</Link>
          </nav>
        </header>
        {children}
      </body>
    </html>
  );
}
