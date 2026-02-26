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

echo "dbaas_url=${dbaas_url}"

if [[ -n ${DB_POSTGRESQL_ADDRESS} ]] && [[ -n ${DB_POSTGRESQL_DATABASE} ]] && [[ -n ${DB_POSTGRESQL_USERNAME} ]] && [[ -n ${DB_POSTGRESQL_PASSWORD} ]]; then
  echo "You specified PostgreSQL parameters, it is assumed you have created database manually."
elif [[ -n ${DBAAS_CLUSTER_DBA_CREDENTIALS_USERNAME} ]] && [[ -n ${DBAAS_CLUSTER_DBA_CREDENTIALS_PASSWORD} ]]; then
    echo "Start creating database through DbaaS"
    dbaas_response=$(cat << EOF | curl --insecure -k -s --write-out "HTTPSTATUS:%{http_code}" -X PUT \
          "${dbaas_url}/api/v3/dbaas/${NAMESPACE}/databases" \
          -H "Authorization: Basic $(printf ${DBAAS_CLUSTER_DBA_CREDENTIALS_USERNAME}:${DBAAS_CLUSTER_DBA_CREDENTIALS_PASSWORD} | base64 -w 0)" \
          -H 'Content-Type: application/json' \
          -d '
           {
                "originService":"maas-service",
                "classifier":{
                    "microserviceName":"maas-service",
                    "namespace":"'${NAMESPACE}'",
                    "scope":"service"
                },
                "type":"postgresql",
                "namePrefix":"maas"
            } '
EOF
)
    http_status=$(echo  "${dbaas_response}" | tr -d '\n' | sed -e 's/.*HTTPSTATUS://')

    if [[ ${http_status} -eq 202 ]]; then
        echo "Dbaas responded with code 202 - need to make a retry"
        echo "Response: ${dbaas_response}"

        for i in {1..100}
        do
        sleep 3
        echo "Retry number: $i"
        dbaas_response=$(cat << EOF | curl --insecure -k -s --write-out "HTTPSTATUS:%{http_code}" -X PUT \
                    "${dbaas_url}/api/v3/dbaas/${NAMESPACE}/databases" \
                    -H "Authorization: Basic $(printf ${DBAAS_CLUSTER_DBA_CREDENTIALS_USERNAME}:${DBAAS_CLUSTER_DBA_CREDENTIALS_PASSWORD} | base64 -w 0)" \
                    -H 'Content-Type: application/json' \
                    -d '
                     {
                          "originService":"maas-service",
                          "classifier":{
                              "microserviceName":"maas-service",
                              "namespace":"'${NAMESPACE}'",
                              "scope":"service"
                          },
                          "type":"postgresql",
                          "namePrefix":"maas"
                      } '
EOF
)
          http_status=$(echo  "${dbaas_response}" | tr -d '\n' | sed -e 's/.*HTTPSTATUS://')
          echo "Response: ${dbaas_response}"

           if [[ ! ${http_status} -eq 202 ]]; then
              echo "HTTP status: ${http_status}"
              echo "Status changed from 202, end retries"
              break
           fi

        done
    fi

    if [[ ! ${http_status} -eq 201 ]] &&  [[ ! ${http_status} -eq 200 ]]; then
          echo "Error creating database after retries, http status code should be 200 or 201"
          echo "Response: ${dbaas_response}"
        exit 121
    fi

    http_body=$(echo  ${dbaas_response} | sed -e 's/HTTPSTATUS\:.*//g')
    maas_db_host=$(echo ${http_body} | python -c 'import json,sys; print(json.load(sys.stdin)["connectionProperties"]["host"])')
    maas_db_port=$(echo ${http_body} | python -c 'import json,sys; print(json.load(sys.stdin)["connectionProperties"]["port"])')

    export DB_POSTGRESQL_ADDRESS="${maas_db_host}:${maas_db_port}"
    export DB_POSTGRESQL_DATABASE=$(echo ${http_body} | python -c 'import json,sys; print(json.load(sys.stdin)["name"])')
    export DB_POSTGRESQL_USERNAME=$(echo ${http_body} | python -c 'import json,sys; print(json.load(sys.stdin)["connectionProperties"]["username"])')
    export DB_POSTGRESQL_PASSWORD=$(echo ${http_body} | python -c 'import json,sys; print(json.load(sys.stdin)["connectionProperties"]["password"])')
    export DB_POSTGRESQL_TLS=$(echo ${http_body} | python -c 'import json,sys; print(json.load(sys.stdin)["connectionProperties"].get("tls", "false"))')

    echo "database host: ${DB_POSTGRESQL_ADDRESS}"
    echo "database name: ${DB_POSTGRESQL_DATABASE}"
    echo "tls enabled: ${DB_POSTGRESQL_TLS}"
else
    echo "ERROR: you must either specify DbaaS parameters [DBAAS_CLUSTER_DBA_CREDENTIALS_USERNAME, DBAAS_CLUSTER_DBA_CREDENTIALS_PASSWORD] to create database automatically or specify PostgreSQL parameters [DB_POSTGRESQL_ADDRESS, DB_POSTGRESQL_DATABASE, DB_POSTGRESQL_USERNAME, DB_POSTGRESQL_PASSWORD] if you created database manually"
    exit 121
fi
