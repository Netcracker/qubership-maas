---
name: troubleshoot-maas
description: Diagnose MaaS broker registration, 403/502 auth-proxy failures, unhealthy Kafka or RabbitMQ instances, topic/vhost registry drift, instance recovery, Cloud-Core deploy validation, and DR standby. Use when triaging MaaS tickets, maas-agent errors, /health showing no brokers, Kafka client timeouts, or PostgreSQL connectivity.
---

# Troubleshooting MaaS

MaaS (Messaging as a Service) registers Kafka topics and RabbitMQ vhosts
and returns broker connection metadata. It does **not** proxy traffic to
brokers.

`references/troubleshooting.md` is the fact source for known failure
modes. It is a copy of `docs/troubleshooting.md` in this repository.
Do **not** read it in full for a single symptom.

## Architecture

```text
App namespace (Cloud-Core)
  deployment/maas-configuration.yaml ──► maas-agent ──M2M──► maas-service
  kind: MaaS CR ──► core-operator ──► maas-agent ──► maas-service
  app pods ──direct produce/consume──► Kafka / RabbitMQ
                                         ▲
MaaS namespace                           │ metadata only
  maas-service :8080 GET /health ────────┘
  PostgreSQL (state; not on brokers)
```

- **maas-service** lives in its **own** namespace. Chart sources:
  `helm-templates/maas-service`. PostgreSQL is required.
- **Cloud-Core** in the application namespace includes **maas-agent**
  (security proxy; apps never call MaaS directly) and **core-operator**
  (processes `apiVersion: core.netcracker.com/v1`, `kind: MaaS` CRs).
- Application entities come from `deployment/maas-configuration.yaml` or
  those CRs — not from Kafka/Rabbit CLIs, except
  `externallyManaged: true` (see Forbidden actions).
- Instance registration is **optional** for MaaS to start. There is no
  mandatory post-install registration step. Current path is install-time
  envgen params `MAAS_KAFKA_INSTANCE_BODY` / `MAAS_RABBIT_INSTANCE_BODY`
  (and `_2` variants) written into secret `maas-instance-registrations`.
  Manager REST register still exists. `KafkaInstance` / `RabbitInstance`
  CRs are a future path — do not triage against them.
- Health is `GET /health` on port 8080. Missing `kafka` / `rabbit` keys
  means no instance is registered; MaaS itself can still be up.

If PostgreSQL already has a default instance, that **default flag wins**
over the flag in `MAAS_*_INSTANCE_BODY`.

## Reading the reference file

1. Grep headers with line numbers first:
   `grep -n '^#' references/troubleshooting.md`
2. Match the symptom against the jump table below (or the raw headers
   if the table is stale).
3. Read from the matched header through the **next header of the same
   or higher level**. A `##` section includes its `###` children; stop
   at the next `##` or `#`. Never load the whole file for one lookup.

## Symptom → reference section

| Symptom | Header in references/troubleshooting.md |
| --- | --- |
| HTTP 403 with credentials that look correct | Why MaaS responds with Forbidden (403) error code with correct credentials |
| Cloud-Core MaaS check: `There is wrong MAAS URL or MAAS is broken` or warning `no registered RabbitMQ/Kafka brokers yet` | Why Cloud-Core deployment fails with validation of MaaS |
| Kafka instance DOWN; `client has run out of available brokers` | Why Kafka instance could be unhealthy |
| Amazon MSK SSL / CA cert registration | Amazon case |
| maas-agent HTTP 502 / missing MaaS secrets / CMDB checkbox | Why maas-agent responds with Bad Gateway (502) status |
| Need client / demo service examples | Where to find MaaS libs examples |
| Broker crashed; need to move topics/vhosts to a new instance | Change instance after crash |
| Kafka list/create/update/delete times out | Kafka broker operations timeout |
| Topics/vhosts deleted on the broker but still in MaaS | Recover Kafka topics and Rabbit vhosts |
| Entity in MaaS registry, missing on Kafka or RabbitMQ | Entity exists in MaaS registry but missing in Kafka or RabbitMQ |

### Other docs (not in the troubleshooting reference)

| Symptom | Doc |
| --- | --- |
| Grafana status bar, DR, broker health, discrepancy panels | `docs/monitoring_guide.md` |
| Lost Kafka topic **names** (`status=absent`) | `docs/rest_api.md` header `Discrepancy Report` — `GET /api/v2/kafka/discrepancy-report/{ns}?status=absent`. Kafka only; no Rabbit equivalent. |
| Classifier fields and topic-name templates | `README.md#classifier` |
| Externally managed Kafka topics | `docs/kafka_externally_managed_topics.md` |

Start from the exact error text, `/health` JSON, and whether the caller
is maas-agent, deployer, or a direct MaaS API client. Check attachments
for logs and custom resources before asking for them.

## Classifier

A **classifier** is the composite identity of a MaaS entity (topic,
vhost, …). Canonical definition: `README.md#classifier`.

```json
{
  "name": "<custom name>",
  "namespace": "<caller namespace>",
  "tenantId": "<optional tenant id>"
}
```

`name` and `namespace` are mandatory. `tenantId` is optional.
`namespace` must match the caller namespace (or the composite
base-namespace). If Kafka `spec.name` is omitted, MaaS generates the
broker topic name from the classifier
(`maas.{{namespace}}.{{name}}` or
`maas.{{namespace}}.{{tenantId}}.{{name}}`).

## Deploy channel

Platform delivery is **envgen parameters → ArgoCD sync** (Cloud-Core
`HELM_V1`). Parameter names match chart values.

Phrase config changes as: set envgen parameter `<NAME>` to `<value>` in
the MaaS / Cloud-Core descriptor and let ArgoCD sync.

