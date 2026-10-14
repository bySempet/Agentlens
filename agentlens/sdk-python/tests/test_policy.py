"""Tests del enforcement en el borde (E4-T03).

Ejecutan de verdad el ``policy.wasm`` que compila el Policy Engine (Go) con
wasmtime. El bundle se construye con ``go run ./cmd/buildbundle`` (o se toma de
``AGENTLENS_TEST_POLICY_BUNDLE``). Sin Go los tests se saltan, salvo con
``AGENTLENS_REQUIRE_POLICY_TESTS=1`` (CI), donde saltarlos es un fallo.
"""
import json
import os
import shutil
import subprocess
import sys
import threading
from pathlib import Path

import pytest

pytest.importorskip("wasmtime")

import agentlens  # noqa: E402
from agentlens import policy as policy_mod  # noqa: E402
from agentlens.policy import PolicyError, PolicyViolation, WasmPolicy  # noqa: E402
from opentelemetry.sdk.trace.export.in_memory_span_exporter import (  # noqa: E402
    InMemorySpanExporter,
)
from opentelemetry.trace import StatusCode  # noqa: E402

POLICY_ENGINE = Path(__file__).resolve().parents[2] / "policy-engine"


def _unavailable(reason):
    if os.getenv("AGENTLENS_REQUIRE_POLICY_TESTS") == "1":
        pytest.fail(reason)
    pytest.skip(reason)


@pytest.fixture(scope="session")
def go_tools(tmp_path_factory):
    """Construye el bundle WASM y el CLI de referencia ``policyeval``."""
    out = tmp_path_factory.mktemp("policy")
    given = os.getenv("AGENTLENS_TEST_POLICY_BUNDLE")
    if given:
        return {"bundle": given, "policyeval": None}
    go = shutil.which("go")
    if go is None:
        _unavailable("Go no disponible para compilar el bundle de políticas")
    bundle = out / "bundle.tar.gz"
    evaluator = out / "policyeval"
    subprocess.run([go, "run", "./cmd/buildbundle", "-out", str(bundle)],
                   cwd=POLICY_ENGINE, check=True, capture_output=True)
    subprocess.run([go, "build", "-o", str(evaluator), "./cmd/policyeval"],
                   cwd=POLICY_ENGINE, check=True, capture_output=True)
    return {"bundle": str(bundle), "policyeval": str(evaluator)}


@pytest.fixture(scope="session")
def wasm_policy(go_tools):
    return WasmPolicy.from_bundle(go_tools["bundle"])


@pytest.fixture(autouse=True)
def no_global_policy():
    yield
    # instrument() deja la política como estado global del SDK.
    sys.modules["agentlens.instrument"]._POLICY = None


CASES = [
    ({"tool": "search"}, True, []),
    ({"tool": "shell.exec"}, False, ["herramienta bloqueada: shell.exec"]),
    ({"tool": "fs.delete"}, False, ["herramienta bloqueada: fs.delete"]),
    ({"pii_detected": True, "destination": "external"}, False,
     ["salida con PII hacia destino externo bloqueada"]),
    ({"pii_detected": True, "destination": "internal"}, True, []),
    ({"output_tokens": 100000}, True, []),
    ({"output_tokens": 150000}, False,
     ["supera el límite de tokens de salida (150000 > 100000)"]),
    ({"tool": "shell.exec", "pii_detected": True, "destination": "external",
      "output_tokens": 200000}, False, [
        "herramienta bloqueada: shell.exec",
        "salida con PII hacia destino externo bloqueada",
        "supera el límite de tokens de salida (200000 > 100000)",
    ]),
]


@pytest.mark.parametrize("action,allow,denials", CASES)
def test_wasm_decisions(wasm_policy, action, allow, denials):
    d = wasm_policy.evaluate(action)
    assert d.allow is allow
    assert d.denials == sorted(denials)


@pytest.mark.parametrize("action,allow,denials", CASES)
def test_wasm_matches_go_engine(go_tools, wasm_policy, action, allow, denials):
    """Diferencial: el WASM en wasmtime decide igual que el motor OPA en Go."""
    if go_tools["policyeval"] is None:
        pytest.skip("sin policyeval (bundle aportado externamente)")
    proc = subprocess.run([go_tools["policyeval"]], input=json.dumps(action),
                          capture_output=True, text=True)
    assert proc.returncode in (0, 3), proc.stderr
    ref = json.loads(proc.stdout)
    d = wasm_policy.evaluate(action)
    assert d.allow == ref["allow"]
    assert d.denials == sorted(ref["denials"])


def test_data_override_changes_decision(go_tools):
    p = WasmPolicy.from_bundle(
        go_tools["bundle"],
        data_override={"config": {"blocked_tools": ["web.fetch"],
                                  "limits": {"max_output_tokens": 10}}},
    )
    assert p.evaluate({"tool": "shell.exec"}).allow  # ya no está bloqueada
    assert not p.evaluate({"tool": "web.fetch"}).allow
    assert not p.evaluate({"output_tokens": 11}).allow


