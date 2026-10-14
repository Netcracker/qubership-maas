# Delta for broker-instances

`KafkaInstance` and `RabbitInstance` CR behavior, mapping, defaults, REST lock, delete.

## ADDED Requirements

### Requirement: Instance CR kinds

The system SHALL serve namespaced kinds `KafkaInstance` and `RabbitInstance` in group `maas.netcracker.com/v1` (short
names `mkafi` / `mrabi`). Credentials SHALL be read from Secrets in the same namespace as the CR.

#### Scenario: Successful Register

- GIVEN a claimed CR with valid spec and readable Secrets
- WHEN ProcessCR applies
- THEN InstanceService SHALL Register or Update the PG row
- AND status SHALL be `Ready=True` reason `InstanceRegistered`
- AND finalizer `maas.netcracker.com/instance` SHALL be present
- AND `managed_by_operator` SHALL be true on that row

#### Scenario: Secret rotation picked up on reconcile

- GIVEN a referenced Secret's `resourceVersion` differs from `status.secretRevisions`
- WHEN the CR is next reconciled (resync, CR change, or refresh annotation)
- THEN ProcessCR SHALL Update the PG row with the new credentials
- AND `generation` SHALL NOT be required to change
- AND `status.secretRevisions` SHALL store the new revisions, never secret bytes

#### Scenario: Secret change alone

- GIVEN a referenced Secret changes and nothing else happens
- WHEN no resync, CR change, or refresh annotation has occurred
- THEN the operator SHALL NOT be notified and SHALL NOT reconcile the CR

### Requirement: Mapping layer

The operator SHALL map CR + Secrets into existing `model.KafkaInstance` / `RabbitInstance`. Those models and manager
REST SHALL keep resolved credentials, not SecretRef or `operatorNamespace`.

#### Scenario: Mapper does not set Default

- GIVEN ProcessCR maps a claimed CR
- WHEN it calls InstanceService
- THEN the mapped model `Default` field SHALL be false
- AND default selection SHALL follow `DEFAULT_KAFKA_INSTANCE` / `DEFAULT_RABBIT_INSTANCE` after Register/Update

### Requirement: Kafka credentials by role

`KafkaInstance` SHALL carry admin and client credentials like manager REST. `spec.credentials` is optional; when set,
`admin` (exactly one entry) and `client` (one or more) SHALL both be present. Each entry SHALL name an auth `type` and a
Secret whose keys map to Auth DTO fields. The CRD SHALL reject an entry missing the fields its `type` needs.

#### Scenario: Admin and client credentials

- GIVEN a claimed `KafkaInstance` with `credentials.admin` and two `credentials.client` entries
- WHEN ProcessCR maps it
- THEN `Credentials["admin"]` SHALL hold the admin entry and `Credentials["client"]` both client entries, in order
- AND passwords SHALL be sent as `plain:<value>` from the raw Secret value
- AND topic responses SHALL return the client entries, as for REST-registered instances

#### Scenario: Kafka without authentication

- GIVEN a claimed `KafkaInstance` with no `spec.credentials`
- WHEN ProcessCR maps it
- THEN `Credentials` SHALL be empty and MaaS SHALL connect without authentication

#### Scenario: Incomplete credentials

- GIVEN a `KafkaInstance` with `credentials.admin` but no `credentials.client`, or a `SCRAM` entry without a
  `password` key
- WHEN it is applied
- THEN the API server SHALL reject it

### Requirement: Kafka addresses

`spec.addresses` SHALL accept every protocol MaaS accepts (`PLAINTEXT`, `SASL_PLAINTEXT`, `SSL`, `SASL_SSL`) and SHALL
contain an entry for `spec.maasProtocol`. MaaS connects with `addresses[maasProtocol]`; topic responses return the whole
map.

#### Scenario: Missing maasProtocol entry

- GIVEN `maasProtocol: SASL_SSL` and `addresses` with only `SASL_PLAINTEXT`
- WHEN the CR is applied
- THEN the API server SHALL reject it

### Requirement: Status conditions

Status SHALL expose `Ready` and `Stalled` only. `phase` is for kubectl.

