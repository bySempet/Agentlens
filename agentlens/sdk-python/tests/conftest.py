"""Resetea el estado global de OpenTelemetry entre tests.

OTel solo permite fijar el TracerProvider global una vez por proceso. En
producción esto es correcto (``instrument()`` se llama una sola vez), pero en
los tests necesitamos un provider limpio por caso.
"""
import pytest
from opentelemetry import trace
from opentelemetry.util._once import Once


@pytest.fixture(autouse=True)
def reset_otel_global():
    trace._TRACER_PROVIDER_SET_ONCE = Once()
    trace._TRACER_PROVIDER = None
    yield
    trace._TRACER_PROVIDER_SET_ONCE = Once()
    trace._TRACER_PROVIDER = None
