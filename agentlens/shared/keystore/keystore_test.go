package keystore

import "testing"

func TestStaticKeyStore(t *testing.T) {
	ks := NewStaticKeyStore(map[string]string{"k1": "acme"})
	if tenant, ok := ks.TenantForKey("k1"); !ok || tenant != "acme" {
		t.Fatalf("TenantForKey(k1) = %q, %v", tenant, ok)
	}
	if _, ok := ks.TenantForKey("desconocida"); ok {
		t.Fatal("una clave desconocida no debería resolver")
	}
}

func TestParseSpec(t *testing.T) {
	m, err := ParseSpec("k1:acme, k2:globex")
	if err != nil {
		t.Fatal(err)
	}
	if m["k1"] != "acme" || m["k2"] != "globex" {
		t.Fatalf("parseo incorrecto: %+v", m)
	}
}

func TestParseSpec_Empty(t *testing.T) {
	m, err := ParseSpec("")
	if err != nil || len(m) != 0 {
		t.Fatalf("cadena vacía debería dar mapa vacío sin error: %+v %v", m, err)
	}
}

func TestParseSpec_Invalid(t *testing.T) {
	for _, bad := range []string{"sintoken", "k:", ":tenant"} {
		if _, err := ParseSpec(bad); err == nil {
			t.Fatalf("%q debería ser inválido", bad)
		}
	}
}
