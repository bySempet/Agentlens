-- Esquema PostgreSQL del plano de control de AgentLens (E2-T08).
-- Modelo ACID para organizaciones, usuarios, agentes y políticas. Multi-tenant:
-- toda fila de negocio cuelga de una organización (tenant).
--
-- Migración idempotente (IF NOT EXISTS) y versionada. El runner registra cada
-- migración aplicada en schema_migrations.

BEGIN;

-- Extensiones.
CREATE EXTENSION IF NOT EXISTS "pgcrypto";  -- gen_random_uuid()

-- Organizaciones (tenants).
CREATE TABLE IF NOT EXISTS organizations (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug        TEXT NOT NULL UNIQUE,           -- identificador legible (tenant_id)
    name        TEXT NOT NULL,
    plan        TEXT NOT NULL DEFAULT 'free'
                CHECK (plan IN ('free', 'starter', 'growth', 'enterprise')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Usuarios de una organización.
CREATE TABLE IF NOT EXISTS users (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    email       TEXT NOT NULL,
    role        TEXT NOT NULL DEFAULT 'member'
                CHECK (role IN ('owner', 'admin', 'member', 'viewer')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (org_id, email)
);
CREATE INDEX IF NOT EXISTS idx_users_org ON users(org_id);

-- Agentes registrados (inventario, E3-T07).
CREATE TABLE IF NOT EXISTS agents (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    agent_id    TEXT NOT NULL,                  -- id estable usado en las trazas
    name        TEXT NOT NULL,
    framework   TEXT,                           -- openai | langchain | crewai | ...
    environment TEXT NOT NULL DEFAULT 'production',
    status      TEXT NOT NULL DEFAULT 'active'
                CHECK (status IN ('active', 'disabled')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (org_id, agent_id)
);
CREATE INDEX IF NOT EXISTS idx_agents_org ON agents(org_id);

-- API keys por organización (autenticación del SDK/gateway/API).
CREATE TABLE IF NOT EXISTS api_keys (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    key_hash    TEXT NOT NULL UNIQUE,           -- hash de la clave, nunca en claro
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at  TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_api_keys_org ON api_keys(org_id);

-- Políticas de gobernanza (Rego), versionadas por organización (E4).
CREATE TABLE IF NOT EXISTS policies (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    rego        TEXT NOT NULL,                  -- fuente Rego de la política
    version     INTEGER NOT NULL DEFAULT 1,
    enabled     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (org_id, name, version)
);
CREATE INDEX IF NOT EXISTS idx_policies_org ON policies(org_id);

COMMIT;
