#!/usr/bin/env bash

set -euo pipefail

compose_service="postgres"
db_user="pte"
db_name="pte"
script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
migrations_dir="${script_dir}/migrations"

docker compose -f "${script_dir}/docker-compose.yml" up -d "${compose_service}"

shopt -s nullglob
migration_files=("${migrations_dir}"/*.sql)

if ((${#migration_files[@]} == 0)); then
    echo "No migration files found in ${migrations_dir}" >&2
    exit 1
fi

for migration_file in "${migration_files[@]}"; do
    migration_name="$(basename -- "${migration_file}")"
    echo "Running migration: ${migration_name}"

    docker compose -f "${script_dir}/docker-compose.yml" exec -T \
        "${compose_service}" \
        psql \
        -v ON_ERROR_STOP=1 \
        -U "${db_user}" \
        -d "${db_name}" \
        -f "/migrations/${migration_name}"
done

echo "Migrations completed."
