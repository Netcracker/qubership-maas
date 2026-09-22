# MaaS broker-discovery operator

Design proposal (not implemented yet). Do not confuse with `[custom_resources(CR).md](custom_resources(CR)`.md) (`kind: MaaS` Topic/VHost declarations processed by core-operator).

Background (Lease lock, alternatives): `[operator_design_notes.md](operator_design_notes.md)`.

## Table of Contents

- [Overview](#overview)
- [High-Level Architecture](#high-level-architecture)
  - [Backward compatibility and downgrade](#backward-compatibility-and-downgrade)
  - [Secret access (namespaced)](#secret-access-namespaced)
  - [Restricted environment](#restricted-environment)
  - [Reconcile](#reconcile)
- [Prerequisites and Installation](#prerequisites-and-installation)
  - [MaaS first, then instance CRs](#maas-first-then-instance-crs)
  - [Instance CRs first (otherwise)](#instance-crs-first-otherwise)
- [CRD sketch](#crd-sketch)
  - [KafkaInstance](#kafkainstance)
  - [RabbitInstance](#rabbitinstance)
  - [Secrets](#secrets)
  - [Status](#status)
- [Kubernetes Events](#kubernetes-events)
- [Mapping layer (CR vs service structs)](#mapping-layer-cr-vs-service-structs)
- [Topology](#topology)
  - [Single microservice](#single-microservice-reconciler-and-maas-logic-in-one-process)
  - [Scaling if it is a single service](#scaling-if-it-is-a-single-service)
- [Special cases](#special-cases)
  - [Default instance](#default-instance)
  - [Name and id in the database](#name-and-id-in-the-database)
  - [CR vs REST (and optional takeOver)](#cr-vs-rest-and-optional-takeover)
  - [CR lifecycle](#cr-lifecycle-watcher-create-vs-update-vs-delete-delete-with-existing-topics-or-vhosts)
    - [Watcher, create vs update vs delete](#watcher-create-vs-update-vs-delete)
    - [Delete](#delete)
    - [Delete with existing topics or vhosts](#delete-with-existing-topics-or-vhosts)
  - [Multi-MaaS and ownership](#multi-maas-and-ownership)
- [Related notes](#related-notes)
- [TODO](#todo)

## Overview

MaaS operator is a Kubernetes operator that integrates with maas-service. It runs cluster-wide and manages the following custom resources (CRs):

| Custom Resource | API Group | Scope | Purpose |
|-----------------|-----------|-------|---------|
| `KafkaInstance` | `maas.netcracker.com/v1` | Namespaced | Registers a Kafka broker with maas-service |
| `RabbitInstance` | `maas.netcracker.com/v1` | Namespaced | Registers a RabbitMQ broker with maas-service |

`KafkaInstance` / `RabbitInstance` map 1:1 onto `[model.KafkaInstance](../maas/maas-service/model/kafka_model.go)` / `[model.RabbitInstance](../maas/maas-service/model/rabbit_model.go)`. Instance CRs are the **desired** connection config; MaaS DB remains the **runtime** store used by topic/vhost APIs. Default Kafka/Rabbit instance: MaaS Application `DEFAULT_KAFKA_INSTANCE` / `DEFAULT_RABBIT_INSTANCE` ([Default instance](#default-instance)).

ProcessCR is one function (claim, finalizer, Register vs Update). See [High-Level Architecture](#high-level-architecture).

## High-Level Architecture

```mermaid
flowchart TD
  subgraph maas [MaaS]
    service[maas-service]
  end
  service -.-> en["OPERATOR_ENABLED: Watch + ProcessCR can be enabled or disabled"]
  service --> watch[CR Watcher]
  watch --> event["Watch event"]
  event --> load["Get CR from apiserver by Watch key namespace/name"]
  load -->|"NotFound"| skipNf["Skip"]
  load -->|"Get succeeded"| del{"Is it DELETE?\n(metadata.deletionTimestamp set)"}
  del -->|"yes: kubectl delete, CR is Terminating"| policy{"Delete registration in db?\n(spec.deletionPolicy is Orphan or Unregister)"}
  policy -->|"Orphan: drop CR, keep PG row"| rmFin["PATCH CR metadata: remove finalizer. Apiserver then drops the CR"]
  policy -->|"Unregister: also delete PG row, MaaS 200"| unregDel["MaaS Unregister"]
  unregDel --> rmFin
  policy -->|"Unregister, MaaS 400 topics still on instance"| inUse["PATCH CR status Ready=False reason=InstanceInUse. Keep finalizer. RequeueAfter 30s"]
  del -->|"no: live CR"| claim{"Does this MaaS install claim the CR?\n(we own spec.operatorNamespace)"}
  claim -->|"no"| stop["Stop. Do not PATCH status"]
  claim -->|"yes"| stale{"Spec or Secret changed, Ready not True, or 10m resync?"}
  stale -->|no| skip["Skip apply. Keep status"]
  stale -->|yes| val["Validate spec"]
  val --> secrets["Read Secrets. Map to KafkaInstance or RabbitInstance"]
  secrets --> apply{"Do we store a row in DB for this CR?"}
  apply -.-> nameId["TODO: name vs id in PG"]
  apply -->|no row| reg["New instance: add new row to DB,\n set managed_by_operator true, set default true if no prev default"]
  apply -->|"managed_by_operator true, origin_cr is this CR"| upd["MaaS Update. Same CR, not a new one"]
  apply -->|"managed_by_operator false"| adopt["MaaS Update. Adopt: set managed_by_operator true and origin_cr"]
  apply -->|"managed_by_operator true, origin_cr is another CR"| dup["PATCH status Ready=False reason=DuplicateInstanceName"]
  reg --> fin["Ensure finalizer on CR metadata"]
  upd --> fin
  adopt --> fin
  fin --> patch["PATCH CR status: phase, Ready, Stalled, lastRequestId, observedGeneration, isDefault, secretRevisions"]
  classDef todo fill:#fff4cc,stroke:#c9a227,color:#1a1a1a
  classDef tip fill:#e8f4fc,stroke:#4a90c4,color:#1a1a1a
  class todoTake,todoReady,nameId todo
  class en tip
```



**Key design decisions:**

- The operator runs **cluster-wide** — no static `--watch-namespaces` list. Instance CRs may live in any namespace.
- Each managed CR declares its operator in immutable `spec.operatorNamespace`.
- CRs whose `spec.operatorNamespace` differs from this MaaS `CLOUD_NAMESPACE` are silently skipped (no PATCH, no Register).
- Credentials for `KafkaInstance` / `RabbitInstance` are read from Kubernetes Secrets at reconcile. The operator **Watches** those Secrets. A Secret `resourceVersion` change enqueues the CR even when spec `generation` did not change. `status.secretRevisions` stores those revisions, never secret bytes.
- **Periodic resync every 10 minutes.** Each claimed instance CR is reconciled again (`MAAS_INSTANCE_RESYNC_INTERVAL`, default `10m`) even when spec and Secrets did not change. Re-reads Secrets, refreshes `status.isDefault` from PG (install default names), and retries InUse / SecretError. When ProcessCR runs and backoff: [Reconcile](#reconcile).
- Secret access is **namespaced**, not cluster-wide: the ClusterRole carries no `secrets` permission. Each namespace containing Secret-backed CRs grants access through a small Role + RoleBinding — see [Secret access (namespaced)](#secret-access-namespaced).
- **One Deployment.** ProcessCR runs in `maas-service`, same process as Fiber. No sibling operator pod, no manager REST hop, no extra basic-auth/M2M to another MaaS process. Apply is in-process `KafkaInstanceService` / `RabbitInstanceService`. See [Single microservice](#single-microservice-reconciler-and-maas-logic-in-one-process).
- **Lease, not a singleton pod.** HPA still scales HTTP. Only the `maas-operator-leader` holder Watches. See [Scaling](#scaling-if-it-is-a-single-service).
- **Operator optional.** Helm `OPERATOR_ENABLED` enables or disables Watch + ProcessCR. See [Backward compatibility and downgrade](#backward-compatibility-and-downgrade).
- **deletionPolicy.** `Unregister` (default): delete the CR and the PG row. `Orphan`: delete the CR, keep the row. Only read while Terminating. See [Delete](#delete).
- **Finalizer.** `maas.netcracker.com/instance` after successful Register. Without it, `kubectl delete` drops the CR immediately and can leave an orphan PG row. See [Delete](#delete).
- **Create vs update.** Register vs Update from `GetById`, not from Watch ADDED vs MODIFIED. How name maps to PG `id`: [Name and id in the database](#name-and-id-in-the-database).
- **Status.** `Ready` + `Stalled` only. `phase` is for `kubectl`. Automate on conditions.
- **Events.** Optional Kubernetes Events on the instance CR (`K8S_EVENTS_ENABLED`). Same reasons as status. See [Kubernetes Events](#kubernetes-events).
- **No SecretRef on the service model.** Mapper loads CR + Secrets into existing `model.KafkaInstance` / `RabbitInstance`. InstanceService and the manager REST body stay resolved credentials, not Secret names.
- **managed_by_operator.** PG boolean. `true` after the operator Register/adopt. Existing REST rows stay `false` until a CR writes that id. Manager REST may Update only while this is `false`; after `true`, REST of that id is rejected. See [CR vs REST](#cr-vs-rest-and-optional-takeover).
- **Default.** MaaS Application `DEFAULT_KAFKA_INSTANCE` / `DEFAULT_RABBIT_INSTANCE` (CR `metadata.name`, first install = broker namespace). Empty: first Register is default. Named CR becomes default when it appears. See [Default instance](#default-instance).

### Backward compatibility and downgrade

Manager REST stays. Instances can still be Register/Update/Unregister’d when the operator is off. Existing PG rows are unchanged until a CR adopts them (`managed_by_operator`).

`OPERATOR_ENABLED` turns Watch + ProcessCR on or off. Flipping it false must **not** Unregister instances. Only CR deletion with `deletionPolicy: Unregister` does that.

**Downgrade to a MaaS version without the operator.** Argo CD Sync of an older Application (no Watch, no ProcessCR, no instance CRDs in that chart).

- PG instance rows stay. Topics/vhosts keep using them. Do not Unregister on rollback.
- Manager REST is the only writer again. The old binary does not read `managed_by_operator`; the column (if the schema stays) defaults `false` so the old process still starts. REST Update of those ids works (no lock in that binary).
- Instance CRs are not reconciled. If the old chart does not ship those CRDs and Argo drops them, the CRs leave the API; PG is unchanged. If the CRDs stay, the CRs sit unused. A CR with a finalizer cannot be deleted until the finalizer is removed or the CRD is deleted.
- `DEFAULT_KAFKA_INSTANCE` / `DEFAULT_RABBIT_INSTANCE` are ignored (not on the old chart). PG default flags stay.

### Secret access (namespaced)

The ClusterRole is for cluster watch of instance CRs only (`get` / `list` / `watch` / status PATCH / finalizers). It does **not** include `secrets`.

Each namespace that holds a `KafkaInstance` or `RabbitInstance` (and their `*SecretRef` Secrets) needs a Role + RoleBinding on the operator SA: `get` / `watch` of Secrets in that namespace. Same NS as the CR in v1 (`secretRef.namespace` is out of scope).

### Restricted environment

Use when the MaaS Application cannot create cluster-scoped objects. Watch stays cluster-wide. This is **not** “watch only the MaaS namespace” (rejected — broker CRs live in `kafka-infra` / `rabbit-infra`).

Default (`restrictedEnvironment: false`): the chart creates CRDs, `ClusterRole` / `ClusterRoleBinding` (instance CRs cluster-wide: `get` / `list` / `watch`, status PATCH, finalizers; **no `secrets`**; `events` `create` / `patch` when `K8S_EVENTS_ENABLED`), plus namespaced `ServiceAccount`, `Role` / `RoleBinding` for `Lease` `maas-operator-leader` in `CLOUD_NAMESPACE`.

`restrictedEnvironment: true`: the chart creates only the namespaced objects. Apply CRDs, `ClusterRole`, and `ClusterRoleBinding` out of band (cluster-admin) **before** the MaaS Application Syncs. Per-namespace Secret Roles stay as [Secret access](#secret-access-namespaced). Without the cluster objects, informers fail and instance kinds are unknown.

MaaS Application value:

```yaml
restrictedEnvironment: true    # default false
```

### Reconcile

Leader only. Claimed CRs (`operatorNamespace == CLOUD_NAMESPACE`). Skip apply if spec and Secrets unchanged, `Ready=True`, and not a resync.

**When**

| Trigger | How |
|---------|-----|
| CR create / spec edit / Terminating | Informer Watch `KafkaInstance` / `RabbitInstance` |
| Secret data change | Informer Watch referenced Secrets; enqueue CRs whose `*SecretRef.name` matches. Does not bump `generation`. |
| Periodic resync | `MAAS_INSTANCE_RESYNC_INTERVAL`, default `10m`. Re-read Secrets, refresh `status.isDefault`, retry InUse / SecretError. |
| `RequeueAfter 30s` | Unregister `InstanceInUse` (or still default). In-process timer, no apiserver PATCH. [Delete with existing topics](#delete-with-existing-topics-or-vhosts). |
| Error return | Transient. Workqueue rate limiter (below). |

**Backoff:**

1. **Error → exponential workqueue.** `SecretError`, `HealthCheckFailed`, apiserver/network. Base `1s`, doubles, cap `5m`, 10% jitter. Reset on success. `MAAS_RECONCILE_BACKOFF_BASE` / `MAAS_RECONCILE_BACKOFF_MAX` (defaults `1s` / `5m`).
2. **`RequeueAfter`, no error → limiter skipped.** `InstanceInUse` 30s. Resync 10m. `Stalled=True` (`InvalidSpec`, `DuplicateInstanceName`): `Result{}`, wait for the next Watch.

Health-check is sync Register/Update. No async poll (no 202 / trackingId).

---

## Prerequisites and Installation

**Prerequisites**

- Kubernetes 1.32 or newer — the CRDs rely on CEL validation rules (`x-kubernetes-validations`), and the operator-assignment cache filter uses CRD **selectable fields** on `spec.operatorNamespace`, which are GA in 1.32. On an older server the operator's informers fail to sync at startup.

**Installation**

Argo CD installs MaaS (chart in the MaaS Application). Watch + ProcessCR is off unless enabled.

MaaS Application values (first install):

```yaml
OPERATOR_ENABLED: true
DEFAULT_KAFKA_INSTANCE: <broker-namespace>    # optional
DEFAULT_RABBIT_INSTANCE: <broker-namespace>   # optional
K8S_EVENTS_ENABLED: true                     # false: no Events, omit events from ClusterRole
restrictedEnvironment: false                 # true: chart skips CRDs and ClusterRole; apply them out of band
```

- `OPERATOR_ENABLED: true` — with the default `false` the chart does not start Watch + ProcessCR (no Lease). Manager REST still runs.
- `DEFAULT_KAFKA_INSTANCE` / `DEFAULT_RABBIT_INSTANCE` — optional. CR `metadata.name` to make the MaaS default for that kind. On first install set each to the broker **namespace** (same string as the instance CR name). Empty: first registered instance of that kind becomes default. Names unknown yet: omit, then update the MaaS Application and Sync. See [Default instance](#default-instance).
- `K8S_EVENTS_ENABLED: true` — [Kubernetes Events](#kubernetes-events). Default on. `false` is a no-op recorder and drops `events` from the ClusterRole.
- `restrictedEnvironment: true` — [Restricted environment](#restricted-environment). Cluster-admin applies CRDs + ClusterRole first.

Install **MaaS first**, then Kafka/Rabbit instance CRs. Brokers may already be running; MaaS does not proxy traffic. Who becomes the PG default is [Default instance](#default-instance).

How **Kubernetes** and **Argo CD** treat the two install orders. Argo **Sync** is apply to the API. **Health** is separate: `Ready=False` is Degraded; no status yet is Progressing. A wave that waits for Healthy blocks.

### MaaS first, then instance CRs

MaaS chart installs CRDs and `maas-service`. Leader Watches an empty list. A later wave / other Application applies `KafkaInstance` / `RabbitInstance`.

Kubernetes: kinds exist, apply succeeds, ProcessCR runs on each create. Argo: MaaS Application Healthy (Deployment). Instance Application Sync green, Health Progressing until ProcessCR PATCHes `Ready`, then Healthy. Topic APIs have no default until that first Register — same as today. Named default: [Default instance](#default-instance).

```mermaid
sequenceDiagram
  participant Argo
  participant API as kube_apiserver
  participant Pod as maas-service
  Argo->>API: apply MaaS CRDs
  Argo->>Pod: deploy maas-service
  Pod->>API: leader Watch empty
  Note over Argo: later wave or other app
  Argo->>API: apply instance CRs
  API->>Pod: Watch create
  Pod->>Pod: ProcessCR
  Pod->>API: PATCH Ready
```

### Instance CRs first (otherwise)

**Bad order.** Kafka/Rabbit (or the instance Application) can already be deployed, then the CR still fails. Brokers do not need MaaS to run; the GitOps app for the CR does.

**No MaaS, no CRDs.** Apiserver rejects the kind. Argo Sync of the instance Application **fails** after the broker install is already green.

**CRDs exist, operator not watching yet.** Apply writes CRs to etcd. Sync is green. Health stays Progressing, then can go **Degraded** when ProcessCR later fails (`HealthCheckFailed`, `SecretError`). The instance is already out; the CR is the thing that looks failed.

```mermaid
sequenceDiagram
  participant Argo
  participant API as kube_apiserver
  participant Pod as maas-service
  Note over Argo: brokers or instance app already deployed
  Argo->>API: apply KafkaInstance
  alt no CRDs
    API-->>Argo: Sync fail unknown kind
  else CRDs exist operator not ready
    Note over API: CRs in etcd Sync green
    Argo->>Pod: later leader Watch
    Pod->>Pod: ProcessCR
    Pod->>API: PATCH Ready or Ready=False
    Note over Argo: Health Degraded after instances are already up
  end
```

---

## CRD sketch

Two namespaced kinds, group `maas.netcracker.com/v1`: `KafkaInstance`, `RabbitInstance` (any namespace). Claim: required immutable `spec.operatorNamespace == CLOUD_NAMESPACE`. Credentials live in Secrets in the **same** namespace as the instance CR (no `secretRef.namespace` in v1). CRD extras: category `maas`, short names, printer columns, `selectableFields` on `spec.operatorNamespace` (K8s 1.32+), CEL `self == oldSelf` on identity fields.

| Kind | Short name | `kubectl get` columns |
|------|------------|------------------------|
| `KafkaInstance` | `mkafi` | `PHASE`, `READY`, `DEFAULT`, `AGE` |
| `RabbitInstance` | `mrabi` | `PHASE`, `READY`, `DEFAULT`, `AGE` |

### KafkaInstance

#### CR example

What ProcessCR sees after Get on a registered Kafka CR (apiserver-filled metadata included). Inline comments on spec/status are documentation, not YAML schema.

```yaml
apiVersion: maas.netcracker.com/v1
kind: KafkaInstance
metadata:
  name: platform-kafka
  namespace: kafka-infra
  uid: 7f3c9a10-4b2e-4d11-9c0a-0b1a2c3d4e5f
  resourceVersion: "184400"
  generation: 3
  finalizers:
    - maas.netcracker.com/instance
spec:
  operatorNamespace: maas-core         # required. Claim: must equal this operator CLOUD_NAMESPACE. CEL immutable (self == oldSelf). RFC-1123
  addresses:
    SASL_PLAINTEXT: ["kafka.kafka-infra:9092"]
  maasProtocol: SASL_PLAINTEXT         # PLAINTEXT | SASL_PLAINTEXT | SSL | SASL_SSL. Protocol MaaS uses to talk to the broker
  caCertSecretRef:
    name: kafka-ca
    key: ca.crt
  credentialsSecretRef:
    name: kafka-admin
    keys:
      - key: type
        name: type
      - key: username
        name: username
      - key: password
        name: password
  deletionPolicy: Unregister           # Unregister (default): delete CR and PG row. Orphan: delete CR, keep PG row. Only read while Terminating
status:
  phase: BackingOff                    # kubectl column only. Processing | Succeeded | BackingOff | InvalidConfiguration. Automate on conditions, not phase
  isDefault: false                     # observed PG default. Not a request
  lastRequestId: "req-3f8c"            # ProcessCR generates X-Request-Id and PATCHes it (no HTTP header)
  secretRevisions:                     # Secret resourceVersions. Never secret bytes
    kafka-ca: "184291"
    kafka-admin: "184310"
  conditions:                          # Ready + Stalled. list-type=map keyed by type
    - type: Ready                      # True = this generation applied successfully and instance is usable (or Terminating cleanup done)
      status: "False"
      reason: HealthCheckFailed        # see reasons below
      message: "kafka health-check failed: dial tcp kafka.kafka-infra:9092: i/o timeout"
      lastTransitionTime: "2026-08-27T13:40:02Z"
      observedGeneration: 3
    - type: Stalled                    # True = permanent spec error, do not retry until spec changes. False = success or transient retry
      status: "False"
      reason: HealthCheckFailed
      message: "kafka health-check failed: dial tcp kafka.kafka-infra:9092: i/o timeout"
      lastTransitionTime: "2026-08-27T13:40:02Z"
      observedGeneration: 3
  observedGeneration: 3                # stamped on success or Stalled=True. Left behind on transient (BackingOff)
```

#### Resource Fields

| Field | Required | Notes |
|-------|:--------:|-------|
| `metadata.name` | Yes | DNS-1123. First install: equal to `metadata.namespace`. `DEFAULT_KAFKA_INSTANCE` matches this. Maps to PG `id`: [Name and id](#name-and-id-in-the-database). |
| `spec.operatorNamespace` | Yes | Claim (`CLOUD_NAMESPACE`). CEL immutable. Skip, no PATCH, if it does not match. |
| `spec.addresses` | Yes | One protocol key, matching `maasProtocol`. |
| `spec.maasProtocol` | Yes | `PLAINTEXT` \| `SASL_PLAINTEXT` \| `SSL` \| `SASL_SSL`. |
| `spec.caCertSecretRef` | No | `{name, key}` same NS. Omit if no CA. |
| `spec.credentialsSecretRef` | Yes | `{name, keys[{key, name}]}` same NS. `key` = `Secret.data`; `name` = Kafka Auth DTO field. |
| `spec.deletionPolicy` | No | `Unregister` (default) or `Orphan`. Read only while Terminating. |

Status: [Status](#status).

#### CRD

```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: kafkainstances.maas.netcracker.com
spec:
  group: maas.netcracker.com
  names:
    categories: [maas]
    kind: KafkaInstance
    listKind: KafkaInstanceList
    plural: kafkainstances
    shortNames: [mkafi]
    singular: kafkainstance
  scope: Namespaced
  versions:
    - name: v1
      served: true
      storage: true
      additionalPrinterColumns:
        - jsonPath: .status.phase
          name: Phase
          type: string
        - jsonPath: .status.conditions[?(@.type=='Ready')].status
          name: Ready
          type: string
        - jsonPath: .status.isDefault
          name: Default
          type: boolean
        - jsonPath: .metadata.creationTimestamp
          name: Age
          type: date
      selectableFields:
        - jsonPath: .spec.operatorNamespace
      subresources:
        status: {}
      schema:
        openAPIV3Schema:
          type: object
          required: [spec]
          properties:
            spec:
              type: object
              required: [operatorNamespace, addresses, maasProtocol, credentialsSecretRef]
              properties:
                operatorNamespace:
                  type: string
                  description: MaaS install that claims this CR. Must equal the operator CLOUD_NAMESPACE. Immutable.
                  minLength: 1
                  maxLength: 63
                  pattern: '^[a-z0-9]([-a-z0-9]*[a-z0-9])?$'
                  x-kubernetes-validations:
                    - rule: self == oldSelf
                      message: spec.operatorNamespace is immutable after creation
                addresses:
                  type: object
                  description: Broker addresses keyed by protocol. One key, matching maasProtocol.
                  additionalProperties:
                    type: array
                    items:
                      type: string
                      minLength: 1
                  minProperties: 1
                maasProtocol:
                  type: string
                  description: Protocol MaaS uses to talk to this Kafka.
                  enum: [PLAINTEXT, SASL_PLAINTEXT, SSL, SASL_SSL]
                caCertSecretRef:
                  type: object
                  description: CA certificate Secret in the same namespace as this CR. Omit if no CA.
                  required: [name, key]
                  properties:
                    name:
                      type: string
                      minLength: 1
                    key:
                      type: string
                      minLength: 1
                credentialsSecretRef:
                  type: object
                  description: Credentials Secret in the same namespace. keys map Secret.data to Kafka Auth DTO fields.
                  required: [name, keys]
                  properties:
                    name:
                      type: string
                      minLength: 1
                    keys:
                      type: array
                      minItems: 1
                      items:
                        type: object
                        required: [key, name]
                        properties:
                          key:
                            type: string
                            minLength: 1
                            description: Key in Secret.data
                          name:
                            type: string
                            minLength: 1
                            description: Field name in the Kafka Auth DTO (type, username, password, clientKey, clientCert)
                deletionPolicy:
                  type: string
                  description: On CR delete. Unregister drops the PG row; Orphan keeps it. Only read while Terminating.
                  enum: [Unregister, Orphan]
                  default: Unregister
            status:
              type: object
              properties:
                phase:
                  type: string
                  description: kubectl summary. Automate on conditions, not phase.
                isDefault:
                  type: boolean
                  description: Observed PG default flag.
                lastRequestId:
                  type: string
                secretRevisions:
                  type: object
                  additionalProperties:
                    type: string
                observedGeneration:
                  type: integer
                  format: int64
                conditions:
                  type: array
                  x-kubernetes-list-type: map
                  x-kubernetes-list-map-keys: [type]
                  items:
                    type: object
                    required: [lastTransitionTime, message, reason, status, type]
                    properties:
                      type:
                        type: string
                      status:
                        type: string
                        enum: ["True", "False", "Unknown"]
                      reason:
                        type: string
                        minLength: 1
                      message:
                        type: string
                      lastTransitionTime:
                        type: string
                        format: date-time
                      observedGeneration:
                        type: integer
                        format: int64
```

### RabbitInstance

#### CR example

```yaml
apiVersion: maas.netcracker.com/v1
kind: RabbitInstance
metadata:
  name: platform-rabbit
  namespace: rabbit-infra
  uid: 1a2b3c4d-5e6f-7081-92a3-b4c5d6e7f809
  resourceVersion: "22010"
  generation: 1
  finalizers:
    - maas.netcracker.com/instance
spec:
  operatorNamespace: maas-core         # required, immutable, same claim as Kafka
  apiUrl: http://rabbit.rabbit-infra:15672/api   # Rabbit management API
  amqpUrl: amqp://rabbit.rabbit-infra:5672       # AMQP
  credentialsSecretRef:
    name: rabbit-admin
    keys:
      - key: user
        name: user
      - key: password
        name: password
  deletionPolicy: Unregister
status:
  phase: Succeeded
  isDefault: false
  lastRequestId: "req-22010"
  secretRevisions:
    rabbit-admin: "22001"
  conditions:
    - type: Ready
      status: "True"
      reason: InstanceRegistered
      lastTransitionTime: "2026-08-27T13:41:02Z"
      observedGeneration: 1
    - type: Stalled
      status: "False"
      reason: Succeeded
      lastTransitionTime: "2026-08-27T13:41:02Z"
      observedGeneration: 1
  observedGeneration: 1
```

#### Resource Fields

| Field | Required | Notes |
|-------|:--------:|-------|
| `metadata.name` | Yes | Same rules as Kafka. `DEFAULT_RABBIT_INSTANCE` matches this. |
| `spec.operatorNamespace` | Yes | Same claim as Kafka. CEL immutable. |
| `spec.apiUrl` | Yes | Rabbit management HTTP API. |
| `spec.amqpUrl` | Yes | AMQP. |
| `spec.credentialsSecretRef` | Yes | `{name, keys[{key, name}]}` same NS. DTO fields: `user`, `password`. |
| `spec.deletionPolicy` | No | `Unregister` (default) or `Orphan`. |

Status: [Status](#status).

#### CRD

```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: rabbitinstances.maas.netcracker.com
spec:
  group: maas.netcracker.com
  names:
    categories: [maas]
    kind: RabbitInstance
    listKind: RabbitInstanceList
    plural: rabbitinstances
    shortNames: [mrabi]
    singular: rabbitinstance
  scope: Namespaced
  versions:
    - name: v1
      served: true
      storage: true
      additionalPrinterColumns:
        - jsonPath: .status.phase
          name: Phase
          type: string
        - jsonPath: .status.conditions[?(@.type=='Ready')].status
          name: Ready
          type: string
        - jsonPath: .status.isDefault
          name: Default
          type: boolean
        - jsonPath: .metadata.creationTimestamp
          name: Age
          type: date
      selectableFields:
        - jsonPath: .spec.operatorNamespace
      subresources:
        status: {}
      schema:
        openAPIV3Schema:
          type: object
          required: [spec]
          properties:
            spec:
              type: object
              required: [operatorNamespace, apiUrl, amqpUrl, credentialsSecretRef]
              properties:
                operatorNamespace:
                  type: string
                  description: MaaS install that claims this CR. Must equal the operator CLOUD_NAMESPACE. Immutable.
                  minLength: 1
                  maxLength: 63
                  pattern: '^[a-z0-9]([-a-z0-9]*[a-z0-9])?$'
                  x-kubernetes-validations:
                    - rule: self == oldSelf
                      message: spec.operatorNamespace is immutable after creation
                apiUrl:
                  type: string
                  minLength: 1
                  description: Rabbit management HTTP API URL
                amqpUrl:
                  type: string
                  minLength: 1
                  description: AMQP URL
                credentialsSecretRef:
                  type: object
                  required: [name, keys]
                  properties:
                    name:
                      type: string
                      minLength: 1
                    keys:
                      type: array
                      minItems: 1
                      items:
                        type: object
                        required: [key, name]
                        properties:
                          key:
                            type: string
                            minLength: 1
                          name:
                            type: string
                            minLength: 1
                            description: Field name in the Rabbit payload (user, password)
                deletionPolicy:
                  type: string
                  enum: [Unregister, Orphan]
                  default: Unregister
            status:
              type: object
              properties:
                phase:
                  type: string
                isDefault:
                  type: boolean
                lastRequestId:
                  type: string
                secretRevisions:
                  type: object
                  additionalProperties:
                    type: string
                observedGeneration:
                  type: integer
                  format: int64
                conditions:
                  type: array
                  x-kubernetes-list-type: map
                  x-kubernetes-list-map-keys: [type]
                  items:
                    type: object
                    required: [lastTransitionTime, message, reason, status, type]
                    properties:
                      type:
                        type: string
                      status:
                        type: string
                        enum: ["True", "False", "Unknown"]
                      reason:
                        type: string
                        minLength: 1
                      message:
                        type: string
                      lastTransitionTime:
                        type: string
                        format: date-time
                      observedGeneration:
                        type: integer
                        format: int64
```

### Secrets

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: kafka-ca
  namespace: kafka-infra
type: Opaque
stringData:
  ca.crt: |
    -----BEGIN CERTIFICATE-----
    ...
    -----END CERTIFICATE-----
---
apiVersion: v1
kind: Secret
metadata:
  name: kafka-admin
  namespace: kafka-infra
type: Opaque
stringData:
  type: plain                     # plain | sslCert | SCRAM | sslCert+plain | sslCert+SCRAM
  username: admin
  password: "..."
  # clientKey / clientCert when type is sslCert or sslCert+plain / sslCert+SCRAM:
  # clientKey: |
  #   -----BEGIN PRIVATE KEY-----
  # clientCert: |
  #   -----BEGIN CERTIFICATE-----
---
apiVersion: v1
kind: Secret
metadata:
  name: rabbit-admin
  namespace: rabbit-infra
type: Opaque
stringData:
  user: guest
  password: "..."
```


### Status

Shared by `KafkaInstance` and `RabbitInstance`. ProcessCR PATCHes it. Do not put these on spec.

| Field | Notes |
|-------|-------|
| `status.phase` | `Processing` \| `Succeeded` \| `BackingOff` \| `InvalidConfiguration`. kubectl only. |
| `status.conditions` | `Ready` + `Stalled` only. |
| `status.lastRequestId` | ProcessCR `X-Request-Id` (no HTTP header). |
| `status.observedGeneration` | Stamped on success or `Stalled=True`. Left behind on transient. |
| `status.isDefault` | Observed PG default. [Default instance](#default-instance). |
| `status.secretRevisions` | Secret `resourceVersion`s. Never secret bytes. |

| Ready / Stalled | Meaning |
|-----------------|---------|
| `Ready=True`, `Stalled=False` | Applied. Retry not needed. |
| `Ready=False`, `Stalled=False` | Transient. Retry. |
| `Ready=False`, `Stalled=True` | Permanent. Wait for spec change. |

| Reason | Ready | Stalled | Meaning |
|--------|-------|---------|---------|
| `InstanceRegistered` | True | False | Register/Update succeeded. |
| `ForcedDefault` | True | False | First row became PG default. Informational. |
| `SecretError` | False | False | Secret missing, key missing/empty, or forbidden. |
| `HealthCheckFailed` | False | False | Register/Update health-check 400. Do not Unregister a previous good row. |
| `InstanceInUse` | False | False | Unregister 400, topics/vhosts still on the instance. Keep finalizer. |
| `InvalidSpec` | False | True | Duplicate `keys[].name`, bad `maasProtocol`, etc. |
| `DuplicateInstanceName` | False | True | Another CR owns that id. |

Finalizer `maas.netcracker.com/instance` after Register. `deletionTimestamp` set means Terminating. Secret Watch does not bump `generation`.

---

## Kubernetes Events

ProcessCR may emit a Kubernetes Event on the instance CR (`involvedObject` = that CR). The Event lives in the **CR namespace** (`kafka-infra` / `rabbit-infra`), not in `CLOUD_NAMESPACE`. Same `reason` strings as [Status](#status).

`K8S_EVENTS_ENABLED` (default `true`). `false`: recorder is a no-op; chart omits `events` `create` / `patch` from the ClusterRole (and from the out-of-band ClusterRole when `restrictedEnvironment: true`).

**Emit** when ProcessCR first writes that reason (or the reason changes). **Do not emit** on skip-apply, skip-claim, 10m resync that stays `Ready=True`, or a retry that already has the same reason (`InstanceInUse` every 30s). `ForcedDefault` stays status-only.

| Reason | Type | When |
|--------|------|------|
| `InstanceRegistered` | Normal | Register/Update succeeded. |
| `InvalidSpec` | Warning | Duplicate `keys[].name`, bad `maasProtocol`, etc. |
| `SecretError` | Warning | Secret missing, key missing/empty, or forbidden. |
| `HealthCheckFailed` | Warning | Register/Update health-check 400. |
| `InstanceInUse` | Warning | Unregister 400; topics/vhosts still on the instance. Once until the reason changes. |
| `DuplicateInstanceName` | Warning | Another CR owns that id. |

ClusterRole (not the namespaced Secret Role) grants `events` `create` / `patch` so Events can land in the broker namespace. `kubectl describe kafkainstance … -n kafka-infra` shows them.

---

## Mapping layer (CR vs service structs)

`*SecretRef` and `operatorNamespace` do **not** belong on `[model.KafkaInstance](../maas/maas-service/model/kafka_model.go)` / `[RabbitInstance](../maas/maas-service/model/rabbit_model.go)`. Those structs are the REST body and the PostgreSQL row: they store **resolved** `caCert`, `credentials` / `user`+`password`, not pointers to Secrets. Putting refs there would change the public manager API and persist names instead of secrets.

Add a **new operator layer** (not a new DB table):

- CR Go types (`KafkaInstance` / `RabbitInstance`, `SecretKeyMapping`, `SecretKeyRef`) live in the operator package. They are the apiserver schema. Kind names match the service models (`model.KafkaInstance` / `model.RabbitInstance`); the packages differ so the types do not collide.
- Mapper: load CR + Secrets → fill existing `model.KafkaInstance` / `RabbitInstance` (`Id` mapping open — [Name and id](#name-and-id-in-the-database), `Addresses`, `Default: false`, `MaasProtocol`, `CACert`, `Credentials` / `ApiUrl`, `AmqpUrl`, `User`, `Password`). Instance mapper never sets `Default: true`.
- Apply still calls `KafkaInstanceService` / `RabbitInstanceService` with that model. No SecretRef in the service. `SetDefault` is driven by `DEFAULT_KAFKA_INSTANCE` / `DEFAULT_RABBIT_INSTANCE` after Register/Update ([Default instance](#default-instance)).

Optional DB columns on the instance row (`managed_by_operator`, `namespace`, `origin_cr`) are schema extras, not a second instance struct. Existing rows: `managed_by_operator = false`, `namespace` empty until a CR writes it.

```mermaid
flowchart LR
  cr[KafkaInstance spec refs]
  secrets[Secret objects]
  mapper[operator mapper]
  model[model.KafkaInstance]
  svc[KafkaInstanceService]
  cr --> mapper
  secrets --> mapper
  mapper --> model
  model --> svc
```



---

## Topology

### Single microservice (reconciler and MaaS logic in one process)

One Deployment, one process. The **leader** replica runs the watch loop in-process (see [Scaling](#scaling-if-it-is-a-single-service)). Followers run only the existing MaaS logic (Fiber). Sibling Deployment comparison: [operator_design_notes.md](operator_design_notes.md#scenario-a-vs-b-recommendation).

**Apply path: Go services, not HTTP.** ProcessCR is wired to the same `[KafkaInstanceService](../maas/maas-service/service/instance/kafka_instances_service.go)` / `[RabbitInstanceService](../maas/maas-service/service/instance/rabbit_instances_service.go)` as `[InstanceController](../maas/maas-service/controller/instance_controller.go)`. Do not POST localhost `/api/v2/...`.

```mermaid
flowchart TB
  subgraph pods [maas-service Deployment HPA N replicas]
    direction LR

    subgraph podA [LEADER]
      direction TB
      processA[ProcessCR]
      appsA[Apps REST]
      leaseA[Lease maas-operator-leader]
      crA[KafkaInstance / RabbitInstance]
      fiberA[Fiber REST]
      recA[Watch CR and Secrets]
      instA[InstanceService]
      leaseA --> processA
      crA --> recA
      recA --> processA
      processA -->|"in-process Register Update Unregister"| instA
      appsA --> fiberA --> instA
      processA -->|"PATCH status finalizer"| crA
    end

    subgraph podF [FOLLOWER]
      direction TB
      noteF[no Lease, no Watch, no ProcessCR]
      appsF[Apps REST]
      fiberF[Fiber REST]
      instF[InstanceService]
      appsF --> fiberF --> instF
    end
  end

  pgA[(PostgreSQL)]
  instA --> pgA
  instF --> pgA
```



ProcessCR lives **inside** the `maas-service` binary, next to Fiber. Only the Lease holder starts Watch + ProcessCR. Followers serve Fiber REST only — they do **not** forward Register to the leader. Apps hit any replica; CR apply is leader-only, in-process.

**Features**

- One Deployment, one process. HPA stays; do not pin `REPLICAS: 1`. The operator is a singleton *role* (Lease), not a singleton *pod*.
- Apply is in-process `InstanceService`. No localhost HTTP.
- `status.lastRequestId`: ProcessCR generates `X-Request-Id`, puts it on the Go context (InstanceService logs), PATCHes the CR. Fiber `ExtractOrAttachXRequestId` is not on this path.
- ClusterRole (cluster watch) lands on the HTTP SA.
- Watcher panic / client-go deadlock can take REST down.
- Already has `drMode`.
- Helm flag on the existing chart (`OPERATOR_ENABLED`).
- Start controller-runtime alongside Fiber in `[server.go](../maas/maas-service/server.go)`.

---

### Scaling if it is a single service

If the reconciler is embedded in `maas-service` (one Deployment, one process), **keep today’s HTTP scaling**. The operator is a singleton *role*, not a singleton *pod*. Do not pin `REPLICAS: 1` or disable HPA.

```mermaid
flowchart TB
  hpa[HPA CPU] --> deploy[maas-service Deployment N replicas]
  deploy --> podA[Pod A leader]
  deploy --> podB[Pod B follower]
  deploy --> podC[Pod C follower]
  lease[Lease maas-operator-leader] --> podA
  podA --> reconcile[Watch CRs and in-process Register/Update]
  podB --> httpOnly[Fiber REST only]
  podC --> httpOnly
```



Replicas do **not** talk to each other about who watches. Kubernetes is the source of truth: a namespaced `Lease` `maas-operator-leader`. At most one replica holds it; that replica is the watcher. Everyone else is HTTP-only until the lease changes. Mechanism (acquire, renew, expire, 409 on the Lease object, code samples): [Kubernetes Lease lock](operator_design_notes.md#kubernetes-lease-lock).

#### What `maas-operator-leader` is

- **Lease** `maas-operator-leader` — `coordination.k8s.io/v1` in `CLOUD_NAMESPACE`. This **is** the lock. `holderIdentity` = pod name.
- **Metric** `maas_operator_leader` — optional gauge (`1` = this process holds the Lease). Not a lock; other replicas must not use it to decide who watches.

```yaml
apiVersion: coordination.k8s.io/v1
kind: Lease
metadata:
  name: maas-operator-leader
  namespace: maas-core
spec:
  holderIdentity: maas-service-7f8c9d-xk2l
  leaseDurationSeconds: 15
```

`kubectl get lease maas-operator-leader -n maas-core` shows the watcher.

How replicas campaign, callbacks, followers, DR, two MaaS, RBAC: [operator_design_notes.md](operator_design_notes.md#how-replicas-know-who-is-watching).

- Leave `[REPLICAS](../helm-templates/maas-service/values.yaml)` and HPA as they are.
- Start `controller-runtime` in a goroutine from `[server.go](../maas/maas-service/server.go)` with **LeaderElection = true**. Inject the already-constructed `kafkaInstanceService` / `rabbitInstanceService` (same objects as the REST controllers).
- Size informer cache modestly (two CRDs + referenced Secrets). Do not raise HTTP CPU targets just for the operator.
- Keep Register/Update/Unregister idempotent (in-process Unregister of missing id = success).

Several replicas watching the same CRs is a bug (two writers on one PG registry). Cases: [Why several MaaS replicas watching the same CRs is a bug](operator_design_notes.md#why-several-maas-replicas-watching-the-same-crs-is-a-bug).

---

## Special cases

Defaults, name/id in PG, REST coexistence, delete/lifecycle, and multi-MaaS. ProcessCR rules above still apply.

### Default instance

MaaS allows **one** default Kafka instance and **one** default Rabbit instance per PostgreSQL. Topics/vhosts with no `instance` id use that default.

No `spec.default` on the instance CR and no `DefaultInstance` CR. Names come from the MaaS Argo CD Application: `DEFAULT_KAFKA_INSTANCE` / `DEFAULT_RABBIT_INSTANCE`. Install: [Prerequisites and Installation](#prerequisites-and-installation).

First insert into an empty DB still becomes default (`ForcedDefault` in [DAO](../maas/maas-service/service/instance/kafka_instances_dao.go)) when the install param for that kind is empty, or when the named CR is not registered yet. “First” is whichever Register commits.

**Install params.** Optional. Value = instance CR `metadata.name`. Kafka and Rabbit are independent. On first MaaS install set each to the broker **namespace** (new instance: `metadata.name` equals `metadata.namespace` — [Name and id](#name-and-id-in-the-database)). If the names are not known yet, install MaaS without them and update the MaaS Application afterwards (Argo CD Sync).

ProcessCR on a claimed instance CR (Watch, Argo CD Sync of MaaS after a param change, or 10m resync), Kafka and Rabbit separately:

1. **Param empty** for this kind. No default in PG → this Register is `ForcedDefault`. Default already in PG → leave it. PATCH `status.isDefault` from PG. Do not steal on every reconcile.
2. **Param set**, this CR `metadata.name` equals the param, row exists, not already default → `SetDefault` this id. Already default → PATCH status only (do not `SetDefault` again on every Watch / 10m resync).
3. **Param set**, this CR name does not match. No default in PG → `ForcedDefault` this instance (topics work until the named CR appears). Default already in PG → leave it.
4. **Param set**, named CR not in the cluster yet. MaaS stays Healthy. Do not fail Argo. Do not clear PG. When that CR Registers (or on the next reconcile after a later MaaS Application Sync), rule 2 runs.
5. **Names unknown at first MaaS install.** Omit the params. First Register is `ForcedDefault`. Later update the MaaS Application with the names and Sync; the Deployment rolls, ProcessCR sees the new env, rule 2 `SetDefault`s the named CR (may switch the ForcedDefault winner).
6. Clearing a param later does **not** unset the PG default. Do not send Update `default: false` on the current default (DAO 400).

`status.isDefault` is observed PG state, not a request. PATCH it on the CRs this reconcile touches (and on the previous default after a switch).

Manager REST steal is unchanged. This naming rule is operator-only.

**Deploy order.** If MaaS does not exist, CRs cannot exist ([Prerequisites and Installation](#prerequisites-and-installation)). The `ForcedDefault` “first informer item” case is only after CRDs exist and the leader Lists several CRs at once. First Register into empty PG wins. If the install param names another CR already in that List, rule 2 then `SetDefault`s it.

If the operator is already watching and CRs are applied one by one, the first Register is ForcedDefault unless that CR already matches the param.

A REST instance that is already default is not moved until the install param names a registered CR.

```mermaid
flowchart TD
  ev[Watch, Argo Sync of MaaS, or 10m resync]
  ev --> param{"DEFAULT_*_INSTANCE set?"}
  param -->|empty| empty{"PG has a default?"}
  empty -->|no| force["ForcedDefault this instance"]
  empty -->|yes| keep["Leave PG default"]
  param -->|set| match{"this CR metadata.name equals param?"}
  match -->|yes, row exists, not default| set["SetDefault this id"]
  match -->|yes, already default| patchOnly["PATCH status only"]
  match -->|no, no PG default| force
  match -->|no, PG has default| keep
  set --> patch["PATCH status.isDefault"]
  force --> patch
  keep --> patch
  patchOnly --> patch
```

```mermaid
sequenceDiagram
  participant Argo
  participant Pod as maas-service
  participant Named as KafkaInstance kafka-infra
  participant Other as KafkaInstance other
  participant DB as PostgreSQL

  Note over Argo,DB: 1 first install names known
  Argo->>Pod: Sync MaaS OPERATOR_ENABLED DEFAULT_KAFKA_INSTANCE=kafka-infra
  Argo->>Named: apply
  Named->>Pod: ProcessCR
  Pod->>DB: Register ForcedDefault kafka-infra
  Note over DB: already the named default

  Note over Argo,DB: 2 names unknown then update MaaS Application
  Argo->>Pod: Sync MaaS OPERATOR_ENABLED params empty
  Argo->>Other: apply
  Other->>Pod: ProcessCR
  Pod->>DB: Register ForcedDefault other
  Argo->>Pod: Sync MaaS DEFAULT_KAFKA_INSTANCE=kafka-infra
  Argo->>Named: apply
  Named->>Pod: ProcessCR
  Pod->>DB: SetDefault kafka-infra
  Note over DB: switched

  Note over Argo,DB: 3 param set CR not yet there
  Argo->>Pod: Sync MaaS DEFAULT_KAFKA_INSTANCE=kafka-infra
  Note over Pod: Healthy wait
  Argo->>Other: apply first
  Other->>Pod: ProcessCR ForcedDefault
  Argo->>Named: apply later
  Named->>Pod: ProcessCR SetDefault kafka-infra
```

---

### Name and id in the database

REST instance `id` is a free-form string (Helm name, UUID, …). The CR has `metadata.name` and `metadata.namespace`. Those need not match the existing PG `id`. Topics, vhosts, and designators FK that id — it cannot be renamed. Two CRs (or a CR and a REST row) can collide on name, namespace, or Kafka `addresses` (jsonb unique). Rabbit has no URL unique. A CR in another namespace must not rewrite an instance already bound to one NS.

Always use both `metadata.name` and `metadata.namespace`. Do not treat “put the old REST id as the CR name” as the main path — people have several instances or do not know the previous id.

**New instance.** PG `id` = CR `metadata.namespace`. One Kafka and one Rabbit per namespace. For a new broker, set `metadata.name` equal to the namespace. Insert a new row (`managed_by_operator` true on that row). An old REST row with some other id is left alone — no adopt, no `managed_by_operator` switch on that old id.

If the CR is new (no row for that namespace) but Kafka `addresses` already belong to another id → unique error (`23505`). They forgot the old id and must use the migrate path below. Rabbit has no URL unique today.

**Migrate when the old id already equals the namespace.** CR in ns `X`, REST row `id = X`. `GetById(namespace)` hits. Update that row (topics/vhosts keep the short id — do not rename). No name-as-old-id; no switch to a different id.

**Migrate when the old id is not the namespace.** `metadata.name` = old REST id. `metadata.namespace` is stored on the row (new `namespace` column). First CR: Update, set `managed_by_operator` true, write that namespace. Second CR with the same name from another NS: if `managed_by_operator` and stored namespace ≠ this CR namespace → error. Do not allow changing an instance from another namespace.

```mermaid
flowchart TD
  cr["CR metadata.name + metadata.namespace"]
  cr --> byNs{"GetById namespace"}
  byNs -->|hit| sameNs["Update / adopt that row. id stays"]
  byNs -->|miss| byName{"name != ns AND GetById name?"}
  byName -->|hit, not managed_by_operator| migr["Adopt: set managed_by_operator, store CR namespace"]
  byName -->|"hit, managed_by_operator, stored ns != this ns"| deny["Ready=False. Do not Update"]
  byName -->|"hit, managed_by_operator, same ns"| upd["Update"]
  byName -->|miss| reg["Register id = namespace"]
  reg -->|Kafka addresses already used| uniq["unique error. forgot old id"]
```

Do not rename an existing PG `id`. Topics, vhosts, and designators FK that id.

---

### CR vs REST (and optional `takeOver`)

Manager REST already inserts rows into PostgreSQL. How the CR finds that row is open: [Name and id in the database](#name-and-id-in-the-database). Register of an existing id is **400 unique**, not merge. The first matching CR **Updates** the row (adopt): copy spec+Secrets, set `managed_by_operator = true`. Topics/vhosts stay. Applying the CR **is** the migrate. There is no `spec.takeOver` in v1.

**One owner.** `managed_by_operator` (existing rows `false`):

- `false` — REST row, never written by a CR. Manager REST may still Update that id.
- `true` — a CR already Register’d or adopted it. REST Update/Unregister of that id is **rejected**. Later CR reconciles are normal Updates.
- Two CRs for one name: second `DuplicateInstanceName`. Default switch is [Default instance](#default-instance), not a flag on those CRs.

TODO: discuss migrate back — `deletionPolicy: Orphan` (CR gone, row stays, set `managed_by_operator` false) or REST-only after the operator is disabled — not two writers on a live CR.

```mermaid
sequenceDiagram
  participant CR as KafkaInstance
  participant REST as Manager REST
  participant DB as PostgreSQL
  Note over DB: row platform-kafka from REST managed_by_operator false
  REST->>DB: Update allowed
  CR->>DB: Update adopt set managed_by_operator true
  REST->>DB: Update rejected
  CR->>DB: later Updates allowed
```



**Optional later: spec.takeOver.** v1 auto-adopt means a CR whose `metadata.name` matches a REST row overwrites addresses/credentials, then locks REST out. If we later want **not** to migrate by default, and require explicit consent:

- Default `takeOver: false`: existing `managed_by_operator` false → `Ready=False` `InstanceOwnedByOtherSource`. Do not Update. Ops keeps REST for that name.
- `takeOver: true`: same adopt as v1 (Update, set the flag). Only the first adopt cares; later reconciles are normal Updates.
- Not “steal default from another CR” and not “two CRs may share one name.” Default is [Default instance](#default-instance).

Until that flag exists, do not add `InstanceOwnedByOtherSource` on the CR.

---

### CR lifecycle (watcher, create vs update vs delete, delete with existing topics or vhosts)

Kubernetes Watch is level-triggered: every event is “reconcile this key”, not a typed create/update. **ProcessCR must choose Register vs Update** by reading MaaS state (`GetById`), not by ADDED vs MODIFIED.

#### Watcher, create vs update vs delete

The picture below is **four different times**, not one call that Registers then Updates twice.

##### Four times: Register, Secret Update, spec Update, Unregister

```mermaid
sequenceDiagram
  participant Pod as operator_or_leader
  participant API as kube_apiserver
  participant CR as KafkaInstance
  participant Sec as Secrets
  participant Svc as InstanceService
  participant DB as PostgreSQL

  Note over Pod: time 0 start
  Pod->>API: campaign Lease maas-operator-leader
  API-->>Pod: OnStartedLeading
  Pod->>API: start informers Watch CR and Secrets

  Note over Pod,DB: time 1 new CR no MaaS row
  CR->>API: create CR
  API->>Pod: enqueue
  Pod->>CR: Get CR
  Pod->>Sec: Get SecretRefs
  Pod->>Svc: GetById miss
  Pod->>Svc: Register
  Svc->>DB: insert
  Pod->>CR: add finalizer
  Pod->>CR: PATCH status Registered

  Note over Pod,DB: time 2 Secret rotation row already exists
  Sec->>API: Secret data change
  API->>Pod: enqueue CRs that ref this Secret
  Pod->>Svc: GetById hit
  Pod->>Svc: Update
  Svc->>DB: save

  Note over Pod,DB: time 3 spec change same row
  CR->>API: spec change
  API->>Pod: enqueue
  Pod->>Svc: GetById hit
  Pod->>Svc: Update
  Svc->>DB: save

  Note over Pod,DB: time 4 user deletes CR
  CR->>API: kubectl delete
  API->>CR: deletionTimestamp object stays because finalizer
  API->>Pod: enqueue
  Pod->>Svc: Unregister
  Svc->>DB: delete row
  Pod->>CR: remove finalizer
  Note over API: object gone
```



Watcher init happens once per leader (`OnStartedLeading`). Followers do not Watch. DR standby may hold the Lease but must not Register/Update/Unregister.

**Create vs update**

- `existing = GetById` (which string is the id is open: [Name and id in the database](#name-and-id-in-the-database)).
- **Register** only if `existing == nil`.
- **Update** if a row hits and we own it, or `managed_by_operator` is false (adopt). Secret rotation and spec edits are both Update — the row is already there. Register again would be 400 unique.
- If a row hits and another CR owns it: `DuplicateInstanceName`.

**Finalizer is set while the CR is live; it only *acts* on delete.**

`metadata.finalizers` is a list of strings. Adding `maas.netcracker.com/instance` does **not** delete anything. It tells the apiserver: when the user later runs `kubectl delete`, **do not remove the object from etcd** until this controller PATCHes that string **off** the list.

We add it **after a successful Register** (not in the delete handler):

- If we added it only when `deletionTimestamp` is set, current apiservers **reject adding** a finalizer during deletion. The CR would vanish and the MaaS row could stay.
- After Register we must survive crash/restart: the CR still exists, the finalizer is already there, delete will wait for Unregister.

On delete: see `deletionTimestamp` → Unregister (or Orphan) → **remove** the finalizer. That is the only moment the finalizer “does work”. `PATCH status Registered` is a separate write to `status.conditions`; it is not the finalizer.

#### Delete

**How delete works (and why deletionTimestamp exists)**

Yes: user asks Kubernetes to delete the CR → apiserver does **not** drop the object if a finalizer is set → it only writes `metadata.deletionTimestamp` → we Unregister → we remove the finalizer → apiserver then deletes the CR from etcd.

`deletionTimestamp` is **the delete request**, recorded on the object that is still there. We need it because, with a finalizer, `Get` still succeeds. Spec, name, Secrets refs are unchanged. Without that timestamp, ProcessCR cannot tell “user wants this gone” from “normal Update”. It would keep Register/Update forever and never Unregister.

Without a finalizer there is no timestamp: `kubectl delete` removes the object immediately, next reconcile is `NotFound`, we Unregister by name. Then the CR is already gone **before** Unregister finishes (InUse 400, crash, slow health-check) — orphan MaaS row. The finalizer keeps the CR in `Terminating` until Unregister (or Orphan) succeeds. `deletionTimestamp` is how that waiting object is marked “delete in progress”.

##### kubectl delete: Unregister, Orphan, or InUse

```mermaid
sequenceDiagram
  participant User
  participant API as kube_apiserver
  participant CR as KafkaInstance
  participant Pod as ProcessCR
  participant Svc as InstanceService
  User->>API: kubectl delete CR
  API->>CR: set deletionTimestamp keep object
  Note over CR: Terminating finalizer still set
  API->>Pod: enqueue
  Pod->>CR: Get deletionTimestamp set
  alt deletionPolicy Unregister OK
    Pod->>Svc: Unregister
    Pod->>CR: remove finalizer
    API->>API: finalizers empty delete from etcd
  else deletionPolicy Orphan
    Pod->>CR: remove finalizer leave MaaS row
    API->>API: finalizers empty delete from etcd
  else Unregister InUse
    Pod->>CR: status InstanceInUse keep finalizer
    Pod->>Pod: RequeueAfter 30s
    Note over CR: stays Terminating
  end
```



**Keep the instance (do not Unregister).** Kubernetes will not cancel `kubectl delete`. You cannot clear `deletionTimestamp`. The CR is going away. To keep the **MaaS row**:

- Before delete: set `spec.deletionPolicy: Orphan`, then `kubectl delete`. ProcessCR removes the finalizer without Unregister. CR gone, instance stays (REST can still use it).
- Already Terminating: PATCH `spec.deletionPolicy: Orphan` (spec is still writable). Next reconcile skips Unregister, removes the finalizer. CR gone, instance stays. There is no “undelete CR.”

#### Delete with existing topics or vhosts

Unregister does **not** talk to Kafka. `[RemoveInstanceRegistration](../maas/maas-service/service/instance/kafka_instances_dao.go)` deletes the PostgreSQL row; if topics (or Rabbit vhosts) still reference that instance, the FK fails → 400. Same for “cannot delete default while another instance exists.”

**Where InstanceInUse is set.** It is **not** `spec.instanceInUse` and **not** a new MaaS/PostgreSQL column. It is the Kubernetes `status.conditions[].reason` string on the existing `Ready` condition:

```yaml
status:
  conditions:
    - type: Ready
      status: "False"
      reason: InstanceInUse
      message: "unable to delete non empty kafka instance platform-kafka"
```

ProcessCR writes that when Unregister returns 400 (FK: topics/vhosts still reference the instance). The 400 is MaaS; the operator copies it onto CR status so `kubectl get` shows why Terminating is stuck. Do not PATCH status on every retry if the reason is already `InstanceInUse` (that Watch event would hot-loop).

The CR is `Terminating` for as long as that 400 lasts. It is not deleted from etcd. It will not finish by itself just because time passed.

**What retriggers Unregister.** Not a CR spec change, not “topics deleted” Watch (we do not watch topics), not a loop inside KafkaInstanceService.

`RequeueAfter` is **not** a field on the CR. It is a field on `controller-runtime`’s `ctrl.Result`, returned from `Reconcile`. That is the Kubernetes operator library (`sigs.k8s.io/controller-runtime`), which wraps client-go’s workqueue. We do **not** add a queue table in PostgreSQL, a Fiber endpoint, or `spec.requeueAfter`.

```go
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    // ProcessCR: Unregister returned 400 InstanceInUse
    return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}
```

How the library processes it (we do not write this loop):

1. The manager runs workers that pop a key (`namespace/name`) from an in-memory queue and call our `Reconcile`.
2. If we return `Result{}` and `err == nil`, the key is done until the next Watch (CR/Secret).
3. If we return `Result{RequeueAfter: 30s}`, the library calls `queue.AddAfter(req, 30s)` — a timer in that process. After 30s the same key is pushed back. No apiserver PATCH.
4. If we return a non-nil `error`, the library rate-limits and retries ([Reconcile](#reconcile) backoff). Prefer `RequeueAfter` for expected InUse, not an error.

ProcessCR returns that result. After 30s the **same key** is reconciled again: Get CR (still Terminating) → Unregister again. The apiserver does not send a new delete. Users do not PATCH the CR.

When the last topic/vhost is gone, that **timer** Unregister succeeds → remove finalizer → CR deleted. No second `kubectl delete`. Until then, yes, we periodically call MaaS Unregister (PostgreSQL FK check).

Same `RequeueAfter` if Unregister fails because the instance is still default and others remain.

##### InUse: RequeueAfter until topics or vhosts are gone

```mermaid
sequenceDiagram
  participant Q as workqueue
  participant Pod as ProcessCR
  participant Svc as InstanceService
  participant CR as KafkaInstance
  Pod->>Svc: Unregister
  Svc-->>Pod: 400 FK topics remain
  Pod->>CR: PATCH status reason InstanceInUse once
  Pod->>Q: RequeueAfter 30s
  Note over Q: no CR Watch
  Q->>Pod: same key after 30s
  Pod->>Svc: Unregister again
```



---

### Multi-MaaS and ownership

Each MaaS has its own PostgreSQL. A Lease does **not** separate two installs (each namespace has its own `maas-operator-leader`).

Claim is **`spec.operatorNamespace`**. Required, CEL-immutable. ProcessCR applies only if it equals this MaaS `CLOUD_NAMESPACE`. Omitted or other namespace: skip, **do not PATCH**. Watch is the whole cluster (instance CRs); `operatorNamespace` is who applies.

Rejected alternatives (watch-only-own-NS, watch-list, namespace annotation) and the dual-Register bug: [Multi-MaaS alternatives](operator_design_notes.md#multi-maas-alternatives).

```mermaid
flowchart LR
  crA[CR platform-kafka operatorNamespace=maas-core]
  crB[CR tenant-kafka operatorNamespace=maas-tenant]
  maasCore[maas-core]
  maasTenant[maas-tenant]
  crA -->|claims| maasCore
  crB -->|claims| maasTenant
  crA -.->|ignore no status write| maasTenant
  crB -.->|ignore no status write| maasCore
```

DR standby does not reconcile. Replica HA for **one** MaaS is the Lease ([Scaling](#scaling-if-it-is-a-single-service)), not `operatorNamespace`. Composite / tenant app namespaces are unrelated: the operator still sees those CRs (cluster watch); it claims only when `operatorNamespace` matches.

---

## Related notes

See `[operator_design_notes.md](operator_design_notes.md)`: [Scenario A vs B recommendation](operator_design_notes.md#scenario-a-vs-b-recommendation), [Kubernetes Lease lock](operator_design_notes.md#kubernetes-lease-lock), [How replicas know who is watching](operator_design_notes.md#how-replicas-know-who-is-watching), [Why several MaaS replicas watching the same CRs is a bug](operator_design_notes.md#why-several-maas-replicas-watching-the-same-crs-is-a-bug), [Multi-MaaS alternatives](operator_design_notes.md#multi-maas-alternatives).

---

## TODO

- Investigate name migration of the CR (`metadata.name`, `metadata.namespace`) vs instance `id` in the database. See [Name and id in the database](#name-and-id-in-the-database).
- Research Blue/Green when adopting existing CRs onto a new operator (MaaS BG sibling, or replacing an old operator). Who claims (`operatorNamespace` vs two `CLOUD_NAMESPACE`s), finalizers on the old install, `origin_cr` / `managed_by_operator` after switch, and whether the new operator auto-adopts or the CRs must be re-applied.
- Consider making the RabbitMQ URL unique (like Kafka `addresses`). Today Rabbit has no URL unique; a new CR can reuse `apiUrl` / `amqpUrl` of an old REST row. Kafka `23505` is the “forgot old id” safety net.