"""Gate de latencia del SDK (E0-T03 / criterio de E1-T03).

Mide lo que paga el agente por cada span instrumentado en el hot path (abrir y
cerrar ``agent()`` + ``tool()`` con atributos, PII y un payload grande) con un
backend **lento** (cada export tarda 50 ms). El build falla si el p99 supera el
presupuesto (5 ms por defecto, ``AGENTLENS_LATENCY_BUDGET_MS``).

El meta-test comprueba que el gate detecta una regresión real: con export
síncrono (``SimpleSpanProcessor``) el agente esperaría al backend y el gate salta.
"""
import os
import time

import pytest
from opentelemetry import trace
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import (
    SimpleSpanProcessor,
    SpanExporter,
    SpanExportResult,
)

import agentlens

BUDGET_MS = float(os.getenv("AGENTLENS_LATENCY_BUDGET_MS", "5"))
ITERATIONS = 2000
WARMUP = 200
BIG_PAYLOAD = "respuesta del modelo " * 500  # ~10 KB -> se externaliza


class SlowExporter(SpanExporter):
    """Backend lento: cada export bloquea 50 ms (red lenta / Collector saturado)."""

    def __init__(self, delay_s: float = 0.05):
        self.delay_s = delay_s
        self.exported = 0

    def export(self, spans):
        time.sleep(self.delay_s)
        self.exported += len(spans)
        return SpanExportResult.SUCCESS

    def shutdown(self):
        pass


def _one_action(i: int) -> None:
    with agentlens.agent("support-bot"):
        with agentlens.tool("lookup_customer") as span:
            span.set_attribute("gen_ai.tool.call.arguments", f'{{"email": "user{i}@example.com"}}')
            span.set_attribute("gen_ai.output.messages", BIG_PAYLOAD)
            span.set_attribute("gen_ai.usage.output_tokens", 1800)


def measure_hot_path(iterations: int = ITERATIONS, action=_one_action) -> dict:
    for i in range(WARMUP):
        action(i)
    samples = []
    for i in range(iterations):
        t0 = time.perf_counter()
        action(i)
        samples.append((time.perf_counter() - t0) * 1000)
    samples.sort()
    return {
        "p50": samples[len(samples) // 2],
        "p99": samples[int(len(samples) * 0.99) - 1],
        "max": samples[-1],
    }


def check_budget(stats: dict, budget_ms: float = BUDGET_MS) -> None:
    assert stats["p99"] <= budget_ms, (
        f"overhead del SDK por encima del presupuesto: p99={stats['p99']:.3f} ms "
        f"> {budget_ms} ms (p50={stats['p50']:.3f} ms, max={stats['max']:.3f} ms)"
    )


def test_latency_gate_with_slow_backend():
    exporter = SlowExporter()
    provider = agentlens.instrument(
        tenant_id="acme", agent_id="support-bot", exporter=exporter,
        auto_instrument=False, redact_pii=True, payload_mode="reference",
    )
    stats = measure_hot_path()
    provider.force_flush()
    print(f"\n[E0-T03] hot path con backend lento: p50={stats['p50']:.3f} ms "
          f"p99={stats['p99']:.3f} ms max={stats['max']:.3f} ms (presupuesto {BUDGET_MS} ms)")
    assert exporter.exported > 0  # el pipeline real exportó en segundo plano
    check_budget(stats)


def test_latency_gate_with_policy_enforcement(go_tools):
    """Con enforcement local (E4-T03) el hot path también evalúa el policy.wasm."""
    agentlens.instrument(
        tenant_id="acme", agent_id="support-bot", exporter=SlowExporter(),
        auto_instrument=False, policy_bundle=go_tools["bundle"],
    )
    stats = measure_hot_path()
    print(f"\n[E0-T03] hot path + policy.wasm: p50={stats['p50']:.3f} ms "
          f"p99={stats['p99']:.3f} ms max={stats['max']:.3f} ms (presupuesto {BUDGET_MS} ms)")
    check_budget(stats)


def test_gate_detects_synchronous_export():
    """Si el export pasara al hot path (regresión), el gate debe romper el build."""
    provider = TracerProvider()
    provider.add_span_processor(SimpleSpanProcessor(SlowExporter(delay_s=0.01)))
    trace.set_tracer_provider(provider)

    stats = measure_hot_path(iterations=50)
    with pytest.raises(AssertionError, match="presupuesto"):
        check_budget(stats)
