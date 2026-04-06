#!/usr/bin/env bash

# Load parameters from mounted secret files when present (e.g. /var/run/secrets/maas/bootstrap)
MAAS_BOOTSTRAP_SECRETS_DIR="${MAAS_BOOTSTRAP_SECRETS_DIR:-/var/run/secrets/maas/bootstrap}"
ls -a -l /var/run/secrets/maas/bootstrap
if [ -d "${MAAS_BOOTSTRAP_SECRETS_DIR}" ]; then
  for f in "${MAAS_BOOTSTRAP_SECRETS_DIR}"/*; do
    if [ -f "${f}" ]; then
      key=$(basename "${f}")
      export "${key}=$(cat "${f}")"
      echo "${key}=$(cat "${f}")"
    fi
  done
fi

echo "Starting bootstrap scripts"

source /scripts/dbaas-autobalance.sh || exit 121
source /scripts/create-database.sh || exit 121
source /scripts/prepare-secrets.sh || exit 121
source /scripts/prepare-db-cipher-key-secret.sh || exit 121

echo "Finished bootstrap scripts"
