"""Agente de ejemplo con enforcement de políticas en el borde (E4-T03).

El SDK evalúa en local el policy.wasm del Policy Engine con wasmtime: las
herramientas prohibidas se bloquean antes de ejecutarse, sin llamar al backend,
y la decisión queda en la traza.

    cd ../policy-engine && go run ./cmd/buildbundle -out build/bundle.tar.gz
    cd ../examples && AGENTLENS_POLICY_BUNDLE=../policy-engine/build/bundle.tar.gz \\
        python policy_agent.py

Sin stack: añadir AGENTLENS_CONSOLE=1.
"""
import agentlens


def run_tool(name: str, **action) -> None:
    try:
        with agentlens.tool(name, **action):
            print(f"  ejecutada: {name}")
    except agentlens.PolicyViolation as exc:
        d = exc.decision
        print(f"  BLOQUEADA: {name} ({'; '.join(d.denials)}) en {d.eval_us:.0f} µs")


def run():
    agentlens.instrument(
        api_key="demo-key",
        tenant_id="acme-corp",
        agent_id="ops-bot",
        service_name="ops-agent",
    )

    with agentlens.agent("ops-bot", agent_id="ops-bot"):
        run_tool("search_docs")
        run_tool("shell.exec")
        run_tool("send_report", destination="external", pii_detected=True)
        run_tool("send_report", destination="internal", pii_detected=True)

    agentlens.shutdown()


if __name__ == "__main__":
    run()
