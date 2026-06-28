package policy

import (
	"context"
	"fmt"
	"io"

	"github.com/open-policy-agent/opa/compile"
)

// DefaultEntrypoint es el path (data.*) que se expone como entrypoint del WASM.
const DefaultEntrypoint = "agentlens/authz"

// BuildWASMBundle compila las políticas Rego de `src` a un bundle OPA con
// WebAssembly (E4-T02) y lo escribe en `w` como tar.gz. El bundle resultante
// (incluye policy.wasm) sirve para enforcement en el borde (E4-T03).
func BuildWASMBundle(ctx context.Context, w io.Writer, src, entrypoint string) error {
	if entrypoint == "" {
		entrypoint = DefaultEntrypoint
	}
	c := compile.New().
		WithTarget("wasm").
		WithEntrypoints(entrypoint).
		WithPaths(src).
		WithOutput(w)
	if err := c.Build(ctx); err != nil {
		return fmt.Errorf("compilando bundle WASM: %w", err)
	}
	return nil
}
