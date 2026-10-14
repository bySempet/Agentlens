// Gate de latencia del SDK (E0-T03 / criterio de E1-T03).
//
// Mide lo que paga el agente por cada acción instrumentada en el hot path (abrir
// y cerrar agent() + tool() con atributos, PII y un payload grande) con un backend
// lento. Entre acciones se cede el event loop, como un agente real que espera al
// LLM, para que el BatchSpanProcessor exporte de verdad en segundo plano. El
// build falla si el p99 supera el presupuesto (5 ms, AGENTLENS_LATENCY_BUDGET_MS)
// o si alguna acción supera SPIKE_FACTOR veces el presupuesto.
import { test } from "node:test";
import assert from "node:assert/strict";
import { trace } from "@opentelemetry/api";
import { ExportResultCode, type ExportResult } from "@opentelemetry/core";
import {
  BasicTracerProvider,
  SimpleSpanProcessor,
  type ReadableSpan,
  type SpanExporter,
} from "@opentelemetry/sdk-trace-base";
import { instrument, agent, tool, shutdown, GenAI } from "../src/index.js";

const BUDGET_MS = Number(process.env.AGENTLENS_LATENCY_BUDGET_MS ?? "5");
// Ninguna acción puede superar este múltiplo del presupuesto: detecta trabajo
// pesado esporádico (p.ej. procesar un lote entero dentro de span.end()) que el
// p99 no ve, con margen para pausas del GC.
const SPIKE_FACTOR = 4;
const ITERATIONS = 2000;
const WARMUP = 200;
const BIG_PAYLOAD = "respuesta del modelo ".repeat(500); // ~10 KB -> se externaliza

/** Backend lento: cada export responde a los 50 ms (red lenta / Collector saturado). */
class SlowExporter implements SpanExporter {
  exported = 0;
  constructor(private readonly delayMs = 50) {}
  export(spans: ReadableSpan[], done: (r: ExportResult) => void): void {
    setTimeout(() => {
      this.exported += spans.length;
      done({ code: ExportResultCode.SUCCESS });
    }, this.delayMs);
  }
  async shutdown(): Promise<void> {}
}

/** Export que bloquea el hilo (trabajo síncrono): solo para el meta-test. */
class BlockingExporter implements SpanExporter {
  constructor(private readonly blockMs: number) {}
  export(_spans: ReadableSpan[], done: (r: ExportResult) => void): void {
    const until = performance.now() + this.blockMs;
    while (performance.now() < until) {
      // espera activa
    }
    done({ code: ExportResultCode.SUCCESS });
  }
  async shutdown(): Promise<void> {}
}

function oneAction(i: number): void {
  agent("support-bot", () => {
    tool("lookup_customer", (s) => {
      s.setAttribute(GenAI.TOOL_CALL_ARGUMENTS, `{"email": "user${i}@example.com"}`);
      s.setAttribute(GenAI.OUTPUT_MESSAGES, BIG_PAYLOAD);
      s.setAttribute(GenAI.USAGE_OUTPUT_TOKENS, 1800);
    });
  });
}

const yieldLoop = () => new Promise<void>((r) => setImmediate(r));

interface Stats { p50: number; p99: number; max: number }

async function measureHotPath(iterations = ITERATIONS): Promise<Stats> {
  for (let i = 0; i < WARMUP; i++) {
    oneAction(i);
    await yieldLoop();
  }
  const samples: number[] = [];
  for (let i = 0; i < iterations; i++) {
    const t0 = performance.now();
    oneAction(i);
    samples.push(performance.now() - t0);
    await yieldLoop();
  }
  samples.sort((a, b) => a - b);
  return {
    p50: samples[Math.floor(samples.length / 2)],
    p99: samples[Math.floor(samples.length * 0.99) - 1],
    max: samples[samples.length - 1],
  };
}

function checkBudget(s: Stats, budgetMs = BUDGET_MS): void {
  const detail = `(p50=${s.p50.toFixed(3)} ms, p99=${s.p99.toFixed(3)} ms, max=${s.max.toFixed(3)} ms)`;
  assert.ok(s.p99 <= budgetMs, `overhead del SDK por encima del presupuesto de ${budgetMs} ms ${detail}`);
  assert.ok(
    s.max <= budgetMs * SPIKE_FACTOR,
    `pico de overhead por encima de ${SPIKE_FACTOR}x el presupuesto de ${budgetMs} ms ${detail}`,
  );
}

test("E0-T03: gate de latencia con backend lento", async () => {
  const exporter = new SlowExporter();
  const provider = instrument({
    tenantId: "acme", agentId: "support-bot", redactPii: true, payloadMode: "reference", exporter,
  });
  const s = await measureHotPath();
  await provider.forceFlush();
  console.log(
    `[E0-T03] hot path con backend lento: p50=${s.p50.toFixed(3)} ms p99=${s.p99.toFixed(3)} ms ` +
      `max=${s.max.toFixed(3)} ms (presupuesto ${BUDGET_MS} ms)`,
  );
  assert.ok(exporter.exported > 0, "el pipeline real debe exportar en segundo plano");
  checkBudget(s);
  await shutdown();
});

test("E0-T03: el gate detecta export síncrono en el hot path", async () => {
  trace.disable();
  const p = new BasicTracerProvider();
  p.addSpanProcessor(new SimpleSpanProcessor(new BlockingExporter(10)));
  trace.setGlobalTracerProvider(p);

  const s = await measureHotPath(50);
  assert.throws(() => checkBudget(s), /presupuesto/);
  trace.disable();
});
