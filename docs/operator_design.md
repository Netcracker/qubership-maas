# MaaS broker-discovery operator

Design proposal (not implemented yet). Do not confuse with `[custom_resources(CR).md](custom_resources(CR)`.md) (`kind: MaaS` Topic/VHost declarations processed by core-operator).

Background (Lease lock, alternatives): `[operator_design_notes.md](operator_design_notes.md)`.

## Table of Contents

- [Overview](#overview)
- [High-Level Architecture](#high-level-architecture)
  - [Backward compatibility and downgrade](#backward-compatibility-and-downgrade)
  - [Secret access (namespaced)](#secret-access-namespaced)
- [CRD sketch](#crd-sketch)
  - [MaasKafkaInstance](#maaskafkainstance)
  - [MaasRabbitInstance](#maasrabbitinstance)
  - [Secrets](#secrets)
  - [MaasDefaultInstance](#maasdefaultinstance)
- [Mapping layer (CR vs service structs)](#mapping-layer-cr-vs-service-structs)
- [Topology](#topology)
  - [Single microservice](#single-microservice-reconciler-and-maas-logic-in-one-process)
  - [Scaling if it is a single service](#scaling-if-it-is-a-single-service)
- [Deployment sequence](#deployment-sequence)
  - [MaaS first, then instance CRs](#maas-first-then-instance-crs)
  - [Instance CRs first (otherwise)](#instance-crs-first-otherwise)
- [Special cases](#special-cases)
  - [Default instance](#default-instance)
    - [Scenario 1 — spec.default on the instance CR](#scenario-1--specdefault-on-the-instance-cr)
    - [Scenario 2 — dedicated MaasDefaultInstance CR](#scenario-2--dedicated-maasdefaultinstance-cr)
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
| `MaasKafkaInstance` | `maas.netcracker.com/v1` | Namespaced | Registers a Kafka broker with maas-service |
| `MaasRabbitInstance` | `maas.netcracker.com/v1` | Namespaced | Registers a RabbitMQ broker with maas-service |

`MaasKafkaInstance` / `MaasRabbitInstance` map 1:1 onto `[model.KafkaInstance](../maas/maas-service/model/kafka_model.go)` / `[model.RabbitInstance](../maas/maas-service/model/rabbit_model.go)`. Instance CRs are the **desired** connection config; MaaS DB remains the **runtime** store used by topic/vhost APIs. Default: [two scenarios](#default-instance) (`spec.default` on the instance CR, or a dedicated `MaasDefaultInstance`).

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
  adopt -.-> todoTake["TODO: later spec.takeOver for explicit consent. v1 auto-adopts"]
  apply -->|"managed_by_operator true, origin_cr is another CR"| dup["PATCH status Ready=False reason=DuplicateInstanceName"]
  reg --> fin["Ensure finalizer on CR metadata"]
  upd --> fin
  adopt --> fin
  fin --> patch["PATCH CR status: phase, Ready, Stalled, lastRequestId, observedGeneration, isDefault, secretRevisions"]
  patch -.-> todoReady["TODO: later broker down. Check in MaaS health-check, then PATCH CR Ready. Not in ProcessCR."]
  classDef todo fill:#fff4cc,stroke:#c9a227,color:#1a1a1a
  classDef tip fill:#e8f4fc,stroke:#4a90c4,color:#1a1a1a
  class todoTake,todoReady,nameId todo
  class en tip
```



**Key design decisions:**

- The operator runs **cluster-wide** — no static `--watch-namespaces` list. Instance CRs may live in any namespace.
- Each managed CR declares its operator in immutable `spec.operatorNamespace`.
- CRs whose `spec.operatorNamespace` differs from this MaaS `CLOUD_NAMESPACE` are silently skipped (no PATCH, no Register).
- Credentials for `MaasKafkaInstance` / `MaasRabbitInstance` are read from Kubernetes Secrets at reconcile. The operator **Watches** those Secrets. A Secret `resourceVersion` change enqueues the CR even when spec `generation` did not change. `status.secretRevisions` stores those revisions, never secret bytes.
- **Periodic resync every 10 minutes.** Each claimed instance CR is reconciled again (`MAAS_INSTANCE_RESYNC_INTERVAL`, default `10m`) even when spec and Secrets did not change. Re-reads Secrets, refreshes `status.isDefault` from PG, and retries InUse / SecretError. Same interval for `MaasDefaultInstance` if that CR is used.
- Secret access is **namespaced**, not cluster-wide: the ClusterRole carries no `secrets` permission. Each namespace containing Secret-backed CRs grants access through a small Role + RoleBinding — see [Secret access (namespaced)](#secret-access-namespaced).
- **One Deployment.** ProcessCR runs in `maas-service`, same process as Fiber. No sibling operator pod, no manager REST hop, no extra basic-auth/M2M to another MaaS process. Apply is in-process `KafkaInstanceService` / `RabbitInstanceService`. See [Single microservice](#single-microservice-reconciler-and-maas-logic-in-one-process).
- **Lease, not a singleton pod.** HPA still scales HTTP. Only the `maas-operator-leader` holder Watches. See [Scaling](#scaling-if-it-is-a-single-service).
- **Operator optional.** Helm `OPERATOR_ENABLED` enables or disables Watch + ProcessCR. See [Backward compatibility and downgrade](#backward-compatibility-and-downgrade).
- **deletionPolicy.** `Unregister` (default): delete the CR and the PG row. `Orphan`: delete the CR, keep the row. Only read while Terminating. See [Delete](#delete).
- **Finalizer.** `maas.netcracker.com/instance` after successful Register. Without it, `kubectl delete` drops the CR immediately and can leave an orphan PG row. See [Delete](#delete).
- **Create vs update.** Register vs Update from `GetById`, not from Watch ADDED vs MODIFIED. How name maps to PG `id`: [Name and id in the database](#name-and-id-in-the-database).
- **Status.** `Ready` + `Stalled` only. `phase` is for `kubectl`. Automate on conditions.
- **No SecretRef on the service model.** Mapper loads CR + Secrets into existing `model.KafkaInstance` / `RabbitInstance`. InstanceService and the manager REST body stay resolved credentials, not Secret names.
- **managed_by_operator.** PG boolean. `true` after the operator Register/adopt. Existing REST rows stay `false` until a CR writes that id. Manager REST may Update only while this is `false`; after `true`, REST of that id is rejected. See [CR vs REST](#cr-vs-rest-and-optional-takeover).
- **Default.** Scenario 1 does not steal on every reconcile. See [Default instance](#default-instance).

### Backward compatibility and downgrade

Manager REST stays. Instances can still be Register/Update/Unregister’d when the operator is off. Existing PG rows are unchanged until a CR adopts them (`managed_by_operator`).

`OPERATOR_ENABLED` (Helm) turns Watch + ProcessCR on or off. Flipping it false must **not** Unregister instances. Only CR deletion with `deletionPolicy: Unregister` does that.

Open: skip Watch + ProcessCR only, or also drop CRDs/RBAC. Leftover CRs and finalizers, `managed_by_operator` lock on PG rows, and whether REST is allowed again. Older MaaS does not read `managed_by_operator`; the column must default (`false`) so an older binary still works. See [CR vs REST](#cr-vs-rest-and-optional-takeover).

### Secret access (namespaced)

The ClusterRole is for cluster watch of instance CRs only (`get` / `list` / `watch` / status PATCH / finalizers). It does **not** include `secrets`.

Each namespace that holds a `MaasKafkaInstance` or `MaasRabbitInstance` (and their `*SecretRef` Secrets) needs a Role + RoleBinding on the operator SA: `get` / `watch` of Secrets in that namespace. Same NS as the CR in v1 (`secretRef.namespace` is out of scope).

---

## CRD sketch

Two namespaced kinds, group `maas.netcracker.com/v1`: `MaasKafkaInstance`, `MaasRabbitInstance` (any namespace). Claim: required immutable `spec.operatorNamespace == CLOUD_NAMESPACE`. Credentials live in Secrets in the **same** namespace as the instance CR (no `secretRef.namespace` in v1). CRD extras: category `maas`, short names, printer columns, `selectableFields` on `spec.operatorNamespace` (K8s 1.32+), CEL `self == oldSelf` on identity fields. `MaasDefaultInstance` is [under discussion](#maasdefaultinstance).

| Kind | Short name | `kubectl get` columns |
|------|------------|------------------------|
| `MaasKafkaInstance` | `mkafi` | `PHASE`, `READY`, `DEFAULT`, `AGE` |
| `MaasRabbitInstance` | `mrabi` | `PHASE`, `READY`, `DEFAULT`, `AGE` |

### MaasKafkaInstance

#### CR example

What ProcessCR sees after Get on a registered Kafka CR (apiserver-filled metadata included). Inline comments on spec/status are documentation, not YAML schema.

```yaml
apiVersion: maas.netcracker.com/v1
kind: MaasKafkaInstance              # TODO: MaasKafkaInstance vs KafkaInstance
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
  default: false                       # when not using MaasDefaultInstance
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

#### CRD

```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: maaskafkainstances.maas.netcracker.com
spec:
  group: maas.netcracker.com
  names:
    categories: [maas]
    kind: MaasKafkaInstance
    listKind: MaasKafkaInstanceList
    plural: maaskafkainstances
    shortNames: [mkafi]
    singular: maaskafkainstance
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
                default:
                  type: boolean
                  description: Request this instance as the MaaS default when MaasDefaultInstance is not used. SetDefault only if exactly one claimed CR of this kind has this true.
                  default: false
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

### MaasRabbitInstance

#### CR example

```yaml
apiVersion: maas.netcracker.com/v1
kind: MaasRabbitInstance             # TODO: MaasRabbitInstance vs RabbitInstance
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
  default: false                       # when not using MaasDefaultInstance
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

#### CRD

```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: maasrabbitinstances.maas.netcracker.com
spec:
  group: maas.netcracker.com
  names:
    categories: [maas]
    kind: MaasRabbitInstance
    listKind: MaasRabbitInstanceList
    plural: maasrabbitinstances
    shortNames: [mrabi]
    singular: maasrabbitinstance
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
                default:
                  type: boolean
                  description: Request this instance as the MaaS default when MaasDefaultInstance is not used. SetDefault only if exactly one claimed CR of this kind has this true.
                  default: false
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


Comments:

**Identity / claim**

- **kind.** Under discussion: `MaasKafkaInstance` / `MaasRabbitInstance` vs `KafkaInstance` / `RabbitInstance` (same prefix question for `MaasDefaultInstance`).
- **metadata.name.** DNS-1123 (lowercase, ≤63). Kubernetes rename is delete+create; not an in-place rename of the PG row. Mapping to PG `id` is open: [Name and id in the database](#name-and-id-in-the-database).
- **spec.operatorNamespace.** Required. Which MaaS install claims this CR (`CLOUD_NAMESPACE`). CEL `self == oldSelf` — retarget is delete+recreate. Pattern RFC-1123 label. Missing or other namespace: skip, **do not PATCH**. See [Multi-MaaS and ownership](#multi-maas-and-ownership).
- **metadata.namespace.** Where the CR lives (broker NS). Not the claim.

**Broker connection** (map onto `[KafkaInstance](../maas/maas-service/model/kafka_model.go)` / `[RabbitInstance](../maas/maas-service/model/rabbit_model.go)`)

- **spec.addresses / maasProtocol.** Kafka. One protocol key in `addresses`, matching `maasProtocol` (`PLAINTEXT` | `SASL_PLAINTEXT` | `SSL` | `SASL_SSL`).
- **spec.apiUrl / amqpUrl.** Rabbit only.
- **spec.caCertSecretRef.** Kafka CA. `{name, key}` in the **same** NS as the CR. Omit if no CA. Mapper → `caCert`.
- **spec.credentialsSecretRef.** `{name, keys[{key, name}]}`. Secret in the same NS. `key` is `Secret.data`; `name` is the field on the Kafka Auth DTO / Rabbit user+password. Duplicate `keys[].name` → `InvalidSpec`, `Stalled=True`. Inline `spec.password` is rejected.
- **spec.default.** Request this instance as the MaaS default when [MaasDefaultInstance](#maasdefaultinstance) is not used. ProcessCR `SetDefault` only if exactly one claimed CR of this kind has `spec.default: true`. `status.isDefault` is observed PG state.
- **spec.deletionPolicy.** `Unregister` (default) or `Orphan`. Only read while Terminating.

**Status**

- **status.phase.** `Processing` | `Succeeded` | `BackingOff` | `InvalidConfiguration`. kubectl only. Do not automate against it.
- **status.conditions.** `Ready` + `Stalled` only. `x-kubernetes-list-type: map` keyed by `type`.
  - `Ready=True` — this generation applied; instance usable (success reason `InstanceRegistered`).
  - `Ready=False`, `Stalled=False` — transient; retry with backoff (`SecretError`, `HealthCheckFailed`, `InstanceInUse`).
  - `Ready=False`, `Stalled=True` — permanent; wait for spec change (`InvalidSpec`, `DuplicateInstanceName`).
- **status.lastRequestId.** ProcessCR sets this when it PATCHes status. There is no manager HTTP request: ProcessCR generates an `X-Request-Id` (same as Fiber `ExtractOrAttachXRequestId` when the header is missing), puts it on the Go context so InstanceService logs share it, then writes that string here.
- **status.observedGeneration.** Stamped on success or `Stalled=True`. Left behind on transient.
- **status.isDefault.** Observed PG default. Not a request. First instance in an empty DB is true (`ForcedDefault`).
- **status.secretRevisions.** Secret `resourceVersion`s (never bytes). Updated on Secret Watch and on the 10m resync.

**Ready / Stalled reasons**

- `InstanceRegistered` — Register/Update succeeded (`Ready=True`, `Stalled=False`, `phase=Succeeded`).
- `SecretError` — Secret missing, key missing/empty, or forbidden (`Ready=False`, `Stalled=False`, `phase=BackingOff`).
- `HealthCheckFailed` — Register/Update health-check 400. Do not Unregister a previous good row (`BackingOff`).
- `InstanceInUse` — Unregister 400, topics/vhosts still on the instance. Keep finalizer (`BackingOff`).
- `DuplicateInstanceName` — another CR owns that id (`Stalled=True`, `phase=InvalidConfiguration`).
- `InvalidSpec` — duplicate `keys[].name`, bad `maasProtocol`, etc. (`Stalled=True`).
- `ForcedDefault` — first row became default (`Ready=True`). Informational.

**Kubernetes**

- **metadata.uid / resourceVersion / generation.** Apiserver-filled. ProcessCR compares `generation` to `status.observedGeneration`.
- **metadata.finalizers.** `maas.netcracker.com/instance` after successful Register. Without it, `kubectl delete` drops the object immediately — orphan PG row.
- **metadata.deletionTimestamp.** Present only after `kubectl delete`. ProcessCR uses it to tell Terminating from Update.
- **Secrets watch.** A Secret change does not bump `metadata.generation`. Watch those Secrets; enqueue CRs whose `*SecretRef.name` matches.
- **status is not spec.** ProcessCR PATCHes it. Do not add `spec.instanceInUse`.

### MaasDefaultInstance

**Under discussion.** First insert into an empty DB still becomes default (`ForcedDefault`). Later default may stay manager REST `SetDefault` until this kind exists.

| Kind | Short name | `kubectl get` columns |
|------|------------|------------------------|
| `MaasDefaultInstance` | `mdefl` | `PHASE`, `READY`, `KAFKA`, `RABBIT`, `AGE` |

#### CR example

```yaml
# Singleton in the operator namespace.
apiVersion: maas.netcracker.com/v1
kind: MaasDefaultInstance            # TODO: MaasDefaultInstance vs DefaultInstance
metadata:
  name: defaults
  namespace: maas-core
spec:
  operatorNamespace: maas-core         # required, immutable. Skip (no PATCH) if not this operator. metadata.namespace must equal this
  kafka: platform-kafka                # optional. PG instance id. Empty: do not SetDefault for Kafka
  rabbit: platform-rabbit              # optional. Same for Rabbit
status:
  phase: Succeeded
  kafka: platform-kafka                # observed PG default Kafka id
  rabbit: platform-rabbit
  lastRequestId: "req-defl-1"
  conditions:
    - type: Ready
      status: "True"
      reason: DefaultSet
      lastTransitionTime: "2026-08-27T13:42:00Z"
      observedGeneration: 1
    - type: Stalled
      status: "False"
      reason: Succeeded
      lastTransitionTime: "2026-08-27T13:42:00Z"
      observedGeneration: 1
  observedGeneration: 1
```

#### CRD

```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: maasdefaultinstances.maas.netcracker.com
spec:
  group: maas.netcracker.com
  names:
    categories: [maas]
    kind: MaasDefaultInstance
    listKind: MaasDefaultInstanceList
    plural: maasdefaultinstances
    shortNames: [mdefl]
    singular: maasdefaultinstance
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
        - jsonPath: .status.kafka
          name: Kafka
          type: string
        - jsonPath: .status.rabbit
          name: Rabbit
          type: string
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
          x-kubernetes-validations:
            - rule: "self.metadata.name == 'defaults'"
              message: metadata.name must be defaults
            - rule: "self.metadata.namespace == self.spec.operatorNamespace"
              message: metadata.namespace must equal spec.operatorNamespace
          properties:
            spec:
              type: object
              required: [operatorNamespace]
              properties:
                operatorNamespace:
                  type: string
                  description: Must equal metadata.namespace and this operator CLOUD_NAMESPACE. Immutable.
                  minLength: 1
                  maxLength: 63
                  pattern: '^[a-z0-9]([-a-z0-9]*[a-z0-9])?$'
                  x-kubernetes-validations:
                    - rule: self == oldSelf
                      message: spec.operatorNamespace is immutable after creation
                kafka:
                  type: string
                  description: Kafka instance PG id to make the default. Empty means do not SetDefault.
                  minLength: 1
                  maxLength: 63
                  pattern: '^[a-z0-9]([-a-z0-9]*[a-z0-9])?$'
                rabbit:
                  type: string
                  description: Rabbit instance PG id to make the default. Empty means do not SetDefault.
                  minLength: 1
                  maxLength: 63
                  pattern: '^[a-z0-9]([-a-z0-9]*[a-z0-9])?$'
            status:
              type: object
              properties:
                phase:
                  type: string
                kafka:
                  type: string
                  description: Observed PG default Kafka instance id
                rabbit:
                  type: string
                  description: Observed PG default Rabbit instance id
                lastRequestId:
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

Comments:

- **metadata.name.** Fixed `defaults` (CEL). One object per MaaS install.
- **spec.operatorNamespace.** Required, immutable. `metadata.namespace` must equal it. Skip if not `CLOUD_NAMESPACE`.
- **spec.kafka / spec.rabbit.** Optional instance ids. Empty: do not `SetDefault` for that type.
- **status.kafka / status.rabbit.** Observed PG default ids.
- No Secrets, no `deletionPolicy`. Delete of this CR does **not** Unregister instances and does **not** clear PG default flags.
- `InstanceNotFound` — this CR points at an id with no PG row (`Ready=False`, `Stalled=False`; do not clear current default).

---

## Mapping layer (CR vs service structs)

`*SecretRef` and `operatorNamespace` do **not** belong on `[model.KafkaInstance](../maas/maas-service/model/kafka_model.go)` / `[RabbitInstance](../maas/maas-service/model/rabbit_model.go)`. Those structs are the REST body and the PostgreSQL row: they store **resolved** `caCert`, `credentials` / `user`+`password`, not pointers to Secrets. Putting refs there would change the public manager API and persist names instead of secrets.

Add a **new operator layer** (not a new DB table):

- CR Go types (`MaasKafkaInstance` / `MaasRabbitInstance` / `MaasDefaultInstance`, `SecretKeyMapping`, `SecretKeyRef`) live in the operator package. They are the apiserver schema.
- Mapper: load CR + Secrets → fill existing `model.KafkaInstance` / `RabbitInstance` (`Id` mapping open — [Name and id](#name-and-id-in-the-database), `Addresses`, `Default: false`, `MaasProtocol`, `CACert`, `Credentials` / `ApiUrl`, `AmqpUrl`, `User`, `Password`). Instance mapper never sets `Default: true`.
- Apply still calls `KafkaInstanceService` / `RabbitInstanceService` with that model. No SecretRef in the service. `SetDefault` is only the `MaasDefaultInstance` reconciler (under discussion).

Optional DB columns on the instance row (`managed_by_operator`, `namespace`, `origin_cr`) are schema extras, not a second instance struct. Existing rows: `managed_by_operator = false`, `namespace` empty until a CR writes it.

```mermaid
flowchart LR
  cr[MaasKafkaInstance spec refs]
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
      crA[MaasKafkaInstance / MaasRabbitInstance]
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

## Deployment sequence

How **Kubernetes** and **Argo CD** treat the two install orders. Who becomes the PG default is [Default instance](#default-instance). Kafka/Rabbit brokers can be up earlier; MaaS does not proxy traffic.

Argo **Sync** is apply to the API. **Health** is separate: `Ready=False` is Degraded; no status yet is Progressing. A wave that waits for Healthy blocks.

### MaaS first, then instance CRs

MaaS chart installs CRDs and `maas-service`. Leader Watches an empty list. A later wave / other Application applies `MaasKafkaInstance` / `MaasRabbitInstance`.

Kubernetes: kinds exist, apply succeeds, ProcessCR runs on each create. Argo: MaaS Application Healthy (Deployment). Instance Application Sync green, Health Progressing until ProcessCR PATCHes `Ready`, then Healthy. Topic APIs have no default until that first Register — same as today.

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
  Argo->>API: apply MaasKafkaInstance
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

## Special cases

Defaults, name/id in PG, REST coexistence, delete/lifecycle, and multi-MaaS. ProcessCR rules above still apply.

### Default instance

MaaS allows **one** default Kafka instance and **one** default Rabbit instance per PostgreSQL. Topics/vhosts with no `instance` id use that default.

First insert into an empty DB still becomes default (`ForcedDefault` in [DAO](../maas/maas-service/service/instance/kafka_instances_dao.go)). “First” is whichever Register commits — not a name you pick. After that, two ways to request or switch the default:

#### Scenario 1 — `spec.default` on the instance CR

No `MaasDefaultInstance`. Do **not** map `spec.default` onto every Register/Update. If several CRs have `spec.default: true`, last-writer steal would flip the default on every Watch or 10m resync.

ProcessCR on an instance CR (event or resync), Kafka and Rabbit separately, claimed CRs only (`operatorNamespace == CLOUD_NAMESPACE`):

1. **No default in PG.** The instance being applied becomes default (`ForcedDefault`). PATCH its `status.isDefault: true`.
2. **Default already in PG.** List all claimed instance CRs of that kind. Change the default in PG **only if exactly one** CR has `spec.default: true` — `SetDefault` that id. Then PATCH `status.isDefault` on **all** of them from PG. Zero or two-or-more `spec.default: true`: leave PG as-is; still PATCH observed `status.isDefault` on all. Do not send Update `default: false` on the current default (DAO 400).

Either edit order works. Two `true` at once is a no-op on PG. One `true` (the other `false`) is the switch.

```mermaid
flowchart TD
  ev[Watch or 10m resync on one CR]
  ev --> empty{"PG has a default?"}
  empty -->|no| force["ForcedDefault this instance. PATCH isDefault"]
  empty -->|yes| list["List claimed CRs of this kind"]
  list --> n{"How many spec.default true?"}
  n -->|exactly one| set["SetDefault that id"]
  n -->|0 or 2 plus| keep["Leave PG default"]
  set --> patchAll["PATCH status.isDefault on all those CRs"]
  keep --> patchAll
```

```mermaid
sequenceDiagram
  participant User
  participant A as CR_A
  participant B as CR_B
  participant PC as ProcessCR
  participant DB as PostgreSQL

  Note over User,DB: 1 first instance empty DB
  User->>A: apply
  A->>PC: reconcile
  PC->>DB: Register
  Note over DB: no default
  PC->>DB: ForcedDefault A
  PC->>A: PATCH status.isDefault true

  Note over User,DB: 2 second instance
  User->>B: spec.default false
  B->>PC: reconcile
  PC->>DB: Register default false
  PC->>PC: list CRs one spec.default true A
  PC->>A: PATCH status.isDefault true
  PC->>B: PATCH status.isDefault false

  Note over User,DB: 3 two spec.default true no switch
  User->>B: spec.default true
  B->>PC: reconcile
  PC->>PC: list CRs two true
  Note over DB: still A
  PC->>A: PATCH status.isDefault true
  PC->>B: PATCH status.isDefault false

  Note over User,DB: 4 one spec.default true switch
  User->>A: spec.default false
  A->>PC: reconcile
  PC->>PC: list CRs only B true
  PC->>DB: SetDefault B
  PC->>A: PATCH status.isDefault false
  PC->>B: PATCH status.isDefault true
```

Manager REST steal is unchanged. This listing rule is operator-only.

**Deploy order.** If MaaS does not exist, CRs cannot exist ([Deployment sequence](#deployment-sequence)). The `ForcedDefault` “first informer item” case is only after CRDs exist and the leader Lists several CRs at once (same chart, or CRs applied while the pod had no Lease). First Register into empty PG wins, even if that CR has `spec.default: false`. Rule 2 then `SetDefault` if exactly one claimed CR has `spec.default: true`. If all are `false`, that ForcedDefault winner stays.

If the operator is already watching and CRs are applied one by one, the first Register is ForcedDefault — the one you applied first, not a random List order.

No Default CR, so no `InstanceNotFound` at MaaS install. Instance CRs with no status yet: Argo Progressing until ProcessCR PATCHes `Ready`.

```mermaid
sequenceDiagram
  participant API as kube_apiserver
  participant PC as ProcessCR
  participant DB as PostgreSQL
  Note over API,DB: leader starts several CRs already in etcd
  PC->>API: List A and B
  PC->>DB: Register A ForcedDefault
  PC->>DB: Register B
  PC->>PC: list spec.default
  Note over DB: A default unless exactly one spec.default true
```

#### Scenario 2 — dedicated `MaasDefaultInstance` CR

Under discussion. Singleton `metadata.name: defaults` in `CLOUD_NAMESPACE`. If this CR exists, it wins: ignore `spec.default` on instance CRs.

**Reconcile (`MaasDefaultInstance`)**

ProcessCR on the Default CR only. Instance CRs do not `SetDefault`. Ignore `spec.default` on them while this CR exists.

1. Skip unless `metadata.namespace == spec.operatorNamespace == CLOUD_NAMESPACE` (and name is `defaults`).
2. **Empty `spec.kafka` / `spec.rabbit`.** Do not `SetDefault` for that type. Leave PG (ForcedDefault or last pointer). PATCH this CR `Ready=True` — nothing to resolve. First install with MaaS: ship `defaults` empty so Argo is Healthy. After instance CRs Register, set the id (step 4).
3. **Id set, `GetById` miss.** `Ready=False` `InstanceNotFound`. Do **not** clear PG. Argo **fails** (Degraded). Requeue / 10m resync. `Stalled=False` (retry, not a spec error). This is applying `defaults` with an id **before** that instance CR is registered.
4. **Id set, row exists, not already default.** `SetDefault`. PATCH instance CR `status.isDefault` if we own it, and this CR’s `status.kafka` / `status.rabbit`. `Ready=True`.
5. **Id set, row exists, already default.** PATCH status only. Do not `SetDefault` again on every Watch / 10m resync.
6. **Delete.** Drop the object, **leave** PG default flags. Re-apply later to point again.

**Switch.** Both instances already registered, then set `spec.kafka: B` on `defaults`. One `SetDefault`. No two-step on instance specs.

**Scenario 1** has no Default CR, so no `InstanceNotFound` at MaaS install.

```mermaid
sequenceDiagram
  participant Helm
  participant Def as MaasDefaultInstance
  participant Inst as MaasKafkaInstance
  participant DB as PostgreSQL

  Note over Helm,DB: first install empty spec Argo Healthy
  Helm->>Def: apply defaults spec.kafka empty
  Def->>DB: no SetDefault
  Note over Def: Ready=True
  Note over DB: ForcedDefault when instance Registers
  Helm->>Inst: apply platform-kafka
  Inst->>DB: Register ForcedDefault
  Helm->>Def: spec.kafka platform-kafka
  Def->>DB: GetById hit SetDefault
  Note over Def: Ready=True

  Note over Helm,DB: bad id set before instance CR
  Helm->>Def: apply defaults spec.kafka platform-kafka
  Def->>DB: GetById miss
  Note over Def: Ready=False InstanceNotFound Argo Degraded
```

A REST instance that is already default is not moved until `MaasDefaultInstance` names another id.

---

### Name and id in the database

REST instance `id` is a free-form string (Helm name, UUID, …). The CR has `metadata.name` and `metadata.namespace`. Those need not match the existing PG `id`. Topics, vhosts, and designators FK that id — it cannot be renamed. Two CRs (or a CR and a REST row) can collide on name, namespace, or Kafka `addresses` (jsonb unique). Rabbit has no URL unique. A CR in another namespace must not rewrite an instance already bound to one NS.

TODO: investigate how CR `metadata.name` / `metadata.namespace` map to PG `id` on create and on migrate of an old REST row. Draft: [Name and id in the database](operator_design_notes.md#name-and-id-in-the-database).

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
  participant CR as MaasKafkaInstance
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
  participant CR as MaasKafkaInstance
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
  participant CR as MaasKafkaInstance
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
4. If we return a non-nil `error`, the library rate-limits and retries (different from RequeueAfter). Prefer `RequeueAfter` for expected InUse, not an error.

ProcessCR returns that result. After 30s the **same key** is reconciled again: Get CR (still Terminating) → Unregister again. The apiserver does not send a new delete. Users do not PATCH the CR.

When the last topic/vhost is gone, that **timer** Unregister succeeds → remove finalizer → CR deleted. No second `kubectl delete`. Until then, yes, we periodically call MaaS Unregister (PostgreSQL FK check).

Same `RequeueAfter` if Unregister fails because the instance is still default and others remain.

##### InUse: RequeueAfter until topics or vhosts are gone

```mermaid
sequenceDiagram
  participant Q as workqueue
  participant Pod as ProcessCR
  participant Svc as InstanceService
  participant CR as MaasKafkaInstance
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

Claim is **`spec.operatorNamespace`**. Required, CEL-immutable. ProcessCR applies only if it equals this MaaS `CLOUD_NAMESPACE`. Omitted or other namespace: skip, **do not PATCH**. Watch is the whole cluster (instance CRs); `operatorNamespace` is who applies. `MaasDefaultInstance`: operator namespace only (`metadata.namespace == spec.operatorNamespace == CLOUD_NAMESPACE`).

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

See `[operator_design_notes.md](operator_design_notes.md)`: [Scenario A vs B recommendation](operator_design_notes.md#scenario-a-vs-b-recommendation), [Kubernetes Lease lock](operator_design_notes.md#kubernetes-lease-lock), [How replicas know who is watching](operator_design_notes.md#how-replicas-know-who-is-watching), [Why several MaaS replicas watching the same CRs is a bug](operator_design_notes.md#why-several-maas-replicas-watching-the-same-crs-is-a-bug), [Multi-MaaS alternatives](operator_design_notes.md#multi-maas-alternatives), [Name and id in the database](operator_design_notes.md#name-and-id-in-the-database), [comparison with other operators](operator_design_notes.md#comparison-with-dbaas-core-operator-and-nearby-operators).

---

## TODO

- Investigate name migration of the CR (`metadata.name`, `metadata.namespace`) vs instance `id` in the database. See [Name and id in the database](#name-and-id-in-the-database). Draft in notes.
- Research Blue/Green when adopting existing CRs onto a new operator (MaaS BG sibling, or replacing an old operator). Who claims (`operatorNamespace` vs two `CLOUD_NAMESPACE`s), finalizers on the old install, `origin_cr` / `managed_by_operator` after switch, and whether the new operator auto-adopts or the CRs must be re-applied.
- Rabbit has no URL unique — Kafka `addresses` unique is the “forgot old id” safety net. Decide whether Rabbit needs an equivalent.
- Kind names. `MaasKafkaInstance` / `MaasRabbitInstance` / `MaasDefaultInstance` vs `KafkaInstance` / `RabbitInstance` / `DefaultInstance`. Group is already `maas.netcracker.com`; the `Maas` prefix may be redundant or may avoid clashing with other operators' kinds.