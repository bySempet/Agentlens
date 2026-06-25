package ratelimit

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bysempet/agentlens/ingestion/internal/plan"
)

// fixedClock permite controlar el tiempo en los tests.
type fixedClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fixedClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fixedClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// limiterWithClock crea un Limiter con un único plan para todos los tenants y un
// reloj controlado.
func limiterWithClock(p plan.Plan, clk *fixedClock) *Limiter {
	reg := plan.NewStaticRegistry(nil, p)
	l := New(reg)
	l.now = clk.now
	return l
}

func TestAllow_BurstThenDeny(t *testing.T) {
	clk := &fixedClock{t: time.Unix(0, 0)}
	// Sin recarga (RatePerSec=0): solo se permite la ráfaga inicial.
	l := limiterWithClock(plan.Plan{Name: "test", RatePerSec: 0, Burst: 3}, clk)

	for i := 0; i < 3; i++ {
		if !l.Allow("acme") {
			t.Fatalf("petición %d dentro de la ráfaga debería permitirse", i+1)
		}
	}
	if l.Allow("acme") {
		t.Fatal("la 4ª petición debería rechazarse (ráfaga agotada, sin recarga)")
	}
}

func TestAllow_RefillsOverTime(t *testing.T) {
	clk := &fixedClock{t: time.Unix(0, 0)}
	l := limiterWithClock(plan.Plan{Name: "test", RatePerSec: 10, Burst: 1}, clk)

	if !l.Allow("acme") {
		t.Fatal("primera petición debería permitirse")
	}
	if l.Allow("acme") {
		t.Fatal("segunda petición inmediata debería rechazarse")
	}
	// A 10 tokens/seg, 100 ms repone exactamente 1 token.
	clk.advance(100 * time.Millisecond)
	if !l.Allow("acme") {
		t.Fatal("tras 100 ms debería haberse repuesto 1 token")
	}
}

func TestAllow_IsolatedPerTenant(t *testing.T) {
	clk := &fixedClock{t: time.Unix(0, 0)}
	l := limiterWithClock(plan.Plan{Name: "test", RatePerSec: 0, Burst: 1}, clk)

	if !l.Allow("acme") {
		t.Fatal("acme debería poder cursar su primera petición")
	}
	// El límite de acme no afecta a globex.
	if !l.Allow("globex") {
		t.Fatal("globex tiene su propio bucket y debería permitirse")
	}
	if l.Allow("acme") {
		t.Fatal("acme ya agotó su ráfaga")
	}
}

func TestAllow_RespectsPerTenantPlan(t *testing.T) {
	clk := &fixedClock{t: time.Unix(0, 0)}
	reg := plan.NewStaticRegistry(
		map[string]plan.Plan{"big": {Name: "big", RatePerSec: 0, Burst: 5}},
		plan.Plan{Name: "small", RatePerSec: 0, Burst: 1},
	)
	l := New(reg)
	l.now = clk.now

	allowed := 0
	for i := 0; i < 10; i++ {
		if l.Allow("big") {
			allowed++
		}
	}
	if allowed != 5 {
		t.Fatalf("tenant big (Burst=5) permitió %d, esperado 5", allowed)
	}
	if !l.Allow("small") || l.Allow("small") {
		t.Fatal("tenant con plan por defecto (Burst=1) debería permitir exactamente 1")
	}
}

// TestLoad_ConcurrentRespectsBurst es un test de carga ligero: lanza muchas
// peticiones concurrentes con el reloj congelado y verifica que nunca se superan
// los tokens de la ráfaga. Ejecutar con -race comprueba además la ausencia de
// data races.
func TestLoad_ConcurrentRespectsBurst(t *testing.T) {
	clk := &fixedClock{t: time.Unix(0, 0)}
	const burst = 100
	l := limiterWithClock(plan.Plan{Name: "test", RatePerSec: 0, Burst: burst}, clk)

	const goroutines = 50
	const perGoroutine = 20 // 1000 intentos totales
	var allowed int64
	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				if l.Allow("acme") {
					atomic.AddInt64(&allowed, 1)
				}
			}
		}()
	}
	wg.Wait()

	if allowed != burst {
		t.Fatalf("con reloj congelado se permitieron %d peticiones, esperado exactamente %d", allowed, burst)
	}
}
