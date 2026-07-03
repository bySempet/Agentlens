package agents

import (
	"context"
	"sort"
	"sync"
	"time"
)

// MemoryStore es un inventario en memoria para tests y desarrollo sin Postgres.
type MemoryStore struct {
	mu     sync.RWMutex
	byOrg  map[string]map[string]Agent // tenant -> agentID -> Agent
	nowFn  func() time.Time
}

// NewMemoryStore crea un inventario vacío.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{byOrg: map[string]map[string]Agent{}, nowFn: time.Now}
}

func (m *MemoryStore) Create(_ context.Context, tenant string, in CreateInput) (Agent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.byOrg[tenant] == nil {
		m.byOrg[tenant] = map[string]Agent{}
	}
	env := in.Environment
	if env == "" {
		env = "production"
	}
	a := Agent{
		AgentID:     GenerateAgentID(in.Name),
		Name:        in.Name,
		Framework:   in.Framework,
		Environment: env,
		Status:      "active",
		CreatedAt:   m.nowFn(),
	}
	m.byOrg[tenant][a.AgentID] = a
	return a, nil
}

func (m *MemoryStore) List(_ context.Context, tenant string) ([]Agent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []Agent{}
	for _, a := range m.byOrg[tenant] {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func (m *MemoryStore) Get(_ context.Context, tenant, agentID string) (*Agent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if a, ok := m.byOrg[tenant][agentID]; ok {
		return &a, nil
	}
	return nil, nil
}

func (m *MemoryStore) Disable(_ context.Context, tenant, agentID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.byOrg[tenant][agentID]
	if !ok {
		return false, nil
	}
	a.Status = "disabled"
	m.byOrg[tenant][agentID] = a
	return true, nil
}
