package plan

import "testing"

func TestByName(t *testing.T) {
	p, ok := ByName("growth")
	if !ok || p != Growth {
		t.Fatalf("ByName(growth) = %v, %v", p, ok)
	}
	if _, ok := ByName("inexistente"); ok {
		t.Fatal("ByName de un plan inexistente debería devolver false")
	}
}

func TestStaticRegistry_MappedTenant(t *testing.T) {
	reg := NewStaticRegistry(map[string]Plan{"acme": Growth}, Free)
	if got := reg.PlanFor("acme"); got != Growth {
		t.Fatalf("PlanFor(acme) = %v, esperado Growth", got)
	}
}

func TestStaticRegistry_FallbackForUnknownTenant(t *testing.T) {
	reg := NewStaticRegistry(map[string]Plan{"acme": Growth}, Free)
	if got := reg.PlanFor("desconocido"); got != Free {
		t.Fatalf("PlanFor(desconocido) = %v, esperado fallback Free", got)
	}
}