Do **not** recommend `helm upgrade`, `kubectl set env`, or
`kubectl edit` of the MaaS Deployment.

Failed install: inspect Helm pre-install/pre-upgrade hook Jobs
(`hook-delete-policy: before-hook-creation, hook-succeeded` — gone on
success, left on failure):

- `validation-hook-maas-service` — Postgres check only.
- `maas-service-pre-hook` — creates the DB, credentials, and cipher-key
  secrets.

Do not tell users to rerun a Jenkins job or look in an `openshift/`
folder.

### Important envgen parameters

| Parameter | Default | Meaning |
| --- | --- | --- |
| `EXECUTION_MODE` | `active` | DR role. Metric: `maas_db_dr_mode` (`0` active, `1` standby, `2` disabled). |
| `KAFKA_CLIENT_TIMEOUT` | `10s` | Kafka admin client timeout for list/create/update/delete. |
| `DB_POOL_SIZE` | `5` | PostgreSQL pool size. |
| `MONITORING_ENABLED` | `true` | PodMonitor / Grafana dashboards. |
| `M2M_AUTH_MODE` | `legacy` | Kubernetes M2M token auth. |
| `MAAS_KAFKA_INSTANCE_BODY` | unset | Optional install-time Kafka instance JSON. |
| `MAAS_RABBIT_INSTANCE_BODY` | unset | Optional install-time Rabbit instance JSON. |
| `MAAS_ACCOUNT_MANAGER_USERNAME` | `manager` | Bootstrap manager account. |
| `MAAS_DEPLOYER_CLIENT_USERNAME` | `client` | Bootstrap deployer account. |

### Secrets the pod mounts

| Secret | Mount |
| --- | --- |
| `maas-instance-registrations` | `/var/run/secrets/maas/maas-instance-registrations` |
| `maas-accounts` | `/var/run/secrets/maas/maas-accounts` |
| `maas-db-postgresql-credentials-secret` | `/var/run/secrets/maas/db` |
| `maas-db-cipher-key-secret` | `/var/run/secrets/maas/cipher` |

Missing or empty account files are **skipped silently** (log:
`File \`…\` is not exists, skip postdeploy action` or
`has empty body, skip processing`) — the account is never created.
On every **active-mode** restart, existing accounts get their password
**reset from the secret**. That is a common 403 after a secret change.
Missing DB secret → total unavailability.

### Roles

Install creates **two** accounts from `maas-accounts` (no `admin` user):

| Secret keys | Default username | Roles |
| --- | --- | --- |
| `manager-username` / `manager-password` | `manager` | `manager` |
| `deployer-username` / `deployer-password` | `client` | `manager` + `agent` |

Roles:

- `manager` — user accounts and Kafka/Rabbit **instance** registration.
- `agent` — vhosts, exchanges, queues, topics.
- `bgoperator` — blue/green APIs. Role only; not a bootstrap account.

## Forbidden actions

- Call MaaS REST from application microservices — always maas-agent + M2M.
- Use `manager` credentials on `agent` APIs (or the reverse).
- `kafka-topics.sh` / `rabbitmqadmin` create-or-delete for entities
  MaaS already registered — that is how registry drift starts. Recover
  or delete through MaaS APIs instead.
  **Exception:** externally managed Kafka topics
  (`docs/kafka_externally_managed_topics.md`) **must** be created with
  `kafka-topics.sh` first, then declared with `externallyManaged: true`
  and the real broker `name`. That is not drift.
- Expect changing the **default** instance id to move existing
  topics/vhosts. Default-instance changes do not relocate already
  stored entities; use instance update + recovery, or change
  `instanceId` in the app config and redeploy. See the crash section.
- Unregister a broker instance to fix **one namespace**. Unregister is
  cluster-wide and breaks every namespace on that broker. Only
  unregister when the broker is being decommissioned and no remaining
  namespace uses it.
- Recommend live `kubectl edit` / `kubectl set env` / `helm upgrade`
  as a permanent fix — change the envgen parameter and let ArgoCD sync.

## DR

Standby (`EXECUTION_MODE=standby`, `maas_db_dr_mode=1`) is not a crash.
Do not recommend broker re-registration or topic recovery solely because
of standby.

Client signals:

- Standby or read-only DB **writes**: HTTP **405** with
  `DR mode set to …` or `database is in read-only mode`.
- Master DB down: HTTP **503**. Reads can still be served from cache
  when it is populated. Metric `maas_health_is_master_db_alive=0`
  means **all writes fail** — fix PostgreSQL before chasing
  Kafka/Rabbit.

Confirm DR role on the Grafana status bar
(`docs/monitoring_guide.md`) before treating the pod as failed.

## Registry vs broker drift

MaaS stores topics/vhosts in PostgreSQL. Brokers can lose them
independently.

- Counts: metric `maas_discrepancy_lost_entities`.
- Kafka topic **names**:
  `GET /api/v2/kafka/discrepancy-report/{ns}?status=absent`.
  No Rabbit equivalent.

Resolution is in the reference file: recover from MaaS, or delete the
MaaS **entity** registration if it is obsolete. Never unregister the
broker instance as a namespace cleanup.

## First checks

1. `GET /health` — postgres, kafka, rabbit. Missing kafka/rabbit keys
   means no instance registered (MaaS can still be healthy).
2. DR mode and master DB panels on the MaaS Grafana dashboard
   (`docs/monitoring_guide.md`).
3. Whether the caller is maas-agent (502/secrets/CMDB) or MaaS itself
   (403/roles/accounts/registration).
4. Then jump to the matching reference section.
