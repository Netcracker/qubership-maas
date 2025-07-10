#!/usr/bin/env bash
set -e

kubectl --namespace="${ENV_NAMESPACE}" get secret maas-db-cipher-key-secret &> /dev/null
if [ "$?" -ne 0 ]; then
	echo "Going to create a new database cipher key secret."
  (cat << EOF | kubectl --namespace="${ENV_NAMESPACE}" apply -f -
   {
     "apiVersion": "v1",
     "kind": "Secret",
     "metadata": {
       "name": "maas-db-cipher-key-secret"
     },
     "data": {
       "key": "$(tr -dc 'A-Za-z0-9!?%=' < /dev/urandom | head -c 32 | base64 -w 0)"
     }
   }
EOF
   ) || exit 121
	echo "Database cipher key secret created."
else
  echo "Database cipher key secret is already exists. Skip creation."
fi
