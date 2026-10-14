# Design: MaaS broker-instance operator

Architecture diagrams, CRD YAML, and mermaid stay in [docs/operator_design.md](../../../docs/operator_design.md). This
file is the decision record.

## Technical approach

ProcessCR runs in the same `maas-service` process as Fiber. A Kubernetes Lease `maas-operator-leader` in
`CLOUD_NAMESPACE` elects the watcher. HPA still scales HTTP. Only the leader Watches instance CRs (cluster-wide) and
reads referenced Secrets on every reconcile (namespaced Roles, no Secret Watch). Apply calls existing
`KafkaInstanceService` / `RabbitInstanceService` with a mapped `model.KafkaInstance` / `RabbitInstance` (resolved
credentials, never Secret names). PostgreSQL stays the source
of runtime instance ids for topics/vhosts.

Register vs Update is chosen by `GetById`, not by Watch ADDED vs MODIFIED.

## Decisions

### Single process + Lease, not a sidecar operator

One Deployment. No extra basic-auth/M2M hop. Several replicas may serve HTTP; only the Lease holder Watches. Several
watchers on the same CRs would dual-Register.

### Cluster Watch + `spec.operatorNamespace` claim

CRs may live in broker namespaces. Claim is immutable `spec.operatorNamespace == CLOUD_NAMESPACE`. Mismatch: skip, no
status PATCH, no Register. Rejected: watch-only-own-NS, static watch-list, namespace annotation (see design notes).

### Namespaced Secret Roles, not ClusterRole `secrets`

ClusterRole: instance CRs `get/list/watch/patch` (patch for the finalizer), status PATCH; `events` create/patch when
`K8S_EVENTS_ENABLED`. No `secrets`. Each CR namespace grants Secret `get` via Role + RoleBinding.
v1: Secret is in the same namespace as the CR (`secretRef.namespace` out of scope).

### Read Secrets on reconcile, no Secret Watch

Secrets are read with Get on every reconcile. A Secret change alone does not enqueue the CR. Rotation is picked up by
the 10m resync, any CR change, or the `maas.netcracker.com/refresh` annotation. `status.secretRevisions` holds the
`resourceVersion`s from the last apply; a different revision means MaaS Update. The CR informer must not filter on
`generation` only, because the annotation does not bump it.

### Kafka credentials: one admin, one client, each tied to a listener

`spec.credentials.admin` and `spec.credentials.client` are one object each, both required. A credential is
`{protocol, saslMechanism, clientCert, credentialsSecretRef}`. `protocol` must be a key of `addresses`, so the
credential names the listener it is for. `admin.protocol` replaces REST `maasProtocol`: MaaS connects to
`addresses[admin.protocol]` with the admin credential. The MaaS auth type (`plain`, `SCRAM`, `sslCert`, `sslCert+plain`,
`sslCert+SCRAM`) is derived from `protocol` + `saslMechanism` + `clientCert`, so a credential cannot contradict its
listener. Secret references follow dbaas-operator `ExternalDatabase`: `credentialsSecretRef {name, keys[{key, name}]}`
(`CredentialsSecretRef` / `SecretKeyMapping`), so existing Secrets work with any key names; CEL checks that the fields
the credential needs are mapped. Rabbit uses the same type, `caCertSecretRef` is `{name, key}`. Secrets hold raw
passwords; the mapper adds the `plain:` prefix.

Backward compatibility with existing client libraries (Go and Java): topic responses keep their format (whole
`addresses` map, client credential under its auth type, `caCert`); only the CR shape is new. The libraries accept a fixed
set of address key and credential pairs. They read SASL PLAIN without TLS only as a `PLAINTEXT` key with a `plain`
credential and have no `SASL_PLAINTEXT` + `plain` pair. The CR keeps protocols honest (`saslMechanism` is forbidden on
`PLAINTEXT`), and MaaS bridges the gap in the topic response: when the client credential is `plain` on a
`SASL_PLAINTEXT` listener and there is no `PLAINTEXT` key, the response also lists that listener under `PLAINTEXT`. The
alias is built on a copy in `TopicRegistration.ToResponseDto`, never stored, and applies to REST rows too; no client
library changes. CEL on `client` rejects what still cannot work: `SASL_PLAINTEXT` + `PLAIN` while `addresses` has a
real `PLAINTEXT` key (no alias, and the libraries would send SASL to that listener), `SASL_SSL` + `PLAIN` without
`clientCert` (no library reads it), and a client on `SSL` / `SASL_SSL` without `caCertSecretRef`. REST rows with SASL
on a `PLAINTEXT` key keep working unchanged; a CR adopting one writes `SASL_PLAINTEXT`. `admin` is used only by
MaaS (sarama), so it may use any pair. `admin` and `client` both need a Secret or both do not: the health check tries
the client credential on `addresses[admin.protocol]`, so a client-only credential fails Register, and manager REST
validation (which applies again after Orphan) accepts both roles or neither. Mixed setups would need the health check
to use `client.protocol`; later.

