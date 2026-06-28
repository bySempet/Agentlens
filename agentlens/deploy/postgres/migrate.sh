#!/usr/bin/env bash
# Runner de migraciones SQL versionadas para el plano de control (E2-T08).
# Aplica en orden los ficheros migrations/*.sql que aún no consten en la tabla
# schema_migrations. Idempotente: re-ejecutar no vuelve a aplicar lo ya aplicado.
#
# Configura la conexión con la variable PSQL, p.ej.:
#   PSQL="psql -h localhost -p 5432 -U agentlens -d agentlens" ./migrate.sh
set -euo pipefail

PSQL=${PSQL:-psql}
DIR="$(cd "$(dirname "$0")" && pwd)/migrations"

$PSQL -v ON_ERROR_STOP=1 -q -c \
  "CREATE TABLE IF NOT EXISTS schema_migrations (
     version TEXT PRIMARY KEY,
     applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
   );"

for f in "$DIR"/*.sql; do
  v="$(basename "$f")"
  applied="$($PSQL -tAc "SELECT 1 FROM schema_migrations WHERE version='$v'")"
  if [ "$applied" = "1" ]; then
    echo "skip   $v (ya aplicada)"
    continue
  fi
  echo "apply  $v"
  $PSQL -v ON_ERROR_STOP=1 -q -f "$f"
  $PSQL -v ON_ERROR_STOP=1 -q -c "INSERT INTO schema_migrations(version) VALUES ('$v');"
done

echo "migraciones al día."
