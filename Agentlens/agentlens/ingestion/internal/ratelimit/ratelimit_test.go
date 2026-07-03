package ratelimit

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeClock permite avanzar el tiempo de forma determinista en los tests.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

func newTestLimiter(plans map[string]Limits) (*TenantLimiter, *fakeClock) {
	clock := &fakeClock{t: time.Unix(1_000_000, 0)}
	l := New(plans)
	l.now = clock.now
	return l, clock
}

func TestAllow_BurstThenRejects(t *testing.T) {
	l, _ := newTestLimiter(map[string]Limits{"starter": {SpansPerSecond: 10, Burst: 20}})

	if !l.Allow("acme", "starter", 20) {
		t.Fatal("el burst inicial (20) debe admitirse")
	}
	if l.Allow("acme", "starter", 1) {
		t.Fatal("con el bucket vacío no debe admitirse ni 1 span")
	}
}

func TestAllow_RefillsAtPlanRate(t *testing.T) {
	l, clock := newTestLimiter(map[string]Limits{"starter": {SpansPerSecond: 10, Burst: 20}})

	if !l.Allow("acme", "starter", 20) {
		t.Fatal("el burst inicial debe admitirse")
	}
	clock.advance(1 * time.Second) // repone 10 tokens
	if !l.Allow("acme", "starter", 10) {
		t.Fatal("tras 1 s a 10 spans/s deben admitirse 10 spans")
	}
	if l.Allow("acme", "starter", 1) {
		t.Fatal("no debe admitirse más de lo repuesto")
	}
}

func TestAllow_RefillCapsAtBurst(t *testing.T) {
	l, clock := newTestLimiter(map[string]Limits{"starter": {SpansPerSecond: 10, Burst: 20}})

	if !l.Allow("acme", "starter", 20) {
		t.Fatal("el burst inicial debe admitirse")
	}
	clock.advance(1 * time.Hour) // repondría 36k tokens; el tope es Burst
	if !l.Allow("acme", "starter", 20) {
		t.Fatal("tras un periodo largo debe admitirse exactamente el burst")
	}
	if l.Allow("acme", "starter", 1) {
		t.Fatal("el bucket no debe acumular por encima del burst")
	}
}

func TestAllow_TenantsAreIsolated(t *testing.T) {
	l, _ := newTestLimiter(map[string]Limits{"starter": {SpansPerSecond: 10, Burst: 20}})

	if !l.Allow("acme", "starter", 20) {
		t.Fatal("el burst de acme debe admitirse")
	}
	if !l.Allow("globex", "starter", 20) {
		t.Fatal("agotar el bucket de acme no debe afectar a globex")
	}
}

func TestAllow_UnlimitedPlan(t *testing.T) {
	l, _ := newTestLimiter(map[string]Limits{"interno": {SpansPerSecond: 0}})

	for i := 0; i < 100; i++ {
		if !l.Allow("acme", "interno", 1_000_000) {
			t.Fatal("un plan sin límite nunca debe rechazar")
		}
	}
}

func TestAllow_UnknownPlanFallsBackToDefault(t *testing.T) {
	l, _ := newTestLimiter(nil) // DefaultPlans: starter = 100/s, burst 200

	// Un plan desconocido no debe abrir caudal infinito: aplica starter.
	if !l.Allow("acme", "plan-inventado", 200) {
		t.Fatal("el burst del plan por defecto debe admitirse")
	}
	if l.Allow("acme", "plan-inventado", 1) {
		t.Fatal("agotado el burst por defecto, debe rechazar")
	}
}

func TestAllow_BatchLargerThanBurstDrainsBucket(t *testing.T) {
	l, _ := newTestLimiter(map[string]Limits{"starter": {SpansPerSecond: 10, Burst: 20}})

	// Un batch mayor que el burst pasa (coste acotado a Burst)...
	if !l.Allow("acme", "starter", 500) {
		t.Fatal("un batch puntual mayor que el burst debe admitirse con el bucket lleno")
	}
	// ...pero deja el bucket vacío: el caudal sostenido sigue acotado.
	if l.Allow("acme", "starter", 1) {
		t.Fatal("tras un batch gigante el bucket debe quedar vacío")
	}
}

// TestAllow_ConcurrentLoad es el test de carga del criterio de aceptación de
// E2-T06: N goroutines compiten por el mismo bucket con el reloj congelado; el
// total admitido debe ser exactamente el burst, ni un span más, sin data races
// (ejecutar con -race).
func TestAllow_ConcurrentLoad(t *testing.T) {
	const (
		goroutines = 50
		perWorker  = 200
		burst      = 1_000
	)
	l, _ := newTestLimiter(map[string]Limits{"pro": {SpansPerSecond: 100, Burst: burst}})

	var admitted atomic.Int64
	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				if l.Allow("acme", "pro", 1) {
					admitted.Add(1)
				}
			}
		}()
	}
	wg.Wait()

	if got := admitted.Load(); got != burst {
		t.Fatalf("admitidos %d spans bajo carga; el límite exacto era %d", got, burst)
	}
}
