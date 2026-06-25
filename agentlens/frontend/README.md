# Dashboard de trazas (Next.js) — E3-T03

App Next.js (App Router + TypeScript) que muestra la actividad de los agentes:
lista de trazas y detalle con el **timeline de spans** (waterfall), consumiendo
la API hot path (E3-T01/T02).

## Vistas

- **`/`** — lista de trazas (root span, agente, servicio, inicio, duración,
  spans, tokens, estado). Cada fila navega al detalle.
- **`/traces/{traceId}`** — detalle paso a paso: resumen de la traza y un
  timeline donde cada span se posiciona por su offset y duración reales, con el
  operation/model GenAI y marca de error.

## Datos

`lib/api.ts` consulta la API real si está configurada; si no, usa fixtures para
poder desarrollar y demostrar el dashboard sin backend.

| Variable | Descripción |
| --- | --- |
| `AGENTLENS_API_URL` | Base de la API de trazas (p.ej. `http://localhost:8080`). Sin ella, fixtures. |
| `AGENTLENS_API_KEY` | API key del tenant (cabecera `Authorization: Bearer`). |

## Desarrollo

```bash
npm install
npm run dev      # http://localhost:3000 (datos de ejemplo)

# Contra la API real:
AGENTLENS_API_URL=http://localhost:8080 AGENTLENS_API_KEY=demo-key npm run dev
```

## Build

```bash
npm run build && npm start
```

El build (type-check + lint) se ha verificado, y el render de ambas vistas
(lista + timeline) se ha comprobado con un navegador real (Chromium/Playwright).
