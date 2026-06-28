// Command policyeval evalúa una acción de agente (JSON por stdin) contra la
// política base y emite la decisión (JSON). Útil para pruebas y CI.
//
//	echo '{"tool":"shell.exec"}' | policyeval
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/bysempet/agentlens/policy-engine/policy"
)

func main() {
	eng, err := policy.New(context.Background(), policy.DefaultConfig())
	if err != nil {
		fmt.Fprintln(os.Stderr, "error inicializando el motor:", err)
		os.Exit(1)
	}

	var in policy.Input
	if err := json.NewDecoder(os.Stdin).Decode(&in); err != nil {
		fmt.Fprintln(os.Stderr, "entrada JSON inválida:", err)
		os.Exit(2)
	}

	decision, err := eng.Evaluate(context.Background(), in)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error evaluando:", err)
		os.Exit(1)
	}

	out, _ := json.MarshalIndent(decision, "", "  ")
	fmt.Println(string(out))
	if !decision.Allow {
		os.Exit(3) // código distinto si se deniega (útil en scripts/CI)
	}
}