#### Scenario: Condition reasons

- GIVEN ProcessCR finishes an apply
- WHEN it PATCHes status
- THEN reason SHALL be one of `InstanceRegistered`, `ForcedDefault`, `SecretError`, `HealthCheckFailed`,
  `InstanceInUse`, `InvalidSpec`, `DuplicateInstanceName`, `DuplicateNamespace`

### Requirement: deletionPolicy

While Terminating, `deletionPolicy: Unregister` (default) SHALL Unregister the PG row then remove the finalizer.
`Orphan` SHALL remove the finalizer and keep the row.

#### Scenario: Unregister blocked by dependents

- GIVEN `deletionPolicy: Unregister` and topics or vhosts still reference the instance
- WHEN Unregister returns 400
- THEN status reason SHALL be `InstanceInUse`
- AND the CR SHALL stay Terminating with the finalizer

#### Scenario: Orphan

- GIVEN `deletionPolicy: Orphan`
- WHEN the CR is deleted
- THEN `managed_by_operator` SHALL be set false and `cr_namespace` and `cr_name` cleared on the row
- AND the finalizer SHALL be removed
- AND the PG row SHALL remain
- AND manager REST SHALL be able to Update or Unregister that id

### Requirement: REST lock after operator write

While `OPERATOR_ENABLED` is true, manager REST SHALL Update or Unregister an instance id only while
`managed_by_operator` is false. After a CR Register or adopt, REST of that id SHALL be rejected. While `OPERATOR_ENABLED`
is false, the lock SHALL NOT be enforced.

#### Scenario: Unmanaged REST row

- GIVEN a PG row never written by a CR
- WHEN manager REST Updates that id
- THEN the Update SHALL succeed

#### Scenario: Duplicate CR

- GIVEN `managed_by_operator` true and `cr_namespace` and `cr_name` name another CR
- WHEN a second CR would write that id
- THEN status SHALL be `Ready=False` `Stalled=True` reason `DuplicateInstanceName`
- AND ProcessCR SHALL NOT Update

### Requirement: One broker per namespace

A broker namespace SHALL hold at most one claimed `KafkaInstance` and at most one claimed `RabbitInstance`. PG SHALL
enforce it with a partial unique index on `cr_namespace` over rows where `managed_by_operator` is true. Kafka and Rabbit
SHALL be independent.

#### Scenario: Second CR of a kind in one namespace

- GIVEN a row with `managed_by_operator` true and `cr_namespace` X owned by CR X/A
- WHEN ProcessCR applies claimed CR X/B of the same kind
- THEN status SHALL be `Ready=False` `Stalled=True` reason `DuplicateNamespace`, and the message SHALL name CR X/A
- AND ProcessCR SHALL NOT Register or Update

#### Scenario: Orphan frees the namespace

- GIVEN CR X/A was deleted with `deletionPolicy: Orphan`
- WHEN ProcessCR applies claimed CR X/B of the same kind
- THEN the namespace rule SHALL NOT reject it

#### Scenario: REST lock off while operator disabled

- GIVEN `OPERATOR_ENABLED` is false and a row has `managed_by_operator` true
- WHEN manager REST Updates that id
- THEN the Update SHALL succeed
- AND `managed_by_operator` SHALL stay true

### Requirement: Rabbit URL uniqueness

PostgreSQL SHALL reject a Rabbit instance whose `api_url` or `amqp_url` is already used by another id, the same way
Kafka `addresses` are unique. This SHALL apply to manager REST and to CRs.

#### Scenario: Duplicate Rabbit URL

- GIVEN a Rabbit instance row with `amqp_url` `amqp://rabbit.rabbit-infra:5672`
- WHEN another id is Registered or Updated with the same `amqp_url` or the same `api_url`
- THEN the write SHALL fail with a unique violation
- AND a CR write SHALL surface it as `Ready=False` `Stalled=True` reason `InvalidSpec`, with a message naming the URL

#### Scenario: Existing duplicates at upgrade

