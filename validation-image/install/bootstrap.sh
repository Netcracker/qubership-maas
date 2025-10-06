#!/usr/bin/env bash

echo "Starting bootstrap scripts"

source /scripts/dbaas-autobalance.sh || exit 121
source /scripts/create-database.sh || exit 121
source /scripts/prepare-secrets.sh || exit 121
source /scripts/prepare-db-cipher-key-secret.sh || exit 121

echo "Finished bootstrap scripts"