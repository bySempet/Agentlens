package agents

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrTenantNotFound se devuelve si el slug de tenant no existe como organización.
var ErrTenantNotFound = errors.New("tenant no encontrado")

// PostgresStore implementa Store sobre el plano de control PostgreSQL (E2-T08).
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore abre un pool de conexiones y verifica la conectividad.
func NewPostgresStore(ctx context.Context, dsn string) (*PostgresStore, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("abriendo Postgres: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping Postgres: %w", err)
	}
	return &PostgresStore{pool: pool}, nil
}

// Close cierra el pool.
func (s *PostgresStore) Close() { s.pool.Close() }

func (s *PostgresStore) orgID(ctx context.Context, tenant string) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx, `SELECT id FROM organizations WHERE slug = $1`, tenant).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrTenantNotFound
	}
	return id, err
}

func (s *PostgresStore) Create(ctx context.Context, tenant string, in CreateInput) (Agent, error) {
	org, err := s.orgID(ctx, tenant)
	if err != nil {
		return Agent{}, err
	}
	env := in.Environment
	if env == "" {
		env = "production"
	}
	a := Agent{AgentID: GenerateAgentID(in.Name), Name: in.Name, Framework: in.Framework, Environment: env}
	err = s.pool.QueryRow(ctx,
		`INSERT INTO agents (org_id, agent_id, name, framework, environment)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING status, created_at`,
		org, a.AgentID, a.Name, a.Framework, a.Environment,
	).Scan(&a.Status, &a.CreatedAt)
	return a, err
}

func (s *PostgresStore) List(ctx context.Context, tenant string) ([]Agent, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT a.agent_id, a.name, COALESCE(a.framework,''), a.environment, a.status, a.created_at
		 FROM agents a JOIN organizations o ON o.id = a.org_id
		 WHERE o.slug = $1 ORDER BY a.created_at DESC`, tenant)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Agent{}
	for rows.Next() {
		var a Agent
		if err := rows.Scan(&a.AgentID, &a.Name, &a.Framework, &a.Environment, &a.Status, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *PostgresStore) Get(ctx context.Context, tenant, agentID string) (*Agent, error) {
	var a Agent
	err := s.pool.QueryRow(ctx,
		`SELECT a.agent_id, a.name, COALESCE(a.framework,''), a.environment, a.status, a.created_at
		 FROM agents a JOIN organizations o ON o.id = a.org_id
		 WHERE o.slug = $1 AND a.agent_id = $2`, tenant, agentID).
		Scan(&a.AgentID, &a.Name, &a.Framework, &a.Environment, &a.Status, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *PostgresStore) Disable(ctx context.Context, tenant, agentID string) (bool, error) {
	tag, err := s.pool.Exec(ctx,
		`UPDATE agents SET status = 'disabled', updated_at = now()
		 WHERE agent_id = $2 AND org_id = (SELECT id FROM organizations WHERE slug = $1)`,
		tenant, agentID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}
