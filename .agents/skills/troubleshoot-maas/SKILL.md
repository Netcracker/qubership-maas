---
name: troubleshoot-maas
description: Diagnose MaaS broker registration, 403/502 auth-proxy failures, unhealthy Kafka or RabbitMQ instances, topic/vhost registry drift, instance recovery, Cloud-Core deploy validation, and DR standby. Use when triaging MaaS tickets, maas-agent errors, /health showing no brokers, Kafka client timeouts, or PostgreSQL connectivity.
---

# Troubleshooting MaaS

MaaS (Messaging as a Service) registers Kafka topics and RabbitMQ vhosts
and returns broker connection metadata. It does **not** proxy traffic to
brokers. Applications talk to MaaS through **maas-agent** with M2M tokens.

`references/troubleshooting.md` is the fact source for known failure
modes. It is a copy of `docs/troubleshooting.md` in this repository.
Do **not** read it in full for a single symptom.

## Reading the reference file

1. Grep headers with line numbers first:
   `grep -n '^#' references/troubleshooting.md`
2. Match the symptom against the jump table below (or the raw headers
   if the table is stale).
3. Read only that section: offset at its line number, limit through to
   the next header. Never load the whole file for one lookup.

## Symptom → reference section

| Symptom | Header in references/troubleshooting.md |
| --- | --- |
| HTTP 403 with credentials that look correct | Why MaaS responds with Forbidden (403) error code with correct credentials |
| Cloud-Core deploy fails MaaS validation; `There are no one registered broker!` | Why Cloud-Core deployment fails with validation of MaaS |
| Kafka instance DOWN; `client has run out of available brokers` | Why Kafka instance could be unhealthy |
| Amazon MSK SSL / CA cert registration | Amazon case |
| maas-agent HTTP 502 / missing MaaS secrets / CMDB checkbox | Why maas-agent responds with Bad Gateway (502) status |
| Need client / demo service examples | Where to find MaaS libs examples |
| Broker crashed; need to move topics/vhosts to a new instance | Change instance after crash |
| Kafka list/create/update/delete times out | Kafka broker operations timeout |
| Topics/vhosts deleted on the broker but still in MaaS | Recover Kafka topics and Rabbit vhosts |
| Entity in MaaS registry, missing on Kafka or RabbitMQ | Entity exists in MaaS registry but missing in Kafka or RabbitMQ |

Start from the exact error text, `/health` JSON, and whether the caller
is maas-agent, deployer, or a direct MaaS API client. Check ticket
attachments for logs and custom resources before asking for them.

## Deployment

- Chart: `helm-templates/maas-service`. Deploy MaaS in its **own**
  namespace. PostgreSQL is required; state lives there, not on brokers.
- After install, Cloud Ops must register at least one Kafka or Rabbit
  instance (REST or install-time secrets). Health is `GET /health` on
  port 8080.
- Application entities are declared in `deployment/maas-configuration.yaml`
  or `kind: MaaS` CRs processed by core-operator — not by talking to
  Kafka/Rabbit CLIs.
- Apps must not call MaaS directly. They use maas-agent as a security
  proxy with an M2M token.

### Important Helm / env values

| Value | Default | Meaning |
| --- | --- | --- |
| `EXECUTION_MODE` | `active` | DR role. `standby` pods do not serve traffic. Metric: `maas_db_dr_mode` (`0` active, `1` standby, `2` disabled). |
| `KAFKA_CLIENT_TIMEOUT` | `10s` | Kafka admin client timeout for list/create/update/delete. |
| `DB_POOL_SIZE` | `5` | PostgreSQL pool size. |
| `MONITORING_ENABLED` | `true` | PodMonitor / Grafana dashboards. |
| `KUBERNETES_M2M_ENABLED` | `false` | Kubernetes M2M token auth. |

Phrase config changes as «set `<helm-value>: <value>` in the
`maas-service` chart and redeploy through the normal delivery
channel», not as a live `kubectl set env`.

### Secrets the pod mounts

| Secret | Mount |
| --- | --- |
| `maas-instance-registrations` | `/var/run/secrets/maas/maas-instance-registrations` |
| `maas-accounts` | `/var/run/secrets/maas/maas-accounts` |
| `maas-db-postgresql-credentials-secret` | `/var/run/secrets/maas/db` |
| `maas-db-cipher-key-secret` | cipher key |

Missing or empty secrets after a partial install commonly surface as
403 (accounts) or total unavailability (DB).

### Roles

- `manager` — user accounts and Kafka/Rabbit **instance** registration.
- `agent` — vhosts, exchanges, queues, topics.
- Install creates `admin` with `manager` only. Deployer typically needs
  **both** roles.

Classifier identity is `name` + `namespace` (required), optional
`tenantId`. Namespace on the classifier must match the caller namespace
(or the composite base-namespace).

## Forbidden actions

- Call MaaS REST from application microservices — always maas-agent + M2M.
- Use `manager` credentials on `agent` APIs (or the reverse).
- `kafka-topics.sh` / `rabbitmqadmin` create-or-delete for entities
  MaaS already registered — that is how registry drift starts. Recover
  or delete through MaaS APIs instead.
- Expect changing the **default** instance id to move existing
  topics/vhosts. Default-instance changes do not relocate already
  stored entities; use instance update + recovery, or change
  `instanceId` in the app config and redeploy. See the crash section.
- Recommend live `kubectl edit` of MaaS Deployment env as a permanent
  fix — values come from the chart.

## DR

Standby (`EXECUTION_MODE=standby`, `maas_db_dr_mode=1`) is not a crash.
Do not recommend broker re-registration or topic recovery solely because
standby pods do not serve traffic. Confirm DR role on the Grafana
status bar before treating the pod as failed.

Master DB down (`maas_health_is_master_db_alive=0`) means **all writes
fail** — fix PostgreSQL before chasing Kafka/Rabbit.

## Registry vs broker drift

MaaS stores topics/vhosts in PostgreSQL. Brokers can lose them
independently. Metric `maas_discrepancy_lost_entities` is the signal.
Resolution is in the reference file: recover from MaaS, or delete the
MaaS registration if the entity is obsolete.

## First checks

1. `GET /health` — postgres, kafka, rabbit. Missing kafka/rabbit keys
   means no instance registered.
2. DR mode and master DB panels on the MaaS Grafana dashboard.
3. Whether the caller is maas-agent (502/secrets/CMDB) or MaaS itself
   (403/roles/registration).
4. Then jump to the matching reference section.
