from agentlens.redaction import Redactor


def test_redacts_email():
    r = Redactor()
    out = r.redact_text("contacta a alice@example.com por favor")
    assert "alice@example.com" not in out
    assert "[REDACTED_EMAIL]" in out


def test_redacts_iban():
    r = Redactor()
    out = r.redact_text("IBAN ES9121000418450200051332 saldo ok")
    assert "ES9121000418450200051332" not in out
    assert "[REDACTED_IBAN]" in out


def test_redacts_card():
    r = Redactor()
    out = r.redact_text("tarjeta 4111 1111 1111 1111 caducada")
    assert "4111 1111 1111 1111" not in out
    assert "[REDACTED_CARD]" in out


def test_non_string_values_pass_through():
    r = Redactor()
    assert r.redact_value(42) == 42
    assert r.redact_value(True) is True


def test_redact_attributes_does_not_mutate_original():
    r = Redactor()
    original = {"user.email": "bob@acme.io", "model": "gpt-4o"}
    out = r.redact_attributes(original)
    assert original["user.email"] == "bob@acme.io"  # intacto
    assert out["user.email"] == "[REDACTED_EMAIL]"
    assert out["model"] == "gpt-4o"  # no es PII, no se toca


def test_redacts_phone_formats():
    r = Redactor()
    assert "[REDACTED_PHONE]" in r.redact_text("llama al +34 600 123 456")
    assert "[REDACTED_PHONE]" in r.redact_text("tel 123-456-7890")


def test_phone_does_not_overredact_plain_numbers():
    r = Redactor()
    # Dos grupos sueltos (p.ej. un nº de pedido) no deben tomarse por teléfono.
    out = r.redact_text("pedido 1234 5678 confirmado")
    assert out == "pedido 1234 5678 confirmado"
    assert "[REDACTED_PHONE]" not in r.redact_text("cantidad 4096")


def test_custom_pattern():
    r = Redactor(enabled=["email"])
    r.add_pattern("emp_id", r"EMP-\d{5}", "[REDACTED_EMP]")
    out = r.redact_text("empleado EMP-12345 con email x@y.com")
    assert "[REDACTED_EMP]" in out
    assert "[REDACTED_EMAIL]" in out