def test_latency_sub_millisecond(wasm_policy):
    """Criterio E4-T03: decisión local en sub-milisegundos (peor caso: 3 denegaciones)."""
    worst = CASES[-1][0]
    for _ in range(200):
        wasm_policy.evaluate(worst)
    samples = sorted(wasm_policy.evaluate(worst).eval_us for _ in range(2000))
    p50 = samples[len(samples) // 2]
    assert p50 < 1000, f"p50={p50:.0f}µs"


def test_fail_closed_on_runtime_error(wasm_policy, monkeypatch):
    def boom(raw):
        raise RuntimeError("trap simulado")

    monkeypatch.setattr(wasm_policy, "_eval_locked", boom)
    d = wasm_policy.evaluate({"tool": "search"})
    assert not d.allow
    assert "trap simulado" in d.denials[0]
    monkeypatch.undo()
    # Tras el error la instancia se recrea y sigue evaluando bien.
    assert wasm_policy.evaluate({"tool": "search"}).allow


def test_fail_closed_on_unserializable_input(wasm_policy):
    d = wasm_policy.evaluate({"tool": object()})
    assert not d.allow


def test_unsupported_builtin_fails_on_load(go_tools, monkeypatch):
    monkeypatch.setattr(policy_mod, "_BUILTINS", {})
    with pytest.raises(PolicyError, match="sprintf"):
        WasmPolicy.from_bundle(go_tools["bundle"])


def test_bundle_without_wasm_is_rejected(tmp_path):
    import tarfile

    bad = tmp_path / "bad.tar.gz"
    with tarfile.open(bad, "w:gz") as tar:
        data = tmp_path / "data.json"
        data.write_text("{}")
        tar.add(data, arcname="/data.json")
    with pytest.raises(PolicyError, match="policy.wasm"):
        WasmPolicy.from_bundle(str(bad))


def test_concurrent_evaluations(wasm_policy):
    errors = []

    def worker(i):
        for j in range(200):
            blocked = (i + j) % 2 == 0
            d = wasm_policy.evaluate({"tool": "shell.exec" if blocked else "search"})
            if d.allow == blocked:
                errors.append((i, j, d))

    threads = [threading.Thread(target=worker, args=(i,)) for i in range(8)]
    for t in threads:
        t.start()
    for t in threads:
        t.join()
    assert errors == []


# --- integración con la API del SDK ----------------------------------------------


def _instrument(go_tools, mem):
    return agentlens.instrument(
        tenant_id="acme", agent_id="support-bot", exporter=mem,
        auto_instrument=False, policy_bundle=go_tools["bundle"],
    )


def test_tool_is_blocked_before_running(go_tools):
    mem = InMemorySpanExporter()
    provider = _instrument(go_tools, mem)
    ran = []

    with pytest.raises(PolicyViolation) as exc:
        with agentlens.tool("shell.exec"):
            ran.append(True)

    assert ran == []  # el cuerpo de la herramienta no llegó a ejecutarse
    assert exc.value.decision.denials == ["herramienta bloqueada: shell.exec"]

    provider.force_flush()
    (span,) = mem.get_finished_spans()
    assert span.status.status_code == StatusCode.ERROR
    assert span.attributes["agentlens.policy.allow"] is False
    assert list(span.attributes["agentlens.policy.denials"]) == [
        "herramienta bloqueada: shell.exec"]


def test_allowed_tool_runs_and_records_decision(go_tools):
    mem = InMemorySpanExporter()
    provider = _instrument(go_tools, mem)

    with agentlens.tool("search") as span:
        span.set_attribute("resultado", "ok")

    provider.force_flush()
    (span,) = mem.get_finished_spans()
    assert span.attributes["agentlens.policy.allow"] is True
    assert span.attributes["agentlens.policy.eval_us"] > 0
    assert "agentlens.policy.denials" not in span.attributes


def test_tool_action_fields_reach_policy(go_tools):
    _instrument(go_tools, InMemorySpanExporter())
    with pytest.raises(PolicyViolation, match="PII"):
        with agentlens.tool("send_email", destination="external", pii_detected=True):
            pass


def test_check_and_enforce(go_tools):
    _instrument(go_tools, InMemorySpanExporter())
    assert agentlens.check(output_tokens=10).allow
    assert not agentlens.check(output_tokens=10**6).allow
    with pytest.raises(PolicyViolation):
        agentlens.enforce(output_tokens=10**6)


def test_policy_bundle_from_env(go_tools, monkeypatch):
    monkeypatch.setenv("AGENTLENS_POLICY_BUNDLE", go_tools["bundle"])
    agentlens.instrument(exporter=InMemorySpanExporter(), auto_instrument=False)
    assert not agentlens.check(tool="shell.exec").allow


def test_without_policy_everything_is_allowed():
    agentlens.instrument(exporter=InMemorySpanExporter(), auto_instrument=False)
    assert agentlens.check(tool="shell.exec").allow
    with agentlens.tool("shell.exec"):
        pass


def test_missing_bundle_fails_instrument(tmp_path):
    with pytest.raises(PolicyError):
        agentlens.instrument(exporter=InMemorySpanExporter(), auto_instrument=False,
                             policy_bundle=str(tmp_path / "no-existe.tar.gz"))
