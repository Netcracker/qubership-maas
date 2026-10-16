# Proposal: MaaS broker-instance operator

## Why

Kafka and Rabbit instances are registered only through manager REST. GitOps cannot own broker connection config.
Credentials travel in the REST body. Two MaaS installs have no claim rule for the same CR. Operators and Argo CD already
manage brokers; MaaS still needs a separate REST step to learn them.

## What Changes

Add a Kubernetes operator **inside** `maas-service` that watches namespaced `KafkaInstance` and `RabbitInstance` CRs
(`maas.netcracker.com/v1`) in any namespace, claims those whose `spec.operatorNamespace` equals this install's
`CLOUD_NAMESPACE`, reads credentials from Secrets in the CR namespace, and Register/Update/Unregister's the existing
PostgreSQL instance rows via `KafkaInstanceService` / `RabbitInstanceService`.

Instance CRs are desired connection config. PostgreSQL remains the runtime store used by topic/vhost APIs. MaaS still
does not proxy broker traffic.

## In scope

- CRDs, Helm (`OPERATOR_ENABLED`, default-instance params, `K8S_EVENTS_ENABLED`, `restrictedEnvironment`), Lease,
  ProcessCR, Secrets read on every reconcile (no Secret Watch) plus refresh annotation, status (`Ready` + `Stalled`),
  Kubernetes Events, `managed_by_operator` REST lock, `deletionPolicy` Unregister/Orphan (Orphan hands the row back to
  REST), install order (MaaS first, then instance CRs), downgrade without Unregister.
- Rabbit `api_url` / `amqp_url` uniqueness in PostgreSQL (like Kafka `addresses`), for REST and CRs alike.
- Kubernetes 1.32+ (CEL + selectable fields).

## Non-goals

- Proxying Kafka or Rabbit traffic.
- Watching only `CLOUD_NAMESPACE` (broker CRs live in `kafka-infra` / `rabbit-infra`).
- A sibling operator Deployment or manager-REST hop from ProcessCR to another MaaS process.
- `DefaultInstance` CR or `spec.default` on the instance CR.
- Topic / vhost declaration CRDs (separate change `maas-declaration-crds`).

## Approach

Distill agreed behavior from [docs/operator_design.md](../../../docs/operator_design.md) into this change's delta specs.
Keep that document as architecture (diagrams, CRD sketch); link it instead of pasting mermaid or CRD YAML into spec.md.
This is a large cross-module change: separate SPEC PR first, implement only after it merges, then `/opsx:verify` and
archive in the completing Code PR.

Operator specs describe MaaS behavior only; do not name other products.

## Acceptance (Jira → scenarios)

No ticket is wired yet. Delta scenarios already cover:

| Area | Criterion |
| ------ | ----------- |
| Success | Claimed CR Register/Update → `Ready=True` reason `InstanceRegistered`. |
| Errors | `SecretError`, `HealthCheckFailed` (do not Unregister a previous good row), `InstanceInUse`, `InvalidSpec`, `DuplicateInstanceName`. |
| Compatibility | Manager REST while operator off; existing PG rows unmanaged until a CR writes them; `OPERATOR_ENABLED=false` and old-chart downgrade do not Unregister. |
| Uniqueness | A second Rabbit instance with an `apiUrl` or `amqpUrl` already used by another id is rejected (REST and CR). |
| Migrate back | `Orphan` delete leaves the row with `managed_by_operator` false; manager REST can Update it again. |

Copy these into Jira AC before archive.

## Links

- Architecture: [docs/operator_design.md](../../../docs/operator_design.md)
- Alternatives / Lease: [docs/operator_design_notes.md](../../../docs/operator_design_notes.md)
- Existing REST instance APIs: [docs/rest_api.md](../../../docs/rest_api.md)
