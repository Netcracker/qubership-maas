# Delta for instance-operator

Operator runtime: Lease, Watch, claim, reconcile, Events, Helm enablement. CR field behavior is in `broker-instances`.

## ADDED Requirements

### Requirement: Operator enablement

The system SHALL run Watch and ProcessCR only when `OPERATOR_ENABLED` is true and this replica holds Lease
`maas-operator-leader` in `CLOUD_NAMESPACE`.

#### Scenario: Operator disabled

- GIVEN `OPERATOR_ENABLED` is false
- WHEN a client Register/Updates an instance through manager REST
- THEN the request SHALL succeed as today
- AND no Unregister SHALL run because the operator is off

#### Scenario: Operator enabled, non-leader replica

- GIVEN several `maas-service` replicas and `OPERATOR_ENABLED` is true
- WHEN HTTP traffic reaches a non-leader
- THEN that replica SHALL serve Fiber
- AND it SHALL NOT Watch instance CRs

### Requirement: Claim by operatorNamespace

ProcessCR SHALL apply a CR only when `spec.operatorNamespace` equals this MaaS `CLOUD_NAMESPACE`.

#### Scenario: Foreign claim

- GIVEN a `KafkaInstance` or `RabbitInstance` whose `spec.operatorNamespace` is another namespace
- WHEN this operator reconciles that key
- THEN it SHALL skip
- AND it SHALL NOT PATCH status
- AND it SHALL NOT Register or Update

### Requirement: Reconcile backoff

Transient apply failures SHALL retry with exponential workqueue backoff. Expected InUse SHALL use `RequeueAfter` without
that limiter. Permanent stall SHALL wait for the next Watch.

#### Scenario: Secret or health-check failure

- GIVEN a claimed CR whose Secret is missing or whose Register/Update health-check returns 400
- WHEN ProcessCR returns an error
- THEN the workqueue SHALL retry from a 1s base, doubling, cap 5m, with jitter
- AND a previous good PG row SHALL NOT be Unregistered because of `HealthCheckFailed`

#### Scenario: Instance still in use

- GIVEN Unregister returns 400 because topics or vhosts still reference the instance
- WHEN ProcessCR handles Terminating
- THEN it SHALL PATCH `Ready=False` reason `InstanceInUse` once
- AND it SHALL keep the finalizer
- AND it SHALL `RequeueAfter` 30s
- AND it SHALL NOT re-PATCH that reason on every retry while it is already `InstanceInUse`

#### Scenario: Invalid spec

- GIVEN `InvalidSpec` or `DuplicateInstanceName`
- WHEN ProcessCR finishes
- THEN `Stalled` SHALL be True
- AND the key SHALL NOT be requeued until a Watch (spec or Secret change)

### Requirement: Periodic resync

The leader SHALL reconcile each claimed instance CR on interval `MAAS_INSTANCE_RESYNC_INTERVAL` (default 10m) even when
spec and Secrets did not change.

#### Scenario: Skip apply

- GIVEN spec and Secrets unchanged, `Ready=True`, and this is not a resync
- WHEN a Watch fires
- THEN ProcessCR SHALL skip apply and keep status

### Requirement: Kubernetes Events

When `K8S_EVENTS_ENABLED` is true, ProcessCR SHALL emit a Kubernetes Event on the instance CR in the CR namespace, using
the same reason strings as status except `ForcedDefault`.

#### Scenario: Events disabled

- GIVEN `K8S_EVENTS_ENABLED` is false
- WHEN ProcessCR writes status
- THEN it SHALL NOT create Events
- AND the ClusterRole SHALL omit `events` create/patch

#### Scenario: Emit once per reason change

- GIVEN status reason is already `InstanceInUse`
- WHEN a 30s retry still sees Unregister 400
- THEN ProcessCR SHALL NOT emit another Event for that reason

#### Scenario: ForcedDefault is status-only

- GIVEN the first Register of a kind became the PG default
- WHEN ProcessCR PATCHes `ForcedDefault`
- THEN it SHALL NOT emit an Event for `ForcedDefault`

### Requirement: Restricted environment

When `restrictedEnvironment` is true, the chart SHALL create only namespaced objects (ServiceAccount, Lease Role). CRDs,
ClusterRole, and ClusterRoleBinding SHALL be applied out of band. Watch SHALL remain cluster-wide.

#### Scenario: Restricted env without cluster objects

- GIVEN `restrictedEnvironment` is true and cluster-scoped objects are missing
- WHEN the leader starts informers
- THEN instance kinds SHALL be unknown and Watch SHALL fail
- AND this SHALL NOT be implemented as “watch only `CLOUD_NAMESPACE`”

### Requirement: Prerequisites

Instance CRDs SHALL require Kubernetes 1.32 or newer (CEL validations and selectable fields on
`spec.operatorNamespace`).

#### Scenario: MaaS first

- GIVEN the MaaS Application has Synced CRDs and `maas-service`
- WHEN a later Application applies `KafkaInstance` / `RabbitInstance`
- THEN kinds exist and ProcessCR SHALL run on create
