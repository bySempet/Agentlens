# Policy Engine (Go + OPA/Rego) — E4-T01 / E4-T02

Motor de decisiones de gobernanza para acciones de agentes. Evalúa políticas
**Rego** (OpenAGent Policy Agent, OPA) compiladas una sola vez y evaluadas por
acción en **sub-milisegundos**, apto para el hot path / enforcement en el borde
(E4-T03).

## Qué decide

Dada una acción de agente (`policy.Input`: tenant, agente, operación, herramienta,
modelo, destino, PII, tokens…), devuelve una `Decision { allow, denials[] }`. La
política base (`policy/policies/authz.rego`) acumula motivos de denegación y solo
permite si no hay ninguno.

Reglas base incluidas:

- **Herramientas bloqueadas** (`data.config.blocked_tools`, p.ej. `shell.exec`).
- **Minimización de datos**: PII no puede salir a un destino `external` (GDPR).
- **Límite de tokens de salida** por acción (`data.config.limits.max_output_tokens`).

Los umbrales son **data-driven**: el control-plane puede ajustarlos sin recompilar
la política (se inyectan como documento `data.config.*`).

## Uso (librería)

```go
eng, _ := policy.New(ctx, policy.DefaultConfig())
d, _ := eng.Evaluate(ctx, policy.Input{Tenant: "acme", Tool: "shell.exec"})
// d.Allow == false; d.Denials == ["herramienta bloqueada: shell.exec"]
```

## CLI

```bash
echo '{"tool":"shell.exec"}' | go run ./cmd/policyeval
# {"allow": false, "denials": ["herramienta bloqueada: shell.exec"]}  (exit 3)
```

## Bundle WASM (E4-T02)

La política se compila a un bundle OPA con WebAssembly (`policy.wasm`), listo para
enforcement en el borde (E4-T03). Se genera en CI:

```bash
go run ./cmd/buildbundle -out build/bundle.tar.gz
# build/bundle.tar.gz contiene policy.wasm (+ data.json, .manifest)
```

## Tests

```bash
go test ./...
go test -bench=. ./policy
```

Cubren permitir/denegar (herramienta, PII→externo, límite de tokens, denegaciones
acumuladas) y el **criterio de latencia < 5 ms** (medido ~60 µs por evaluación
tras compilar).

## Siguiente

- **E4-T03**: enforcement síncrono en el borde (SDK/gateway evalúa el WASM local).
- **E4-T04**: sincronización de políticas control-plane → borde.
