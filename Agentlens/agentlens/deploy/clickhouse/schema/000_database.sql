-- E2-T07 · Base de datos de AgentLens.
--
-- En docker-compose la crea también el entrypoint (CLICKHOUSE_DB); se declara
-- aquí para que el esquema sea autocontenido fuera de Docker (CI, binario local).

CREATE DATABASE IF NOT EXISTS agentlens;
