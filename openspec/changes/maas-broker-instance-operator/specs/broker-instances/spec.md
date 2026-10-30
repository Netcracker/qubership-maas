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
- AND `cr_namespace` SHALL be the CR namespace on that row

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
REST SHALL keep resolved credentials, not SecretRef or `operatorNamespace`. Secret refs SHALL be `{name}` only.
Secret.data keys SHALL be fixed: Kafka SASL `username` / `password`, mTLS also `clientKey` / `clientCert`, CA `ca.crt`,
Rabbit `username` / `password`.

#### Scenario: Mapper does not set Default

- GIVEN ProcessCR maps a claimed CR
- WHEN it calls InstanceService
- THEN the mapped model `Default` field SHALL be false
- AND default selection SHALL follow `DEFAULT_KAFKA_INSTANCE` / `DEFAULT_RABBIT_INSTANCE` after Register/Update

#### Scenario: Rabbit Secret keys

- GIVEN a claimed `RabbitInstance` with `credentialsSecretRef.name` pointing at a Secret with `username` and `password`
- WHEN ProcessCR maps it
- THEN the REST `User` field SHALL be the Secret `username`
- AND the password SHALL be sent as `plain:<value>` from the raw Secret value

### Requirement: Kafka credentials

`KafkaInstance` SHALL have one `spec.address` shared by one `credentials.admin` and one `credentials.client`. Protocol
and `saslMechanism` SHALL live on `address`. Each credential SHALL be `{clientCert, credentialsSecretRef}`. The MaaS
auth type SHALL be derived from `address.protocol`, `saslMechanism` and `clientCert`. MaaS SHALL connect to
`address.protocol`.

#### Scenario: Admin and client map to the existing model

- GIVEN a claimed `KafkaInstance` with `address.protocol` `SASL_SSL`, `saslMechanism` `SCRAM`, and admin and client
  Secrets
- WHEN ProcessCR maps it
- THEN `MaasProtocol` SHALL be `SASL_SSL`
- AND `Credentials["admin"]` and `Credentials["client"]` SHALL each hold one `SCRAM` entry
- AND passwords SHALL be sent as `plain:<value>` from the raw Secret value
- AND the PG `addresses` map SHALL have that one protocol key

#### Scenario: Kafka without authentication

- GIVEN `address.protocol` `PLAINTEXT`, no `saslMechanism`, and admin and client with no `credentialsSecretRef`
- WHEN ProcessCR maps it
- THEN `Credentials` SHALL be empty and MaaS SHALL connect without authentication

#### Scenario: Credential that contradicts its address

- GIVEN `saslMechanism` on `PLAINTEXT` / `SSL` or missing on `SASL_*`, `clientCert` without TLS, `credentialsSecretRef`
  missing when the protocol needs a Secret, a Secret without the contract keys, or only one of admin and client
  needing a Secret
- WHEN the CR is applied
- THEN the API server SHALL reject it

### Requirement: Client library compatibility

Topic responses SHALL be built as before the operator, for CR-managed and REST-registered instances alike: the stored
`addresses` map, the client credential under its MaaS auth type, and `caCert`, with nothing added or removed. The CRD
SHALL reject `SASL_SSL` + `PLAIN` without `clientCert`, and TLS without `caCertSecretRef`, with a message naming a form
that works. `SASL_PLAINTEXT` + `PLAIN` SHALL be accepted (libraries may not connect until they are updated).

#### Scenario: SASL PLAIN without TLS

- GIVEN `address.protocol` `SASL_PLAINTEXT` and `saslMechanism: PLAIN`
- WHEN the CR is applied
- THEN the API server SHALL accept it

#### Scenario: SASL PLAIN over TLS without a client certificate

- GIVEN `address.protocol` `SASL_SSL`, `saslMechanism: PLAIN` and `clientCert: false`
- WHEN the CR is applied
- THEN the API server SHALL reject it, naming `clientCert: true` or `saslMechanism: SCRAM`

#### Scenario: TLS client without CA

- GIVEN `address.protocol` `SASL_SSL` and no `caCertSecretRef`
- WHEN the CR is applied
- THEN the API server SHALL reject it

#### Scenario: Legacy SASL on a PLAINTEXT key

- GIVEN a REST-registered instance with `addresses` with only `PLAINTEXT` and a `SCRAM` client credential
- WHEN an application gets a topic
- THEN the response SHALL hold `addresses.PLAINTEXT` and the `SCRAM` client credential, unchanged

#### Scenario: SASL PLAIN instance registered over REST

- GIVEN a REST-registered instance with `addresses` with only `SASL_PLAINTEXT` and a `plain` client credential
- WHEN an application gets a topic
- THEN the response SHALL hold exactly the stored `addresses` and the `plain` client credential

### Requirement: Kafka address

