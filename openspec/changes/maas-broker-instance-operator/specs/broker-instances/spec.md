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

#### Scenario: Secret change without spec generation

- GIVEN a referenced Secret's `resourceVersion` changes
- WHEN the Secret informer fires
- THEN the CR SHALL be enqueued
- AND `generation` SHALL NOT be required to change
- AND `status.secretRevisions` SHALL store those revisions, never secret bytes

### Requirement: Mapping layer

The operator SHALL map CR + Secrets into existing `model.KafkaInstance` / `RabbitInstance`. Those models and manager
REST SHALL keep resolved credentials, not SecretRef or `operatorNamespace`.

#### Scenario: Mapper does not set Default

- GIVEN ProcessCR maps a claimed CR
- WHEN it calls InstanceService
- THEN the mapped model `Default` field SHALL be false
- AND default selection SHALL follow `DEFAULT_KAFKA_INSTANCE` / `DEFAULT_RABBIT_INSTANCE` after Register/Update

### Requirement: Status conditions

Status SHALL expose `Ready` and `Stalled` only. `phase` is for kubectl.

#### Scenario: Condition reasons

- GIVEN ProcessCR finishes an apply
- WHEN it PATCHes status
- THEN reason SHALL be one of `InstanceRegistered`, `ForcedDefault`, `SecretError`, `HealthCheckFailed`,
  `InstanceInUse`, `InvalidSpec`, `DuplicateInstanceName`

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
- THEN the finalizer SHALL be removed
- AND the PG row SHALL remain

### Requirement: REST lock after operator write

Manager REST SHALL Update or Unregister an instance id only while `managed_by_operator` is false. After a CR Register or
adopt, REST of that id SHALL be rejected.

#### Scenario: Unmanaged REST row

- GIVEN a PG row never written by a CR
- WHEN manager REST Updates that id
- THEN the Update SHALL succeed

#### Scenario: Duplicate CR

- GIVEN `managed_by_operator` true and `origin_cr` is another CR
- WHEN a second CR would write that id
- THEN status SHALL be `Ready=False` `Stalled=True` reason `DuplicateInstanceName`
- AND ProcessCR SHALL NOT Update

### Requirement: Default instance params

The default Kafka and Rabbit instance SHALL be selected by Application params `DEFAULT_KAFKA_INSTANCE` /
`DEFAULT_RABBIT_INSTANCE` (value = CR `metadata.name`). There SHALL be no `spec.default` and no `DefaultInstance` CR.

#### Scenario: Empty params

- GIVEN the param for that kind is empty and PG has no default
- WHEN the first Register of that kind commits
- THEN that row SHALL become default (`ForcedDefault`)
- AND later reconciles SHALL NOT steal default just because they reconcile

#### Scenario: Named CR appears

- GIVEN the param is set to a CR `metadata.name` and that row exists and is not default
- WHEN ProcessCR runs
- THEN it SHALL `SetDefault` that id
- AND `status.isDefault` SHALL reflect PG

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
- THEN it SHALL Update that row, set `managed_by_operator` true, and store the CR namespace
- AND it SHALL NOT rename the id

### Requirement: Downgrade without Unregister

Syncing a MaaS version without the operator SHALL leave PG instance rows in place.

#### Scenario: Old chart

- GIVEN topics or vhosts use operator-created instance ids
- WHEN Argo CD Syncs a chart with no Watch, ProcessCR, or instance CRDs
- THEN those PG rows SHALL remain
- AND manager REST SHALL be the writer again
- AND no Unregister SHALL run as part of rollback
