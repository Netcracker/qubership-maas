#!/usr/bin/env bash
set -e

echo "Creating maas-db-postgresql-credentials-secret"

cat << EOF | kubectl --namespace="${NAMESPACE}" apply -f -
{
      "apiVersion": "v1",
      "kind": "Secret",
      "metadata": {
        "name": "maas-db-postgresql-credentials-secret"
      },
      "data": {
        "dbname": "$(printf '%s' "${DB_POSTGRESQL_DATABASE:-}" | base64 -w 0)",
        "pg_address": "$(printf '%s' "${DB_POSTGRESQL_ADDRESS:-}" | base64 -w 0)",
        "username": "$(printf '%s' "${DB_POSTGRESQL_USERNAME:-}" | base64 -w 0)",
        "password": "$(printf '%s' "${DB_POSTGRESQL_PASSWORD:-}" | base64 -w 0)",
        "tls": "$(printf '%s' "${DB_POSTGRESQL_TLS_ENABLED:-false}" | base64 -w 0)"
      }
    }
EOF