`spec.address` SHALL be one object `{protocol, saslMechanism, bootstrapServers, caCertSecretRef}` with `protocol` one
of `PLAINTEXT`, `SASL_PLAINTEXT`, `SSL`, `SASL_SSL`. The mapper SHALL write a one-key MaaS `addresses` map from it.
REST-registered rows SHALL keep their stored map until a CR adopts them. Topic responses SHALL return whatever is
stored. MaaS SHALL NOT rewrite `addresses` for REST rows that were not adopted.

#### Scenario: CR writes one listener

- GIVEN a claimed `KafkaInstance` with `address.protocol` `SASL_SSL` and `bootstrapServers`
- WHEN ProcessCR maps it
- THEN the PG `addresses` map SHALL contain only the `SASL_SSL` key
- AND `MaasProtocol` SHALL be `SASL_SSL`

#### Scenario: REST row with several listeners stays as stored

- GIVEN a REST-registered instance whose `addresses` map has `PLAINTEXT` and `SASL_SSL`
- WHEN an application gets a topic
- THEN the response SHALL hold both listeners, unchanged

### Requirement: Status conditions

Status SHALL expose the conditions `Ready`, `Stalled` and `Default` as `metav1.Condition`. `phase` is for kubectl.
`Default` SHALL be observed PG state, separate from `Ready`.

#### Scenario: Condition reasons

- GIVEN ProcessCR finishes an apply
- WHEN it PATCHes status
- THEN the `Ready` / `Stalled` reason SHALL be one of `InstanceRegistered`, `SecretError`, `HealthCheckFailed`,
  `InvalidSpec`, `DuplicateInstanceName`, `DuplicateNamespace`
- AND the `Default` reason SHALL be `Configured` (PG default and the namespace matches `DEFAULT_*_INSTANCE`),
  `ForcedDefault` (PG default, not named by the param) or `NotDefault` (another row is default)

#### Scenario: Default without a registered row

- GIVEN a CR whose first Register failed
- WHEN ProcessCR PATCHes status
- THEN status SHALL have no `Default` condition

### Requirement: Status alias is the PostgreSQL instance id

Status SHALL expose `alias` as the bound PostgreSQL instance id used by instance designators, topic/vhost `instanceId`,
and manager REST. ProcessCR SHALL set it once a row is bound, including when the id equals the CR namespace. It SHALL
NOT be on spec.

#### Scenario: New Register alias equals namespace

- GIVEN no PG row whose URL / host set equals the CR
- WHEN ProcessCR Registers with `id` equal to `metadata.namespace`
- THEN status SHALL have `alias` equal to that namespace

#### Scenario: Adopt keeps the REST id as alias

- GIVEN an unmanaged PG row whose id is `customer-external-kafka` and whose URL / host set equals the CR
- WHEN ProcessCR adopts that row
- THEN status SHALL have `alias` equal to `customer-external-kafka`
- AND `alias` SHALL NOT equal the CR namespace unless that was already the PG id

#### Scenario: First bind failed has no alias

- GIVEN a CR whose first Register failed
- WHEN ProcessCR PATCHes status
- THEN status SHALL have no `alias`

### Requirement: CR delete keeps the PostgreSQL row

While Terminating, ProcessCR SHALL keep the PG row, clear `cr_namespace` and
`cr_name`, then remove the finalizer. There SHALL be no `deletionPolicy`. Topics and vhosts SHALL NOT block CR delete.
Manager REST Unregister SHALL still return 400 when dependents remain.

#### Scenario: Delete hands the row back to REST

- GIVEN a claimed CR whose PG row exists, with or without topics or vhosts
- WHEN the CR is deleted
- THEN `cr_namespace` and `cr_name` SHALL be cleared on the row
- AND the finalizer SHALL be removed
- AND the PG row SHALL remain
- AND manager REST SHALL be able to Update or Unregister that id

### Requirement: REST lock after operator write

While `OPERATOR_ENABLED` is true, manager REST SHALL Update or Unregister an instance id only while
`cr_namespace` is empty. After a CR Register or adopt, REST of that id SHALL be rejected. While `OPERATOR_ENABLED`
is false, the lock SHALL NOT be enforced.

#### Scenario: Unmanaged REST row

- GIVEN a PG row never written by a CR
- WHEN manager REST Updates that id
- THEN the Update SHALL succeed

#### Scenario: Duplicate CR

- GIVEN `cr_namespace` is `kafka-infra`
- AND a claimed CR in namespace `team-foo` with the same in-cluster host set
- WHEN ProcessCR applies
- THEN status SHALL be `Ready=False` `Stalled=True` reason `InvalidSpec`
- AND ProcessCR SHALL NOT Update

### Requirement: One broker per namespace

A broker namespace SHALL hold at most one claimed `KafkaInstance` and at most one claimed `RabbitInstance`. PG SHALL
enforce it with a partial unique index on `cr_namespace` over rows where `cr_namespace` is not empty. Kafka and Rabbit
SHALL be independent.

#### Scenario: Second CR of a kind in one namespace