Applications still pick the listener by library priority, not by `client.protocol`; MaaS does not filter `addresses`
per credential. Operators list only the listeners applications may use. Adopting a REST row with several client
credentials keeps the one in the CR. Rejected: lists of credentials (several client entries existed only so libraries
could pick); a free `type` per credential (could contradict the listener); fixed Secret key names (existing Secrets,
for example from a Kafka operator, use other names; dbaas-operator maps keys too); `saslMechanism` on a `PLAINTEXT`
key in the CR (a protocol that contradicts its listener, only to fit the libraries; the response alias does that);
filtering
`addresses` by `client.protocol` in topic responses (changes the response for CR-managed instances only).

Schema checks that would otherwise only fail at Register: credential protocols are keys of `addresses`, address keys
are the four Kafka protocols, the credential rules above, Rabbit `apiUrl` is `http(s)://`, `amqpUrl` is `amqp(s)://`.

### CRDs generated from Go types

The Go CR types with `+kubebuilder` markers are the only source of the CRD schema. `controller-gen` writes
`config/crd/bases`, and `make sync-helm-crds` wraps each CRD in the chart condition into
`helm-templates/maas-service/templates/crd-*.yaml`, the same way as dbaas-operator. The templates are never edited by
hand, and CI fails on a stale copy. `docs/operator_design.md` keeps sample CRs and field tables, not a CRD copy.
Status conditions use `metav1.Condition`, so `type`, `reason`, `message` and `observedGeneration` get the upstream
validation. The hand-written CRDs from section 2 stay until the Go types exist (section 4). Rejected: hand-written CRD
YAML next to Go types (two schemas drift; the Kafka credential entry and conditions were already copied by hand).

### Application default params, not `spec.default`

`DEFAULT_KAFKA_INSTANCE` / `DEFAULT_RABBIT_INSTANCE` are the broker namespace: the `metadata.namespace` of the claimed
CR of that kind. Empty: first Register of that kind is `ForcedDefault`. Named CR becomes default when it appears.
Instance mapper never sets `Default: true`. Rejected: `DefaultInstance` CR and `spec.default`; CR `metadata.name`
(unique only inside a namespace, so two claimed CRs with one name in two namespaces would both match and the default
would flip on every resync); PG id (known before registration only from the DB state, and may name a REST-only row);
`namespace/name` (longer, and the one-broker-per-namespace rule already makes the namespace unique).

While the operator is enabled and the param for a kind is set, manager REST SetDefault for that kind returns 409: the
operator would move the default back on the next reconcile, so the REST call would be silently undone. With the param
empty, REST SetDefault stays the way to pick a default, for CR-managed rows too. Rejected: lock SetDefault whenever a
managed row is involved (blocks the only default switch when the param is empty), and allow it always (silent revert).

### Optional operator and safe downgrade

`OPERATOR_ENABLED` default false. Flipping it false, or Syncing a chart without the operator, must not Unregister. Only
CR deletion with `deletionPolicy: Unregister` deletes the PG row. Old binary ignores `managed_by_operator` (column
defaults false).

### `deletionPolicy` Unregister (default) vs Orphan

Unregister: drop CR and PG row (400 + `InstanceInUse` if topics/vhosts remain; keep finalizer; `RequeueAfter 30s`).
Orphan: drop CR, keep row, hand it back to REST. Finalizer `maas.netcracker.com/instance` after successful Register.

### Status Ready + Stalled; Events optional

`phase` is kubectl-only. Automate on conditions. Events in the CR namespace, same reasons as status except
`ForcedDefault` (status-only). Do not emit on skip-apply, skip-claim, successful 10m resync, or a retry that already has
that reason.

### `restrictedEnvironment`

