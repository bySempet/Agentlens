-- Verificación del esquema del plano de control (E2-T08).
-- Ejecutar contra una BD con las migraciones aplicadas:
--   psql -h ... -U agentlens -d agentlens -v ON_ERROR_STOP=1 -f verify_schema.sql
\set ON_ERROR_STOP on

-- Alta encadenada con claves foráneas.
INSERT INTO organizations(slug, name, plan)
  VALUES ('acme-verify', 'ACME', 'growth') RETURNING id AS org_id \gset
INSERT INTO users(org_id, email, role) VALUES (:'org_id', 'a@acme.com', 'owner');
INSERT INTO agents(org_id, agent_id, name, framework)
  VALUES (:'org_id', 'support-bot', 'Support Bot', 'langchain');
INSERT INTO api_keys(org_id, name, key_hash) VALUES (:'org_id', 'default', 'hash-123');
INSERT INTO policies(org_id, name, rego) VALUES (:'org_id', 'base', 'package x');

-- CHECK de plan inválido: debe fallar (lo capturamos para no abortar).
DO $$
BEGIN
  INSERT INTO organizations(slug, name, plan) VALUES ('bad', 'Bad', 'ultra');
  RAISE EXCEPTION 'FALLO: se aceptó un plan inválido';
EXCEPTION WHEN check_violation THEN
  RAISE NOTICE 'ok: CHECK de plan rechaza valores inválidos';
END $$;

-- UNIQUE (org_id, agent_id): el duplicado debe fallar.
DO $$
DECLARE oid uuid;
BEGIN
  SELECT id INTO oid FROM organizations WHERE slug = 'acme-verify';
  INSERT INTO agents(org_id, agent_id, name) VALUES (oid, 'support-bot', 'dup');
  RAISE EXCEPTION 'FALLO: se aceptó un agent_id duplicado';
EXCEPTION WHEN unique_violation THEN
  RAISE NOTICE 'ok: UNIQUE (org_id, agent_id) rechaza duplicados';
END $$;

-- FK CASCADE: borrar la organización elimina sus filas hijas.
DELETE FROM organizations WHERE slug = 'acme-verify';
DO $$
DECLARE n int;
BEGIN
  SELECT count(*) INTO n FROM agents WHERE agent_id = 'support-bot';
  IF n <> 0 THEN
    RAISE EXCEPTION 'FALLO: el CASCADE no borró los agentes (quedan %)', n;
  END IF;
  RAISE NOTICE 'ok: FK ON DELETE CASCADE elimina las filas hijas';
END $$;

\echo 'ESQUEMA POSTGRES VERIFICADO'
