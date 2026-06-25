import { test } from "node:test";
import assert from "node:assert/strict";
import { InMemorySpanExporter } from "@opentelemetry/sdk-trace-base";
import { instrument, agent, tool, shutdown, GenAI } from "../src/index.js";

test("end-to-end: enriquecimiento + redacción + externalización + convención", async () => {
  const mem = new InMemorySpanExporter();
  const provider = instrument({
    tenantId: "acme",
    agentId: "support-bot",
    serviceName: "test-svc",
    redactPii: true,
    payloadMode: "reference",
    exporter: mem,
  });

  agent("support-bot", () => {
    tool("lookup_customer", (s) => {
      s.setAttribute("user.email", "alice@example.com");
      s.setAttribute(GenAI.OUTPUT_MESSAGES, "respuesta " + "y".repeat(5000));
    });
  });

  await provider.forceFlush();
  const spans = mem.getFinishedSpans();
  assert.equal(spans.length, 2); // invoke_agent + execute_tool

  const toolSpan = spans.find((s) => s.name.startsWith("execute_tool"))!;
  // Enriquecimiento de recurso
  assert.equal(toolSpan.resource.attributes["agentlens.tenant.id"], "acme");
  assert.equal(toolSpan.resource.attributes["agentlens.sdk.language"], "node");
  // Redacción de PII
  assert.equal(toolSpan.attributes["user.email"], "[REDACTED_EMAIL]");
  // Externalización de payload grande
  assert.match(String(toolSpan.attributes[GenAI.OUTPUT_MESSAGES]), /^agentlens:\/\/payload\//);
  assert.ok("agentlens.payload.externalized" in toolSpan.attributes);
  // Convención OTel GenAI
  assert.equal(toolSpan.attributes[GenAI.OPERATION_NAME], "execute_tool");
  assert.equal(toolSpan.attributes[GenAI.TOOL_NAME], "lookup_customer");

  await shutdown();
});

test("normalización de alias legacy end-to-end", async () => {
  const mem = new InMemorySpanExporter();
  const provider = instrument({ tenantId: "acme", redactPii: false, payloadMode: "inline", exporter: mem });
  tool("x", (s) => {
    s.setAttribute("gen_ai.usage.prompt_tokens", 200);
    s.setAttribute("llm.response.model", "gpt-4o");
  });
  await provider.forceFlush();
  const span = mem.getFinishedSpans()[0];
  assert.equal(span.attributes[GenAI.USAGE_INPUT_TOKENS], 200);
  assert.equal(span.attributes[GenAI.RESPONSE_MODEL], "gpt-4o");
  assert.ok(!("gen_ai.usage.prompt_tokens" in span.attributes));
  await shutdown();
});

test("payloadMode 'none' descarta el contenido", async () => {
  const mem = new InMemorySpanExporter();
  const provider = instrument({ tenantId: "acme", payloadMode: "none", exporter: mem });
  tool("x", (s) => {
    s.setAttribute(GenAI.OUTPUT_MESSAGES, "contenido sensible");
    s.setAttribute(GenAI.TOOL_NAME, "x");
  });
  await provider.forceFlush();
  const span = mem.getFinishedSpans()[0];
  assert.ok(!(GenAI.OUTPUT_MESSAGES in span.attributes));
  assert.equal(span.attributes[GenAI.TOOL_NAME], "x");
  await shutdown();
});

test("async: agent() espera al callback antes de cerrar el span", async () => {
  const mem = new InMemorySpanExporter();
  const provider = instrument({ tenantId: "acme", exporter: mem, autoInstrument: false } as any);
  await agent("async-bot", async (s) => {
    await new Promise((r) => setTimeout(r, 5));
    s.setAttribute("done", true);
  });
  await provider.forceFlush();
  const span = mem.getFinishedSpans().find((s) => s.name.startsWith("invoke_agent"))!;
  assert.equal(span.attributes["done"], true);
  await shutdown();
});
