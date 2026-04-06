#!/usr/bin/env bash

# Load parameters from mounted secret files when present (e.g. /var/run/secrets/maas/bootstrap)
MAAS_BOOTSTRAP_SECRETS_DIR="${MAAS_BOOTSTRAP_SECRETS_DIR:-/var/run/secrets/maas/bootstrap}"

log() {
  echo "[$(date -u '+%Y-%m-%dT%H:%M:%SZ')] $*"
}

log "Starting bootstrap process"

if [ -d "${MAAS_BOOTSTRAP_SECRETS_DIR}" ]; then
  log "Loading secrets from ${MAAS_BOOTSTRAP_SECRETS_DIR}"
  for f in "${MAAS_BOOTSTRAP_SECRETS_DIR}"/*; do
    if [ -f "${f}" ]; then
      key=$(basename "${f}")
      export "${key}=$(cat "${f}")"
      log "Loaded secret: ${key}"
    fi
  done
  log "Finished loading secrets"
else
  log "No secrets directory found at ${MAAS_BOOTSTRAP_SECRETS_DIR}, skipping"
fi

log "Starting bootstrap scripts"

log "Running dbaas-autobalance.sh"
source /scripts/dbaas-autobalance.sh || { log "ERROR: dbaas-autobalance.sh failed"; exit 121; }
log "Completed dbaas-autobalance.sh"

log "Running create-database.sh"
source /scripts/create-database.sh || { log "ERROR: create-database.sh failed"; exit 121; }
log "Completed create-database.sh"

log "Running prepare-secrets.sh"
source /scripts/prepare-secrets.sh || { log "ERROR: prepare-secrets.sh failed"; exit 121; }
log "Completed prepare-secrets.sh"

log "Running prepare-db-cipher-key-secret.sh"
source /scripts/prepare-db-cipher-key-secret.sh || { log "ERROR: prepare-db-cipher-key-secret.sh failed"; exit 121; }
log "Completed prepare-db-cipher-key-secret.sh"

log "Finished bootstrap scripts"