- GIVEN two existing Rabbit rows share an `api_url` or `amqp_url`
- WHEN the uniqueness migration runs
- THEN it SHALL create no index and `maas-service` SHALL fail to start
- AND the error SHALL list every shared `api_url` / `amqp_url` with the instance ids that use it

#### Scenario: Upgrade after fixing duplicates

- GIVEN the duplicate rows named in that error were fixed
- WHEN `maas-service` starts again
- THEN the migration SHALL create both unique indexes and startup SHALL continue

### Requirement: REST SetDefault while a default param is set

While `OPERATOR_ENABLED` is true and the `DEFAULT_*_INSTANCE` param for a kind is set, manager REST SetDefault for that
kind SHALL be rejected, because the param owns the default. While the param is empty or the operator is disabled, REST
SetDefault SHALL work.

#### Scenario: REST SetDefault while the param is set

- GIVEN `OPERATOR_ENABLED` is true and `DEFAULT_KAFKA_INSTANCE` is set
- WHEN manager REST sets a Kafka default instance
- THEN the request SHALL be rejected with a message naming the param
- AND the PG default SHALL NOT change

#### Scenario: REST SetDefault while the param is empty

- GIVEN `OPERATOR_ENABLED` is true and `DEFAULT_RABBIT_INSTANCE` is empty
- WHEN manager REST sets a Rabbit default instance
- THEN the default SHALL change, whether or not the row is managed by a CR

### Requirement: Default instance params

The default Kafka and Rabbit instance SHALL be selected by Application params `DEFAULT_KAFKA_INSTANCE` /
`DEFAULT_RABBIT_INSTANCE` (value = broker namespace, the `metadata.namespace` of the claimed CR of that kind; empty or a
DNS-1123 label). There SHALL be no `spec.default` and no `DefaultInstance` CR.

#### Scenario: Empty params

- GIVEN the param for that kind is empty and PG has no default
- WHEN the first Register of that kind commits
- THEN that row SHALL become default (`ForcedDefault`)
- AND later reconciles SHALL NOT steal default just because they reconcile

#### Scenario: Named CR appears

- GIVEN the param equals the namespace of a claimed CR whose row exists and is not default
- WHEN ProcessCR runs
- THEN it SHALL `SetDefault` that row's id
- AND `status.isDefault` SHALL reflect PG

#### Scenario: Same CR name in two namespaces

- GIVEN claimed Kafka CRs `ns-a/platform-kafka` and `ns-b/platform-kafka`, and `DEFAULT_KAFKA_INSTANCE` is `ns-a`
- WHEN ProcessCR reconciles both CRs, including later resyncs
- THEN the `ns-a` row SHALL be default and SHALL stay default
- AND ProcessCR of `ns-b/platform-kafka` SHALL NOT `SetDefault`

#### Scenario: Clearing the param

- GIVEN a PG default already exists
- WHEN the Application param is later cleared
- THEN ProcessCR SHALL NOT unset the PG default

### Requirement: Name and id

ProcessCR SHALL NOT rename an existing PG `id`.

#### Scenario: New instance

- GIVEN no row for `GetById(metadata.namespace)` and no migrate-by-name hit
- WHEN ProcessCR Registers
- THEN PG `id` SHALL equal `metadata.namespace`

#### Scenario: Adopt by namespace

- GIVEN `GetById(metadata.namespace)` hits
- WHEN ProcessCR applies
- THEN it SHALL Update that row and keep the id

#### Scenario: Adopt by old REST id

- GIVEN `metadata.name` is not the namespace and `GetById(name)` hits an unmanaged row
- WHEN ProcessCR applies
- THEN it SHALL Update that row, set `managed_by_operator` true, and store the CR namespace and name
- AND it SHALL NOT rename the id

### Requirement: Downgrade without Unregister

Syncing a MaaS version without the operator SHALL leave PG instance rows in place.

#### Scenario: Old chart

- GIVEN topics or vhosts use operator-created instance ids
- WHEN Argo CD Syncs a chart with no Watch, ProcessCR, or instance CRDs
- THEN those PG rows SHALL remain
- AND manager REST SHALL be the writer again
- AND no Unregister SHALL run as part of rollback
