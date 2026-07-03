// Package plan define los tiers comerciales de AgentLens y resuelve el plan de
// cada tenant. Los límites por plan los consume el rate limiter del gateway
// (E2-T06). Mantener los tiers aquí los desacopla del mecanismo de limitación.
package plan

// Plan describe los límites de ingesta de un tier.
type Plan struct {
	Name string
	// RatePerSec es el caudal sostenido de peticiones OTLP/seg permitido.
	RatePerSec float64
	// Burst es el máximo de peticiones que se pueden acumular en una ráfaga.
	Burst int
}

// Tiers de referencia (alineados con el modelo de negocio del backlog; los
// valores numéricos son un punto de partida ajustable por configuración).
var (
	Free       = Plan{Name: "free", RatePerSec: 5, Burst: 10}
	Starter    = Plan{Name: "starter", RatePerSec: 50, Burst: 100}
	Growth     = Plan{Name: "growth", RatePerSec: 200, Burst: 400}
	Enterprise = Plan{Name: "enterprise", RatePerSec: 1000, Burst: 2000}
)

// byName permite resolver un plan a partir de su nombre (config por entorno).
var byName = map[string]Plan{
	Free.Name:       Free,
	Starter.Name:    Starter,
	Growth.Name:     Growth,
	Enterprise.Name: Enterprise,
}

// ByName devuelve el plan con ese nombre y true si existe.
func ByName(name string) (Plan, bool) {
	p, ok := byName[name]
	return p, ok
}

// Registry resuelve el plan de un tenant.
type Registry interface {
	PlanFor(tenantID string) Plan
}

// StaticRegistry mapea tenant -> plan en memoria, con un plan por defecto para
// los tenants no listados. Sustituible por uno respaldado por Postgres (E2-T08).
type StaticRegistry struct {
	plans   map[string]Plan
	fallback Plan
}

// NewStaticRegistry crea el registry. Los tenants ausentes reciben fallback.
func NewStaticRegistry(plans map[string]Plan, fallback Plan) *StaticRegistry {
	cp := make(map[string]Plan, len(plans))
	for k, v := range plans {
		cp[k] = v
	}
	return &StaticRegistry{plans: cp, fallback: fallback}
}

// PlanFor implementa Registry.
func (r *StaticRegistry) PlanFor(tenantID string) Plan {
	if p, ok := r.plans[tenantID]; ok {
		return p
	}
	return r.fallback
}
