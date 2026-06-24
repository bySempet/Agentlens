"""Agente de ejemplo de AgentLens.

Demuestra la instrumentación con spans de agente y herramienta, redacción de PII
y externalización de payloads, sin depender de ningún proveedor de LLM (usa
funciones simuladas). Para verlo contra el stack local:

    cd ../deploy && docker compose up -d      # Collector + ClickHouse
    cd ../examples && python simple_agent.py   # envía trazas vía OTLP

Para verlo sin stack (salida por consola):

    AGENTLENS_CONSOLE=1 python simple_agent.py
"""
import agentlens


def lookup_customer(email: str) -> dict:
    # El email es PII: el SDK lo redacta antes de exportar.
    with agentlens.tool("lookup_customer") as span:
        span.set_attribute("gen_ai.tool.name", "lookup_customer")
        span.set_attribute("gen_ai.tool.call.arguments", f'{{"email": "{email}"}}')
        return {"id": "C-001", "tier": "premium"}


def generate_reply(customer: dict) -> str:
    # Simula una respuesta de LLM larga -> se externaliza como payload.
    with agentlens.tool("generate_reply") as span:
        span.set_attribute("gen_ai.request.model", "claude-sonnet-4-6")
        reply = "Estimado cliente, " + ("gracias por su consulta. " * 300)
        span.set_attribute("gen_ai.output.messages", reply)
        span.set_attribute("gen_ai.usage.output_tokens", 1800)
        return reply


def run():
    agentlens.instrument(
        api_key="demo-key",
        tenant_id="acme-corp",
        agent_id="support-bot",
        service_name="support-agent",
        # endpoint por defecto: http://localhost:4317 (Collector del docker-compose)
    )

    with agentlens.agent("support-bot", agent_id="support-bot") as span:
        span.set_attribute("gen_ai.agent.name", "support-bot")
        customer = lookup_customer("alice@example.com")
        reply = generate_reply(customer)
        print("Respuesta generada (truncada):", reply[:60], "...")

    agentlens.shutdown()
    print("Trazas enviadas. Revisa el Collector / ClickHouse.")


if __name__ == "__main__":
    run()
