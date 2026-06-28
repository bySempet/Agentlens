// Package cost calcula el coste estimado de tokens por agente y por modelo
// (E3-T06) a partir de los agregados del almacén y una tabla de tarifas.
package cost

import (
	"sort"

	"github.com/bysempet/agentlens/api/internal/store"
)

// Rate son las tarifas de un modelo en USD por 1M de tokens.
type Rate struct {
	InputPerM  float64
	OutputPerM float64
}

// rates es una tabla de tarifas de referencia (USD / 1M tokens). Ajustable.
var rates = map[string]Rate{
	"gpt-4o":             {InputPerM: 2.5, OutputPerM: 10},
	"gpt-4o-mini":        {InputPerM: 0.15, OutputPerM: 0.6},
	"claude-opus-4-8":    {InputPerM: 15, OutputPerM: 75},
	"claude-sonnet-4-6":  {InputPerM: 3, OutputPerM: 15},
	"claude-haiku-4-5":   {InputPerM: 0.8, OutputPerM: 4},
}

// defaultRate se aplica a modelos desconocidos (estimación conservadora).
var defaultRate = Rate{InputPerM: 1, OutputPerM: 3}

func rateFor(model string) Rate {
	if r, ok := rates[model]; ok {
		return r
	}
	return defaultRate
}

// USD calcula el coste de un uso de tokens para un modelo.
func USD(model string, inputTokens, outputTokens uint64) float64 {
	r := rateFor(model)
	return float64(inputTokens)/1e6*r.InputPerM + float64(outputTokens)/1e6*r.OutputPerM
}

// AgentCost / ModelCost son los rollups del resumen.
type AgentCost struct {
	AgentID      string  `json:"agent_id"`
	InputTokens  uint64  `json:"input_tokens"`
	OutputTokens uint64  `json:"output_tokens"`
	CostUSD      float64 `json:"cost_usd"`
}

type ModelCost struct {
	Model        string  `json:"model"`
	InputTokens  uint64  `json:"input_tokens"`
	OutputTokens uint64  `json:"output_tokens"`
	CostUSD      float64 `json:"cost_usd"`
}

// Summary es la respuesta del endpoint de coste.
type Summary struct {
	ByAgent           []AgentCost `json:"by_agent"`
	ByModel           []ModelCost `json:"by_model"`
	TotalInputTokens  uint64      `json:"total_input_tokens"`
	TotalOutputTokens uint64      `json:"total_output_tokens"`
	TotalCostUSD      float64     `json:"total_cost_usd"`
}

// Summarize agrega las filas (agente, modelo) en rollups por agente y por
// modelo, calculando el coste con la tarifa de cada modelo. El coste se computa
// por fila (modelo concreto) antes de sumar, para no mezclar tarifas.
func Summarize(rows []store.CostRow) Summary {
	agents := map[string]*AgentCost{}
	models := map[string]*ModelCost{}
	var s Summary

	for _, r := range rows {
		c := USD(r.Model, r.InputTokens, r.OutputTokens)

		a := agents[r.AgentID]
		if a == nil {
			a = &AgentCost{AgentID: r.AgentID}
			agents[r.AgentID] = a
		}
		a.InputTokens += r.InputTokens
		a.OutputTokens += r.OutputTokens
		a.CostUSD += c

		m := models[r.Model]
		if m == nil {
			m = &ModelCost{Model: r.Model}
			models[r.Model] = m
		}
		m.InputTokens += r.InputTokens
		m.OutputTokens += r.OutputTokens
		m.CostUSD += c

		s.TotalInputTokens += r.InputTokens
		s.TotalOutputTokens += r.OutputTokens
		s.TotalCostUSD += c
	}

	for _, a := range agents {
		s.ByAgent = append(s.ByAgent, *a)
	}
	for _, m := range models {
		s.ByModel = append(s.ByModel, *m)
	}
	// Orden estable: mayor coste primero.
	sort.Slice(s.ByAgent, func(i, j int) bool { return s.ByAgent[i].CostUSD > s.ByAgent[j].CostUSD })
	sort.Slice(s.ByModel, func(i, j int) bool { return s.ByModel[i].CostUSD > s.ByModel[j].CostUSD })
	return s
}
