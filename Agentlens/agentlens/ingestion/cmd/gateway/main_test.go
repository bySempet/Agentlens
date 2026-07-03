package main

import (
	"testing"

	"github.com/bysempet/agentlens/ingestion/internal/ratelimit"
)

func TestParseAPIKeys_TenantAndOptionalPlan(t *testing.T) {
	keys := parseAPIKeys("k1:acme, k2:globex:pro")

	acme, ok := keys["k1"]
	if !ok || acme.ID != "acme" {
		t.Fatalf("k1 debía resolver a acme, obtenido %+v (ok=%v)", acme, ok)
	}
	if acme.Plan != ratelimit.DefaultPlan {
		t.Fatalf("sin plan explícito debe aplicarse %q, obtenido %q", ratelimit.DefaultPlan, acme.Plan)
	}
	globex := keys["k2"]
	if globex.ID != "globex" || globex.Plan != "pro" {
		t.Fatalf("k2 debía resolver a globex/pro, obtenido %+v", globex)
	}
}

func TestParsePlanLimits_OverridesAndAdds(t *testing.T) {
	plans := parsePlanLimits("starter=5:10, interno=0")

	if got := plans["starter"]; got.SpansPerSecond != 5 || got.Burst != 10 {
		t.Fatalf("starter debía sobreescribirse a 5:10, obtenido %+v", got)
	}
	if got := plans["interno"]; !got.Unlimited() {
		t.Fatalf("interno=0 debía ser sin límite, obtenido %+v", got)
	}
	// Los planes no mencionados conservan sus defaults.
	if got, def := plans["pro"], ratelimit.DefaultPlans()["pro"]; got != def {
		t.Fatalf("pro debía conservar el default %+v, obtenido %+v", def, got)
	}
}

func TestParsePlanLimits_BurstDefaultsToTwiceRate(t *testing.T) {
	plans := parsePlanLimits("basico=50")
	if got := plans["basico"]; got.SpansPerSecond != 50 || got.Burst != 100 {
		t.Fatalf("basico=50 debía ser 50 spans/s con burst 100, obtenido %+v", got)
	}
}
