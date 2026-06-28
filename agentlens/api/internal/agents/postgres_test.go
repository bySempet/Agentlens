package agents

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Test de integración del inventario sobre PostgreSQL real. Se salta si no se
// define AGENTLENS_TEST_PG_DSN (en CI lo provee un servicio postgres). El
// esquema debe estar migrado (deploy/postgres/migrate.sh).
func TestPostgresStore_CRUD(t *testing.T) {
	dsn := os.Getenv("AGENTLENS_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("define AGENTLENS_TEST_PG_DSN para el test de integración Postgres")
	}
	ctx := context.Background()

	// Tenant de prueba aislado.
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()
	const slug = "acme-agents-it"
	_, _ = pool.Exec(ctx, `DELETE FROM organizations WHERE slug=$1`, slug)
	if _, err := pool.Exec(ctx,
		`INSERT INTO organizations(slug,name,plan) VALUES($1,'IT','growth')`, slug); err != nil {
		t.Fatalf("seed org: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM organizations WHERE slug=$1`, slug) })

	s, err := NewPostgresStore(ctx, dsn)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	defer s.Close()

	a, err := s.Create(ctx, slug, CreateInput{Name: "Support Bot", Framework: "langchain"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if a.AgentID == "" || a.Status != "active" || a.Environment != "production" {
		t.Fatalf("agente creado inesperado: %+v", a)
	}

	list, err := s.List(ctx, slug)
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %v (%d)", err, len(list))
	}

	got, err := s.Get(ctx, slug, a.AgentID)
	if err != nil || got == nil || got.Name != "Support Bot" {
		t.Fatalf("get: %v %+v", err, got)
	}

	ok, err := s.Disable(ctx, slug, a.AgentID)
	if err != nil || !ok {
		t.Fatalf("disable: %v %v", err, ok)
	}
	got, _ = s.Get(ctx, slug, a.AgentID)
	if got.Status != "disabled" {
		t.Fatalf("el agente debería quedar disabled, %+v", got)
	}

	// Tenant inexistente.
	if _, err := s.Create(ctx, "tenant-fantasma", CreateInput{Name: "x"}); err != ErrTenantNotFound {
		t.Fatalf("esperado ErrTenantNotFound, %v", err)
	}
}
