# Dashboard de trazas (Next.js) — E3-T03

App Next.js (App Router + TypeScript) que muestra la actividad de los agentes:
lista de trazas y detalle con el **timeline de spans** (waterfall), consumiendo
la API hot path (E3-T01/T02).

## Vistas

- **`/`** — lista de trazas (root span, agente, servicio, inicio, duración,
  spans, tokens, estado). Cada fila navega al detalle. Incluye un **live feed**
  (E3-T05) que muestra las trazas nuevas en tiempo real vía WebSocket.
- **`/cost`** — dashboard de coste (E3-T06): coste estimado de tokens (tarifa por
  modelo) agregado por agente y por modelo, con KPIs de coste/tokens totales.
- **`/traces/{traceId}`** — detalle paso a paso:
  - **Conversación (E3-T04)**: render chat-style de los atributos `gen_ai.*`
    (system / user / assistant / tool calls), tolerante a contenido vacío,
    redactado o externalizado.
  - **Timeline**: cada span posicionado por su offset y duración reales, con el
    operation/model GenAI y marca de error.

## Datos

`lib/api.ts` consulta la API real si está configurada; si no, usa fixtures para
poder desarrollar y demostrar el dashboard sin backend.

| Variable | Descripción |
| --- | --- |
| `AGENTLENS_API_URL` | Base de la API de trazas (p.ej. `http://localhost:8080`). Sin ella, fixtures. |
| `AGENTLENS_API_KEY` | API key del tenant (cabecera `Authorization: Bearer`). |
| `NEXT_PUBLIC_AGENTLENS_WS_URL` | URL del WebSocket del live feed (p.ej. `ws://localhost:8080/v1/stream`). |
| `NEXT_PUBLIC_AGENTLENS_API_KEY` | API key para el live feed (se pasa como `?api_key=`). |

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

El build (type-check + lint) se ha verificado, y el render de las vistas se
comprueba con un navegador real (Chromium/Playwright).

## E2E (Playwright)

```bash
npm run test:e2e
```

Cubre: la lista de trazas renderiza filas y navega al detalle; el detalle muestra
la conversación y el timeline; la vista de coste muestra los agregados. En CI se
ejecuta en el job `frontend-e2e`. En local, si usas un Chromium preinstalado,
exporta `PW_CHROMIUM_PATH` con la ruta al binario.
