// Package ratelimit aplica límites de caudal por tenant según su plan (E2-T06).
// Implementa un token-bucket por tenant, sin dependencias externas y con reloj
// inyectable para tests deterministas. Es seguro para uso concurrente.
package ratelimit

import (
	"context"
	"sync"
	"time"

	"github.com/bysempet/agentlens/ingestion/internal/auth"
	"github.com/bysempet/agentlens/ingestion/internal/plan"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// bucket es un token-bucket protegido por su propio mutex.
type bucket struct {
	mu         sync.Mutex
	tokens     float64
	max        float64
	refillRate float64 // tokens por segundo
	last       time.Time
}

func newBucket(p plan.Plan, now time.Time) *bucket {
	return &bucket{
		tokens:     float64(p.Burst),
		max:        float64(p.Burst),
		refillRate: p.RatePerSec,
		last:       now,
	}
}

// allow recarga según el tiempo transcurrido y consume un token si hay.
func (b *bucket) allow(now time.Time) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	elapsed := now.Sub(b.last).Seconds()
	if elapsed > 0 {
		b.tokens = minFloat(b.max, b.tokens+elapsed*b.refillRate)
		b.last = now
	}
	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// Limiter mantiene un bucket por tenant, dimensionado según su plan.
type Limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	reg     plan.Registry
	now     func() time.Time
}

// New crea el limiter con el registry de planes dado.
func New(reg plan.Registry) *Limiter {
	return &Limiter{
		buckets: make(map[string]*bucket),
		reg:     reg,
		now:     time.Now,
	}
}

// Allow indica si el tenant puede cursar una petición ahora.
func (l *Limiter) Allow(tenantID string) bool {
	l.mu.Lock()
	b, ok := l.buckets[tenantID]
	if !ok {
		b = newBucket(l.reg.PlanFor(tenantID), l.now())
		l.buckets[tenantID] = b
	}
	l.mu.Unlock()
	return b.allow(l.now())
}

// UnaryInterceptor rechaza con ResourceExhausted las peticiones que excedan el
// plan del tenant. Debe encadenarse DESPUÉS del interceptor de auth, que es
// quien deja el tenant en el contexto.
func UnaryInterceptor(l *Limiter) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		tenantID, ok := auth.TenantFromContext(ctx)
		if ok && !l.Allow(tenantID) {
			return nil, status.Errorf(
				codes.ResourceExhausted,
				"límite de caudal superado para el tenant %s", tenantID,
			)
		}
		return handler(ctx, req)
	}
}
