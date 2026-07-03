// Package policy evalúa decisiones de gobernanza sobre acciones de agentes
// usando OPA/Rego (E4-T01). Compila la política una vez (PrepareForEval) y la
// evalúa por acción en sub-milisegundos, apto para el hot path.
package policy

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"

	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/storage/inmem"
)

//go:embed policies/*.rego
var policyFS embed.FS

// Input es la acción de un agente a evaluar.
type Input struct {
	Tenant       string `json:"tenant"`
	Agent        string `json:"agent"`
	Operation    string `json:"operation"`
	Tool         string `json:"tool"`
	Model        string `json:"model"`
	Destination  string `json:"destination"` // "internal" | "external"
	PIIDetected  bool   `json:"pii_detected"`
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
}

// Decision es el resultado de evaluar la política.
type Decision struct {
	Allow    bool     `json:"allow"`
	Denials  []string `json:"denials"`
}

// Config son los parámetros data-driven de la política (ajustables por el
// control-plane sin tocar el Rego).
type Config struct {
	BlockedTools    []string
	MaxOutputTokens int
}

// DefaultConfig devuelve una configuración base razonable.
func DefaultConfig() Config {
	return Config{
		BlockedTools:    []string{"shell.exec", "fs.delete"},
		MaxOutputTokens: 100_000,
	}
}

// Engine evalúa la política base ya compilada.
type Engine struct {
	query rego.PreparedEvalQuery
}

// New compila la política base con la configuración dada.
func New(ctx context.Context, cfg Config) (*Engine, error) {
	module, err := policyFS.ReadFile("policies/authz.rego")
	if err != nil {
		return nil, fmt.Errorf("leyendo política: %w", err)
	}

	data := map[string]any{
		"config": map[string]any{
			"blocked_tools": cfg.BlockedTools,
			"limits":        map[string]any{"max_output_tokens": cfg.MaxOutputTokens},
		},
	}

	query, err := rego.New(
		rego.Query("data.agentlens.authz"),
		rego.Module("authz.rego", string(module)),
		rego.Store(inmem.NewFromObject(data)),
	).PrepareForEval(ctx)
	if err != nil {
		return nil, fmt.Errorf("compilando política: %w", err)
	}
	return &Engine{query: query}, nil
}

// Evaluate decide sobre una acción de agente.
func (e *Engine) Evaluate(ctx context.Context, in Input) (Decision, error) {
	// Convertimos a map para que OPA reciba datos JSON-limpios.
	var input map[string]any
	raw, _ := json.Marshal(in)
	_ = json.Unmarshal(raw, &input)

	rs, err := e.query.Eval(ctx, rego.EvalInput(input))
	if err != nil {
		return Decision{}, err
	}
	if len(rs) == 0 || len(rs[0].Expressions) == 0 {
		return Decision{}, fmt.Errorf("la política no produjo resultado")
	}
	doc, ok := rs[0].Expressions[0].Value.(map[string]any)
	if !ok {
		return Decision{}, fmt.Errorf("formato de decisión inesperado")
	}

	d := Decision{Denials: []string{}}
	if allow, ok := doc["allow"].(bool); ok {
		d.Allow = allow
	}
	if denials, ok := doc["deny"].([]any); ok {
		for _, m := range denials {
			if s, ok := m.(string); ok {
				d.Denials = append(d.Denials, s)
			}
		}
	}
	return d, nil
}