Chart skips CRDs and ClusterRole; cluster-admin applies them out of band. Watch stays cluster-wide. This is not “watch
only the MaaS namespace”.

### v1 name ↔ PG id (from HLA)

Do not rename an existing PG `id` (topics/vhosts FK it).

- One broker namespace holds one broker: at most one claimed `KafkaInstance` and one claimed `RabbitInstance` per
  namespace. A partial unique index `(cr_namespace) WHERE managed_by_operator` on each table enforces it. ProcessCR
  checks it first; a second CR of that kind in the namespace gets `DuplicateNamespace` (message names the owning CR)
  and is not applied. Orphan clears `managed_by_operator`, so it frees the namespace.
- New instance: PG `id` = CR `metadata.namespace`. Prefer `metadata.name` equal to the namespace.
- `GetById(namespace)` hit → Update/adopt that row.
- Else if `metadata.name` ≠ namespace and `GetById(name)` hits an unmanaged row → adopt (set `managed_by_operator`,
  store CR namespace and name).
- Same name already managed from another namespace → `DuplicateInstanceName`, do not Update.
- Kafka `addresses` and Rabbit `api_url` / `amqp_url` unique (`23505`) are the “forgot old id” net.
- Owner columns: `cr_namespace` = CR `metadata.namespace`, `cr_name` = CR `metadata.name`; together they identify the
  owning CR. Not `origin_cr` with `namespace/name` (would repeat the namespace) and not a bare `namespace` column (reads
  like a topic/vhost classifier namespace).

Applying the CR **is** the migrate. After `managed_by_operator` true, manager REST Update/Unregister of that id is
rejected while the operator is enabled.

### Rabbit URL uniqueness

A DB migration adds a unique index on `rabbit_instances.api_url` and one on `amqp_url`, like Kafka `addresses`. It
applies to manager REST and CRs: Register/Update with a URL used by another id fails with `23505`. Strings are compared
as stored. Existing duplicates fail the upgrade: the migration creates no index and `maas-service` exits at startup
with one error listing each shared URL and the instance ids using it. Ops keep one id per URL, then restart.

### Migrate back to REST

`deletionPolicy: Orphan` sets `managed_by_operator` false and clears `cr_namespace` and `cr_name` before removing the
finalizer, so manager REST owns the row again. `OPERATOR_ENABLED=false` disables the REST lock for all rows without
changing them.

### Update path

Documented install/upgrade is Argo CD Application Sync, not `helm upgrade`.

### Reconcile backoff

Error (SecretError, HealthCheckFailed, apiserver/network): workqueue exponential, base 1s, cap 5m, 10% jitter.
`InstanceInUse`: `RequeueAfter 30s`, not an error. Stalled (`InvalidSpec`, `DuplicateInstanceName`,
`DuplicateNamespace`): wait for a CR change (spec or refresh annotation).
Resync claimed CRs every 10m.

## Rejected alternatives

| Option | Why not |
| -------- | --------- |
| Sibling operator pod | Extra process, REST hop, auth surface. |
| Watch only MaaS namespace | Instance CRs live with brokers. |
| ClusterRole for Secrets | Broader than needed; Secret Roles stay per CR namespace. |
| Secret Watch | Needs `list`/`watch` (read of every Secret in the namespace) and one informer per CR namespace. Rotation latency is covered by resync and the refresh annotation. |
| `spec.default` / DefaultInstance CR | Defaults are install params; one default per kind in PG. |
| Unregister when operator disabled | Breaks topics/vhosts on rollback. |
| Own-NS watch as restricted-env mode | Restricted env is out-of-band CRDs/ClusterRole, not a smaller watch. |

## Risks

- CRDs + finalizers on downgrade: a leftover finalizer blocks CR delete until removed or the CRD is deleted.
- Restricted env without out-of-band ClusterRole: informers fail.
- Instance CRs applied before MaaS CRDs: apply fails (install MaaS first).
- Existing Rabbit rows sharing `api_url` or `amqp_url`: the upgrade stops (new pods fail at startup, the previous
  version keeps serving) until ops fix the rows named in the error.
- REST lock race (accepted): the controller reads `managed_by_operator`, then the DAO writes in a separate transaction,
  so a REST Update/Unregister can land just after a CR adopts the row. Rare (REST change and adopt at the same moment)
  and the next reconcile restores the row from the CR. Closing it means checking ownership in the DAO transaction
  (`SELECT … FOR UPDATE`).
