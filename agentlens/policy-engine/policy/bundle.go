package policy

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/compile"
)

// DefaultEntrypoint es el path (data.*) que se expone como entrypoint del WASM.
const DefaultEntrypoint = "agentlens/authz"

// BuildWASMBundle compila las políticas Rego de `src` a un bundle OPA con
// WebAssembly (E4-T02) y lo escribe en `w` como tar.gz. El bundle incluye
// policy.wasm y, como data.json, la configuración `cfg` (data.config.*), de modo
// que es autocontenido: el borde (E4-T03) lo evalúa sin el motor Go ni el backend.
func BuildWASMBundle(ctx context.Context, w io.Writer, src, entrypoint string, cfg Config) error {
	if entrypoint == "" {
		entrypoint = DefaultEntrypoint
	}
	var compiled bytes.Buffer
	c := compile.New().
		WithTarget("wasm").
		WithEntrypoints(entrypoint).
		WithPaths(src).
		WithOutput(&compiled)
	if err := c.Build(ctx); err != nil {
		return fmt.Errorf("compilando bundle WASM: %w", err)
	}

	// La compilación solo conoce los .rego; añadimos la configuración como data.
	b, err := bundle.NewReader(&compiled).Read()
	if err != nil {
		return fmt.Errorf("leyendo bundle compilado: %w", err)
	}
	b.Data = cfg.Data()
	if err := bundle.NewWriter(w).Write(b); err != nil {
		return fmt.Errorf("escribiendo bundle: %w", err)
	}
	return nil
}
