#!/bin/bash

check_postgres() {
  echo "Start to check postgresql's parameters..."

  if [ -z "${DB_POSTGRESQL_ADDRESS}" ] || [ -z "${DB_POSTGRESQL_DATABASE}" ] || [ -z "${DB_POSTGRESQL_USERNAME}" ] || [ -z "${DB_POSTGRESQL_PASSWORD}" ]; then
      echo "At least one of parameters DB_POSTGRESQL_ADDRESS, DB_POSTGRESQL_DATABASE, DB_POSTGRESQL_USERNAME, DB_POSTGRESQL_PASSWORD is empty, skipping Postgres validation "
      echo
      return 0
  fi

  POSTGRES_HOST=${DB_POSTGRESQL_ADDRESS%%:*}
  POSTGRES_PORT=${DB_POSTGRESQL_ADDRESS##*:}

  POSTGRES_PORT="${POSTGRES_PORT:-"5432"}"
  (echo > /dev/tcp/${POSTGRES_HOST}/${POSTGRES_PORT}) 2>/dev/null
  if [ $(echo $?) != 0 ]; then
    echo "ERROR! CHECK FAILED!"
    echo "Wrong parameter:"
    echo "POSTGRES_HOST=${POSTGRES_HOST}"
    echo "POSTGRES_PORT=${POSTGRES_PORT}"
    exit ${ERROR_EXIT_CODE}
  fi
  echo 'Param DB_POSTGRESQL_ADDRESS is correct'

  IS_DB_EXISTS=$(psql "host=${POSTGRES_HOST} port=${POSTGRES_PORT} user=${DB_POSTGRESQL_USERNAME} password=${DB_POSTGRESQL_PASSWORD} dbname=postgres" -lqt | cut -d \| -f 1 | grep -qw ${DB_POSTGRESQL_DATABASE} || echo FAIL)
  if [ "${IS_DB_EXISTS}" = "FAIL" ]; then
    echo "ERROR! CHECK FAILED!"
    echo "DB_POSTGRESQL_DATABASE=${DB_POSTGRESQL_DATABASE} does not exists."
    exit ${ERROR_EXIT_CODE}
  fi
  echo 'Params DB_POSTGRESQL_USERNAME, DB_POSTGRESQL_PASSWORD are correct'

  echo "Checking postgresql's parameters is completed."
}

#########################

echo 'Validation of MaaS parameters by internal image is starting...'

check_postgres

echo 'Validation of MaaS parameters by internal image was completed successfully!'


exit 0
