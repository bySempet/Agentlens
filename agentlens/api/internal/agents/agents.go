// Package agents implementa el inventario/registro de agentes (E3-T07): alta,
// baja y consulta de agentes por tenant, sobre el plano de control PostgreSQL.
package agents

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"regexp"
	"strings"
	"time"
)

// Agent es un agente registrado de una organización.
type Agent struct {
	AgentID     string    `json:"agent_id"`
	Name        string    `json:"name"`
	Framework   string    `json:"framework"`
	Environment string    `json:"environment"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

// CreateInput son los datos para registrar un agente.
type CreateInput struct {
	Name        string `json:"name"`
	Framework   string `json:"framework"`
	Environment string `json:"environment"`
}

// Store es el contrato del inventario de agentes, por tenant (slug de la org).
type Store interface {
	Create(ctx context.Context, tenant string, in CreateInput) (Agent, error)
	List(ctx context.Context, tenant string) ([]Agent, error)
	Get(ctx context.Context, tenant, agentID string) (*Agent, error)
	// Disable da de baja (status=disabled) un agente; devuelve false si no existe.
	Disable(ctx context.Context, tenant, agentID string) (bool, error)
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = nonSlug.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "agent"
	}
	return s
}

// GenerateAgentID deriva un agent_id estable y único a partir del nombre.
func GenerateAgentID(name string) string {
	b := make([]byte, 3)
	_, _ = rand.Read(b)
	return slugify(name) + "-" + hex.EncodeToString(b)
}
