// Datos de ejemplo para desarrollo/demo y para renderizar el dashboard sin un
// backend en marcha. Se usan cuando AGENTLENS_API_URL no está configurada.
import type { CostSummary, MaturitySummary, Span, TraceSummary } from "./types";

export const fixtureTraces: TraceSummary[] = [
  {
    trace_id: "9f2c1a7b3e",
    root_span_name: "invoke_agent support-bot",
    service_name: "support-agent",
    agent_id: "support-bot",
    start_time: "2026-06-25T10:00:00.000Z",
    duration_ms: 5.0,
    span_count: 2,
    error_count: 0,
    input_tokens: 120,
    output_tokens: 40,
  },
  {
    trace_id: "4b8d0c5f1a",
    root_span_name: "invoke_agent support-bot",
    service_name: "support-agent",
    agent_id: "support-bot",
    start_time: "2026-06-25T10:05:00.000Z",
    duration_ms: 2.0,
    span_count: 1,
    error_count: 1,
    input_tokens: 0,
    output_tokens: 0,
  },
  {
    trace_id: "1c6e9a2d7f",
    root_span_name: "invoke_agent biller",
    service_name: "billing-agent",
    agent_id: "biller",
    start_time: "2026-06-25T09:58:12.000Z",
    duration_ms: 18.4,
    span_count: 4,
    error_count: 0,
    input_tokens: 900,
    output_tokens: 320,
  },
];

export const fixtureCost: CostSummary = {
  by_agent: [
    { agent_id: "biller", input_tokens: 900_000, output_tokens: 320_000, cost_usd: 7.05 },
    { agent_id: "support-bot", input_tokens: 120_000, output_tokens: 40_000, cost_usd: 0.7 },
  ],
  by_model: [
    { model: "claude-sonnet-4-6", input_tokens: 900_000, output_tokens: 320_000, cost_usd: 7.5 },
    { model: "gpt-4o", input_tokens: 120_000, output_tokens: 40_000, cost_usd: 0.7 },
  ],
  total_input_tokens: 1_020_000,
  total_output_tokens: 360_000,
  total_cost_usd: 8.2,
};

export const fixtureMaturity: MaturitySummary = {
  overall_coverage: 0.86,
  frameworks: [
    {
      framework: "EU AI Act",
      version: "2024-1689",
      coverage: 0.9,
      band: "avanzado",
      totals: { total: 40, covered: 36, partial: 0, missing: 4 },
      by_article: [
        { article: "Art. 12", total: 12, covered: 12, coverage: 1.0 },
        { article: "Art. 14", total: 8, covered: 6, coverage: 0.75 },
        { article: "Art. 50", total: 8, covered: 7, coverage: 0.875 },
        { article: "Art. 86", total: 7, covered: 7, coverage: 1.0 },
        { article: "Transversal", total: 5, covered: 4, coverage: 0.8 },
      ],
    },
    {
      framework: "GDPR",
      version: "2016-679",
      coverage: 0.82,
      band: "gestionado",
      totals: { total: 17, covered: 14, partial: 0, missing: 3 },
      by_article: [
        { article: "Art. 5(1)(a)", total: 1, covered: 1, coverage: 1.0 },
        { article: "Art. 5(1)(c)", total: 1, covered: 1, coverage: 1.0 },
        { article: "Art. 30", total: 4, covered: 4, coverage: 1.0 },
        { article: "Art. 15", total: 3, covered: 2, coverage: 0.667 },
        { article: "Art. 17", total: 2, covered: 1, coverage: 0.5 },
      ],
    },
  ],
};

export const fixtureSpans: Record<string, Span[]> = {
  "9f2c1a7b3e": [
    {
      span_id: "s1",
      parent_span_id: "",
      span_name: "invoke_agent support-bot",
      genai_operation: "invoke_agent",
      request_model: "",
      start_time: "2026-06-25T10:00:00.000Z",
      duration_ms: 5.0,
      status_code: "STATUS_CODE_OK",
      status_message: "",
      system_instructions: "Eres el asistente de soporte de ACME. Sé conciso.",
    },
    {
      span_id: "s2",
      parent_span_id: "s1",
      span_name: "chat gpt-4o",
      genai_operation: "chat",
      request_model: "gpt-4o",
      start_time: "2026-06-25T10:00:00.001Z",
      duration_ms: 3.0,
      status_code: "STATUS_CODE_OK",
      status_message: "",
      input_messages:
        '[{"role":"user","content":"¿Cuál es el estado de mi pedido 1234?"}]',
      output_messages:
        '[{"role":"assistant","content":"Tu pedido 1234 está en reparto y llega hoy."}]',
    },
  ],
  "4b8d0c5f1a": [
    {
      span_id: "s3",
      parent_span_id: "",
      span_name: "invoke_agent support-bot",
      genai_operation: "invoke_agent",
      request_model: "",
      start_time: "2026-06-25T10:05:00.000Z",
      duration_ms: 2.0,
      status_code: "STATUS_CODE_ERROR",
      status_message: "tool timeout",
    },
  ],
  "1c6e9a2d7f": [
    {
      span_id: "r1",
      parent_span_id: "",
      span_name: "invoke_agent biller",
      genai_operation: "invoke_agent",
      request_model: "",
      start_time: "2026-06-25T09:58:12.000Z",
      duration_ms: 18.4,
      status_code: "STATUS_CODE_OK",
      status_message: "",
    },
    {
      span_id: "r2",
      parent_span_id: "r1",
      span_name: "chat claude-sonnet-4-6",
      genai_operation: "chat",
      request_model: "claude-sonnet-4-6",
      start_time: "2026-06-25T09:58:12.002Z",
      duration_ms: 9.1,
      status_code: "STATUS_CODE_OK",
      status_message: "",
    },
    {
      span_id: "r3",
      parent_span_id: "r1",
      span_name: "execute_tool fetch_invoice",
      genai_operation: "execute_tool",
      request_model: "",
      start_time: "2026-06-25T09:58:12.012Z",
      duration_ms: 5.0,
      status_code: "STATUS_CODE_OK",
      status_message: "",
    },
    {
      span_id: "r4",
      parent_span_id: "r1",
      span_name: "chat claude-sonnet-4-6",
      genai_operation: "chat",
      request_model: "claude-sonnet-4-6",
      start_time: "2026-06-25T09:58:12.018Z",
      duration_ms: 0.4,
      status_code: "STATUS_CODE_OK",
      status_message: "",
    },
  ],
};
