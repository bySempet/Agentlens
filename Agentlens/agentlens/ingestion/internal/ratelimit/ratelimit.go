// Package ratelimit implementa el límite de ingesta por plan del tenant
// (E2-T06). Cada tenant tiene un token bucket cuyo caudal y capacidad los fija
// su plan; el coste de una petición es su número de spans (no de RPCs, para que
// el tamaño de batch del cliente no cambie lo que puede ingerir).
//
// El estado vive en memoria (una instancia de gateway). Para varias réplicas,
// la interfaz que consume el servidor (gateway.RateLimiter) permite sustituirlo
// por un limitador respaldado por Redis (E2-T12) sin tocar el servidor.
package ratelimit

import (
	"sync"
	"time"
)

// Limits define el caudal de un plan.
type Limits struct {
	// SpansPerSecond es el caudal sostenido. <= 0 significa sin límite.
	SpansPerSecond float64
	// Burst es la capacidad del bucket (pico instantáneo admitido).
	Burst float64
}

// Unlimited informa de si el plan no aplica límite.
func (l Limits) Unlimited() bool { return l.SpansPerSecond <= 0 }

// DefaultPlan es el plan que se asume cuando una API key no declara ninguno o
// declara uno desconocido.
const DefaultPlan = "starter"

// DefaultPlans son los límites de los tiers de AgentLens. Se pueden sobreescribir
// o ampliar por configuración (AGENTLENS_GATEWAY_PLAN_LIMITS).
func DefaultPlans() map[string]Limits {
	return map[string]Limits{
		"starter":    {SpansPerSecond: 100, Burst: 200},
		"pro":        {SpansPerSecond: 1_000, Burst: 2_000},
		"enterprise": {SpansPerSecond: 10_000, Burst: 20_000},
	}
}

// bucket es el estado del token bucket de un tenant.
type bucket struct {
	tokens float64
	last   time.Time
}

// TenantLimiter aplica Limits por tenant con token buckets en memoria.
// Es seguro para uso concurrente.
type TenantLimiter struct {
	mu      sync.Mutex
	plans   map[string]Limits
	buckets map[string]*bucket
	now     func() time.Time // inyectable en tests
}

// New crea el limitador con la tabla de planes dada (nil = DefaultPlans).
func New(plans map[string]Limits) *TenantLimiter {
	if plans == nil {
		plans = DefaultPlans()
	}
	cp := make(map[string]Limits, len(plans))
	for k, v := range plans {
		cp[k] = v
	}
	return &TenantLimiter{plans: cp, buckets: map[string]*bucket{}, now: time.Now}
}

// limitsFor resuelve los límites del plan, cayendo al plan por defecto si el
// plan es desconocido (una key mal dada de alta no debe abrir caudal infinito).
func (l *TenantLimiter) limitsFor(plan string) Limits {
	if lim, ok := l.plans[plan]; ok {
		return lim
	}
	return l.plans[DefaultPlan]
}

// Allow decide si el tenant puede ingerir `spans` spans ahora. Los tokens se
// reponen de forma continua a SpansPerSecond hasta Burst. Un batch mayor que
// Burst cuesta Burst (si no, jamás podría pasar): admite el batch puntual pero
// vacía el bucket, así el caudal sostenido sigue acotado.
func (l *TenantLimiter) Allow(tenantID, plan string, spans int) bool {
	if spans <= 0 {
		return true
	}
	limits := l.limitsFor(plan)
	if limits.Unlimited() {
		return true
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	b, ok := l.buckets[tenantID]
	if !ok {
		b = &bucket{tokens: limits.Burst, last: now}
		l.buckets[tenantID] = b
	}

	elapsed := now.Sub(b.last).Seconds()
	if elapsed > 0 {
		b.tokens += elapsed * limits.SpansPerSecond
		if b.tokens > limits.Burst {
			b.tokens = limits.Burst
		}
		b.last = now
	}

	cost := float64(spans)
	if cost > limits.Burst {
		cost = limits.Burst
	}
	if b.tokens < cost {
		return false
	}
	b.tokens -= cost
	return true
}
