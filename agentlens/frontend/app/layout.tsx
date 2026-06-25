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
          <span className="tag">Trazas de agentes</span>
        </header>
        {children}
      </body>
    </html>
  );
}