- GIVEN a row with `cr_namespace` X owned by CR X/A
- WHEN ProcessCR applies claimed CR X/B of the same kind
- THEN status SHALL be `Ready=False` `Stalled=True` reason `DuplicateNamespace`, and the message SHALL name CR X/A
- AND ProcessCR SHALL NOT Register or Update

#### Scenario: Deleting the owner CR frees the namespace

- GIVEN CR X/A was deleted
- WHEN ProcessCR applies claimed CR X/B of the same kind
- THEN the namespace rule SHALL NOT reject it

#### Scenario: REST lock off while operator disabled

- GIVEN `OPERATOR_ENABLED` is false and a row has `cr_namespace` set
- WHEN manager REST Updates that id
- THEN the Update SHALL succeed
- AND `cr_namespace` SHALL stay set

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
- THEN that row SHALL become default, with `Default` reason `ForcedDefault`
- AND later reconciles SHALL NOT steal default just because they reconcile

#### Scenario: Named CR appears

- GIVEN the param equals the namespace of a claimed CR whose row exists and is not default
- WHEN ProcessCR runs
- THEN it SHALL `SetDefault` that row's id
- AND its `Default` condition SHALL be `True` with reason `Configured`

#### Scenario: Same CR name in two namespaces

- GIVEN claimed Kafka CRs `ns-a/ns-a` and `ns-b/ns-b`, and `DEFAULT_KAFKA_INSTANCE` is `ns-a`
- WHEN ProcessCR reconciles both CRs, including later resyncs
- THEN the `ns-a` row SHALL be default and SHALL stay default
- AND ProcessCR of `ns-b/ns-b` SHALL NOT `SetDefault`

#### Scenario: Clearing the param

- GIVEN a PG default already exists
- WHEN the Application param is later cleared
- THEN ProcessCR SHALL NOT unset the PG default

### Requirement: Name and id

ProcessCR SHALL use `metadata.namespace` as the CR identity (`cr_namespace`, and PG `id` for a new row).
`metadata.name` is the Kubernetes object name only. ProcessCR SHALL NOT use it as a join key or PG id, and SHALL NOT
rename an existing PG `id`.

#### Scenario: New URL

- GIVEN no PG row whose URL / host set equals the CR
- WHEN ProcessCR applies
- THEN it SHALL Register with PG `id` equal to `metadata.namespace`
- AND `cr_namespace` SHALL be that namespace
- AND `status.alias` SHALL equal that namespace

#### Scenario: Same owner updates

- GIVEN a PG row whose `cr_namespace` equals the CR namespace and `cr_name` equals the CR name
- WHEN ProcessCR applies
- THEN it SHALL Update that row and keep the id
- AND it SHALL do so even when the CR URL / host set differs from the row

#### Scenario: Same owner changes URL

- GIVEN a claimed CR in namespace `kafka-infra` whose PG row has bootstrap host `kafka-old:9092`
- AND the CR spec now has bootstrap host `kafka-new:9092`
- WHEN ProcessCR applies
- THEN it SHALL Update that row with the new host set and keep the id
- AND status SHALL NOT be `DuplicateNamespace`

#### Scenario: Adopt unmanaged row when URL namespace matches

- GIVEN an unmanaged PG row with bootstrap host `kafka.kafka-infra:9093` and a claimed CR in namespace `kafka-infra`
  with the same host set
- WHEN ProcessCR applies
- THEN it SHALL Update that row and set `cr_namespace` to `kafka-infra`
- AND it SHALL NOT rename the id
- AND `status.alias` SHALL equal that kept id

### Requirement: Join by URL

ProcessCR SHALL find a PG row by equal Kafka host set or Rabbit AMQP URL, not by `GetById` of the CR name.
Kafka bootstrap hosts SHALL be compared as a set (port stripped, order ignored). Empty `cr_namespace` SHALL NOT count
as a match. ProcessCR SHALL NOT compare a host DNS label to the CR namespace.

#### Scenario: Adopt unmanaged in-cluster URL from another namespace

- GIVEN an unmanaged PG row with bootstrap host `kafka.kafka-infra:9093`
- AND a claimed CR in namespace `team-foo` with the same host set
- WHEN ProcessCR applies
- THEN it SHALL adopt that row (keep id, set `cr_namespace` to `team-foo`)

#### Scenario: Adopt external URL without a namespace in the host

- GIVEN an unmanaged PG row with bootstrap host `kafka.prod.example.com:9093` and a claimed CR in namespace `ext-kafka`
  with the same host set
- WHEN ProcessCR applies
- THEN it SHALL adopt that row (keep id, set `cr_namespace`)

### Requirement: Downgrade without Unregister

Syncing a MaaS version without the operator SHALL leave PG instance rows in place.

#### Scenario: Old chart

- GIVEN topics or vhosts use operator-created instance ids
- WHEN Argo CD Syncs a chart with no Watch or ProcessCR
- THEN those PG rows SHALL remain
- AND manager REST SHALL be the writer again
- AND no Unregister SHALL run as part of rollback
