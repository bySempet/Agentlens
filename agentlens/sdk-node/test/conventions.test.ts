import { test } from "node:test";
import assert from "node:assert/strict";
import { GenAI, canonicalKey, normalizeAttributes } from "../src/conventions.js";

test("canonicalKey resuelve alias conocidos", () => {
  assert.equal(canonicalKey("gen_ai.usage.prompt_tokens"), GenAI.USAGE_INPUT_TOKENS);
  assert.equal(canonicalKey("llm.token_count.completion"), GenAI.USAGE_OUTPUT_TOKENS);
  assert.equal(canonicalKey("ai.model.id"), GenAI.REQUEST_MODEL);
  assert.equal(canonicalKey("custom.attr"), "custom.attr");
});

test("normalizeAttributes renombra claves legacy", () => {
  const out = normalizeAttributes({
    "gen_ai.usage.prompt_tokens": 120,
    "gen_ai.usage.completion_tokens": 40,
    "llm.request.model": "gpt-4o",
    unrelated: "x",
  });
  assert.equal(out[GenAI.USAGE_INPUT_TOKENS], 120);
  assert.equal(out[GenAI.USAGE_OUTPUT_TOKENS], 40);
  assert.equal(out[GenAI.REQUEST_MODEL], "gpt-4o");
  assert.equal(out["unrelated"], "x");
  assert.ok(!("gen_ai.usage.prompt_tokens" in out));
});

test("normalizeAttributes no pisa la canónica existente", () => {
  const out = normalizeAttributes({
    [GenAI.USAGE_INPUT_TOKENS]: 100,
    "gen_ai.usage.prompt_tokens": 999,
  });
  assert.equal(out[GenAI.USAGE_INPUT_TOKENS], 100);
  assert.ok(!("gen_ai.usage.prompt_tokens" in out));
});

test("distintos esquemas convergen al mismo contrato", () => {
  const legacy = normalizeAttributes({ "llm.token_count.prompt": 10, "llm.token_count.completion": 5 });
  const vercel = normalizeAttributes({ "ai.usage.promptTokens": 10, "ai.usage.completionTokens": 5 });
  assert.deepEqual(legacy, { [GenAI.USAGE_INPUT_TOKENS]: 10, [GenAI.USAGE_OUTPUT_TOKENS]: 5 });
  assert.deepEqual(legacy, vercel);
});
