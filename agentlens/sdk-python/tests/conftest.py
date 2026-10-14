"""Resetea el estado global de OpenTelemetry entre tests.

OTel solo permite fijar el TracerProvider global una vez por proceso. En
producción esto es correcto (``instrument()`` se llama una sola vez), pero en
los tests necesitamos un provider limpio por caso.
"""
import os
import shutil
import subprocess
import sys
from pathlib import Path

import pytest
from opentelemetry import trace
from opentelemetry.util._once import Once

POLICY_ENGINE = Path(__file__).resolve().parents[2] / "policy-engine"


@pytest.fixture(autouse=True)
def reset_otel_global():
    trace._TRACER_PROVIDER_SET_ONCE = Once()
    trace._TRACER_PROVIDER = None
    yield
    trace._TRACER_PROVIDER_SET_ONCE = Once()
    trace._TRACER_PROVIDER = None


@pytest.fixture(autouse=True)
def reset_policy():
    """instrument() deja la política local (E4-T03) como estado global del SDK."""
    yield
    sys.modules["agentlens.instrument"]._POLICY = None


def _unavailable(reason):
    if os.getenv("AGENTLENS_REQUIRE_POLICY_TESTS") == "1":
        pytest.fail(reason)
    pytest.skip(reason)


@pytest.fixture(scope="session")
def go_tools(tmp_path_factory):
    """Bundle WASM de políticas y CLI de referencia ``policyeval`` (Go).

    Se compilan con el Policy Engine o se toma el bundle de
    ``AGENTLENS_TEST_POLICY_BUNDLE``. Sin Go los tests que lo usan se saltan,
    salvo con ``AGENTLENS_REQUIRE_POLICY_TESTS=1`` (CI), donde es un fallo.
    """
    pytest.importorskip("wasmtime")
    given = os.getenv("AGENTLENS_TEST_POLICY_BUNDLE")
    if given:
        return {"bundle": given, "policyeval": None}
    go = shutil.which("go")
    if go is None:
        _unavailable("Go no disponible para compilar el bundle de políticas")
    out = tmp_path_factory.mktemp("policy")
    bundle = out / "bundle.tar.gz"
    evaluator = out / "policyeval"
    subprocess.run([go, "run", "./cmd/buildbundle", "-out", str(bundle)],
                   cwd=POLICY_ENGINE, check=True, capture_output=True)
    subprocess.run([go, "build", "-o", str(evaluator), "./cmd/policyeval"],
                   cwd=POLICY_ENGINE, check=True, capture_output=True)
    return {"bundle": str(bundle), "policyeval": str(evaluator)}
