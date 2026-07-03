# Plano de control — PostgreSQL (E2-T08)

Esquema ACID multi-tenant para el plano de control: organizaciones (tenants),
usuarios, agentes (inventario, E3-T07), API keys y políticas (E4).

## Migraciones

Versionadas en `migrations/NNNN_*.sql` y aplicadas en orden por `migrate.sh`, que
registra cada una en la tabla `schema_migrations` (idempotente).

```bash
PSQL="psql -h localhost -p 5432 -U agentlens -d agentlens" ./migrate.sh
```

En el `docker-compose` local, las migraciones se montan en
`/docker-entrypoint-initdb.d` y se aplican al primer arranque del contenedor.

## Verificación

`verify_schema.sql` comprueba contra una instancia real: alta encadenada con
claves foráneas, los CHECK (plan/rol), el UNIQUE `(org_id, agent_id)` y el
`ON DELETE CASCADE`.

```bash
psql -h localhost -p 5432 -U agentlens -d agentlens -v ON_ERROR_STOP=1 -f verify_schema.sql
```

## Tablas

| Tabla | Propósito |
| --- | --- |
| `organizations` | Tenants, con plan (free/starter/growth/enterprise) |
| `users` | Usuarios por organización, con rol |
| `agents` | Inventario de agentes (agent_id estable usado en las trazas) |
| `api_keys` | Claves por organización (se guarda el hash, nunca en claro) |
| `policies` | Políticas Rego versionadas por organización |
| `schema_migrations` | Control de migraciones aplicadas |
