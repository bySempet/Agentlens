import { test } from "node:test";
import assert from "node:assert/strict";
import { Redactor } from "../src/redaction.js";

test("redacta email, IBAN y tarjeta", () => {
  const r = new Redactor();
  assert.equal(r.redactText("contacto: alice@example.com"), "contacto: [REDACTED_EMAIL]");
  assert.match(r.redactText("IBAN ES9121000418450200051332"), /\[REDACTED_IBAN\]/);
  assert.match(r.redactText("tarjeta 4111 1111 1111 1111"), /\[REDACTED_CARD\]/);
});

test("redactAttributes no muta el original y respeta no-strings", () => {
  const r = new Redactor();
  const input = { "user.email": "bob@test.org", count: 3 };
  const out = r.redactAttributes(input);
  assert.equal(out["user.email"], "[REDACTED_EMAIL]");
  assert.equal(out["count"], 3);
  assert.equal(input["user.email"], "bob@test.org"); // original intacto
});

test("redacta teléfonos pero no números sueltos", () => {
  const r = new Redactor();
  assert.match(r.redactText("llama al +34 600 123 456"), /\[REDACTED_PHONE\]/);
  assert.match(r.redactText("tel 123-456-7890"), /\[REDACTED_PHONE\]/);
  // Un nº de pedido suelto no debe redactarse como teléfono.
  assert.equal(r.redactText("pedido 1234 5678"), "pedido 1234 5678");
});

test("permite patrones propios", () => {
  const r = new Redactor(["email"]);
  r.addPattern("emp", /EMP-\d{4}/g, "[REDACTED_EMP]");
  assert.equal(r.redactText("empleado EMP-1234"), "empleado [REDACTED_EMP]");
});
