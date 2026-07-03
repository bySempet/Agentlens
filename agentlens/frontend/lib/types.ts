// Tipos del contrato de la API de trazas (api/spec/openapi.yaml).

export interface TraceSummary {
  trace_id: string;
  root_span_name: string;
  service_name: string;
  agent_id: string;
  start_time: string; // ISO 8601
  duration_ms: number;
  span_count: number;
  error_count: number;
  input_tokens: number;
  output_tokens: number;
}

export interface Span {
  span_id: string;
  parent_span_id: string;
  span_name: string;
  genai_operation: string;
  request_model: string;
  start_time: string; // ISO 8601
  duration_ms: number;
  status_code: string;
  status_message: string;

  // Contenido GenAI para la vista de conversación (E3-T04). Vacío, redactado o
  // referencia (agentlens://payload/...) según la política de privacidad.
  system_instructions?: string;
  input_messages?: string;
  output_messages?: string;
  tool_arguments?: string;
  tool_result?: string;
}

export interface AgentCost {
  agent_id: string;
  input_tokens: number;
  output_tokens: number;
  cost_usd: number;
}

export interface ModelCost {
  model: string;
  input_tokens: number;
  output_tokens: number;
  cost_usd: number;
}

export interface CostSummary {
  by_agent: AgentCost[];
  by_model: ModelCost[];
  total_input_tokens: number;
  total_output_tokens: number;
  total_cost_usd: number;
}
