#!/usr/bin/env bash
set -e

if [[ -n "${FWD_DBAAS_URL}" ]]; then
  export dbaas_url=${FWD_DBAAS_URL}
elif [[ -n "${DBAAS_AGGREGATOR_ADDRESS}" ]]; then
  echo "Deprecated parameter DBAAS_AGGREGATOR_ADDRESS is not empty and will be used instead of FWD_DBAAS_URL and API_DBAAS_ADDRESS"
  echo "Please, consider using API_DBAAS_ADDRESS instead"
  export dbaas_url="${DBAAS_AGGREGATOR_ADDRESS}"
else
  export dbaas_url="${API_DBAAS_ADDRESS:=http://dbaas-aggregator.dbaas:8080}"
fi


function createDbaasAutoBalanceRules() {
      local rules=${DBAAS_LODB_PER_NAMESPACE_AUTOBALANCE_RULES// /} # remove all whitespaces
      rules=(${rules//||/ }) # split by ||
      for i in "${rules[@]}" ; do
          local rule=(${i//=>/ }) # split by =>
          local db_type="${rule[0]}"
          local phy_db_id="${rule[1]}"

          local rule_name="${NAMESPACE}-${db_type}"
          local rule_json=$(cat << EOF
  {
      "type": "${db_type}",
      "rule": {
          "config": {
              "perNamespace": {
                  "phydbid": "${phy_db_id}"
              }
          },
          "type": "perNamespace"
      }
  }
EOF
  )
          echo "Sending dbaas auto balancing rule ${rule_name}: ${rule_json}"
          local aggregator_rules_url="${dbaas_url}/api/v3/dbaas/${NAMESPACE}/physical_databases/balancing/rules/${rule_name}"

          HTTP_RESPONSE=$(echo "$rule_json" | curl --insecure -s --write-out "HTTPSTATUS:%{http_code}" -X PUT \
            -H "Authorization: Basic $(printf ${DBAAS_CLUSTER_DBA_CREDENTIALS_USERNAME}:${DBAAS_CLUSTER_DBA_CREDENTIALS_PASSWORD} | base64)" \
            -H 'content-type: application/json' \
            -d @- ${aggregator_rules_url}) || echo ""

          if [ -z $HTTP_RESPONSE ] ; then
              echo "Error creating dbaas per namespace balancing rule"
              exit 121
          fi

          HTTP_BODY=$(echo $HTTP_RESPONSE | sed -e 's/HTTPSTATUS\:.*//g')
          HTTP_STATUS=$(echo $HTTP_RESPONSE | tr -d '\n' | sed -e 's/.*HTTPSTATUS://')

          if [ $HTTP_STATUS -ne 201 ] && [ $HTTP_STATUS -ne 200 ]; then
              echo "Error creating dbaas per namespace balancing rule [HTTP status: $HTTP_STATUS]"
              echo "[HTTP body: $HTTP_BODY]"
              exit 121
          fi
      done
}

if [ ! -z "${DBAAS_LODB_PER_NAMESPACE_AUTOBALANCE_RULES}" ]; then
    echo "Creating DBaaS per namespace auto balance rules: ${DBAAS_LODB_PER_NAMESPACE_AUTOBALANCE_RULES}"
    createDbaasAutoBalanceRules
fi
