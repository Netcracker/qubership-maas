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

Register vs Update is chosen by the unique URL / host set, not by Watch ADDED vs MODIFIED.

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

### Kafka: one address, protocol and SASL on the address, one admin and one client

`spec.address` is one listener: `{protocol, saslMechanism, bootstrapServers, caCertSecretRef}`. Admin and client share
it. `protocol` replaces REST `maasProtocol`. The mapper writes a **one-key** PG `addresses` map; REST-registered rows
keep whatever map they already have. `credentials.admin` and `credentials.client` are one object each, both required:
`{clientCert, credentialsSecretRef}`. The MaaS auth type (`plain`, `SCRAM`, `sslCert`, `sslCert+plain`,
`sslCert+SCRAM`) is derived from `address.protocol` + `saslMechanism` + `clientCert`. Secret refs are `{name}` only.
Secret.data keys are fixed by contract: Kafka SASL `username`/`password`, mTLS also `clientKey`/`clientCert`, CA
`ca.crt`; Rabbit `username`/`password` (mapper copies `username` into REST `User`). Secrets hold raw passwords; the
mapper adds the `plain:` prefix. `caCertSecretRef` is required for `SSL` / `SASL_SSL`.

CEL on `client` rejects `SASL_SSL` + `PLAIN` without `clientCert`, and TLS without `caCertSecretRef`.
`SASL_PLAINTEXT` + `PLAIN` is allowed so a later library fix can use it. REST rows with SASL on a `PLAINTEXT` key
keep working unchanged until a CR adopts them. A CR adopts `PLAINTEXT` + `SCRAM` as `SASL_PLAINTEXT` + `SCRAM` and
`PLAINTEXT` + `plain` as `SASL_PLAINTEXT` + `PLAIN`. Libraries
stay compatible with topic-response shape, but should be adapted to the new credential design (TLS and SASL from
protocol + mechanism + `clientCert`). `admin` is used only by MaaS (sarama), so it may use any pair. `admin` and
`client` both need a Secret or both do not. Because they share `spec.address`, the health check uses the same listener
as applications.

Adopting a REST row writes the CR's one listener and one client credential (the stored map becomes one key). Rejected:
lists of credentials; a free `type` per credential; key-mapping lists on Secret refs; several listeners in the CR;
`saslMechanism` on a `PLAINTEXT` protocol; rewriting topic-response `addresses` for REST rows that were not adopted.

Schema checks that would otherwise only fail at Register: `protocol` is an enum of the four Kafka protocols, the
credential rules above, Rabbit `apiURL` is `http(s)://`, `amqpURL` is `amqp(s)://`.

### CRDs generated from Go types, installed in zero wave

The Go CR types with `+kubebuilder` markers are the only source of the CRD schema. `controller-gen` writes raw YAML
into `crds/` (no Helm). Platform zero wave installs that folder **before** MaaS. The maas-service chart does not ship
CRDs. The files are never edited by hand, and CI fails on a stale copy. `docs/operator_design.md` keeps sample CRs and
field tables, not a CRD copy. Status conditions use `metav1.Condition`. The hand-written chart CRDs from section 2 are
removed when the generated files exist (section 4). Rejected: CRDs in the MaaS Helm chart (Helm ownership fights when
two MaaS installs share a cluster; CRDs must exist before MaaS starts); hand-written CRD YAML next to Go types.

### CR fields follow the Kubernetes API conventions, not the REST JSON

The CR is a new API, so it follows the conventions where REST differs; the mapper converts to the REST model, and
topic responses do not change.

- `spec.address` is one object `{protocol, saslMechanism, bootstrapServers, caCertSecretRef}`. The mapper writes a
  one-key REST `addresses` map. Rejected: a list of listeners in the CR (admin and client share one listener; the
  health check and applications must use the same one); the REST map as the CR field (cannot add `saslMechanism` /
  `caCertSecretRef` next to a map key).
- Default is a condition, `type: Default` (`Configured` | `ForcedDefault` | `NotDefault`), not `status.isDefault`.
  `Ready` answers whether the registration is applied; being the default is a separate fact, and `ForcedDefault` as a
  `Ready` reason hid the registration result. The `DEFAULT` printer column reads the condition. Rejected:
  `status.isDefault` (booleans do not take an `is` prefix, and it would keep `ForcedDefault` on `Ready`).
- Acronyms keep one case: Rabbit `apiURL`, `amqpURL`. Exception: `status.lastRequestId`, kept as it is.

### Application default params, not `spec.default`

`DEFAULT_KAFKA_INSTANCE` / `DEFAULT_RABBIT_INSTANCE` are the broker namespace: the `metadata.namespace` of the claimed
CR of that kind. Empty: first Register of that kind becomes default (`Default` condition reason `ForcedDefault`).
Named CR becomes default when it appears.
Instance mapper never sets `Default: true`. Rejected: `DefaultInstance` CR and `spec.default`; PG id (known before
registration only from the DB state, and may name a REST-only row). The param is that namespace.

While the operator is enabled and the param for a kind is set, manager REST SetDefault for that kind returns 409: the
operator would move the default back on the next reconcile, so the REST call would be silently undone. With the param
empty, REST SetDefault stays the way to pick a default, for CR-managed rows too. Rejected: lock SetDefault whenever a
managed row is involved (blocks the only default switch when the param is empty), and allow it always (silent revert).

### Optional operator and safe downgrade

`OPERATOR_ENABLED` default false. Flipping it false, or Syncing a chart without the operator, must not Unregister. CR
delete also must not Unregister: the PG row stays. Old binary ignores `cr_namespace` / `cr_name` (columns default empty).

### CR delete always keeps the PG row

There is no `deletionPolicy`. Deleting the CR always leaves the PG row and clears
`cr_namespace` and `cr_name` before removing the finalizer, so manager REST owns the row again. Topics and vhosts are
not a blocker. Manager REST Unregister still returns 400 if dependents remain. Finalizer
`maas.netcracker.com/instance` after successful Register, so ownership is cleared before the CR leaves etcd. Rejected:
Unregister on CR delete (drops topics/vhosts' instance); a `deletionPolicy` field (only one behavior).

### Status Ready, Stalled and Default; Events optional

`phase` is kubectl-only. Automate on conditions. Events in the CR namespace, same reasons as `Ready` / `Stalled`; the
`Default` condition is status-only. Do not emit on skip-apply, skip-claim, successful 10m resync, or a retry that
already has that reason.

### `restrictedEnvironment`

Chart skips ClusterRole; cluster-admin applies it out of band. CRDs are not in the chart (zero wave). Watch stays
cluster-wide. This is not “watch only the MaaS namespace”.

### v1 name ↔ PG id (from HLA)

`metadata.namespace` is the CR identity (new PG `id`, `cr_namespace`, default param). `metadata.name` is only the
Kubernetes object name; examples generate it equal to the namespace. Do not rename an existing PG `id` (topics/vhosts
and designators FK it).
The CR never carries the old REST id. Join is the unique URL / host set (Kafka bootstrap hosts as a set; Rabbit AMQP
host). In-cluster hosts are `service.namespace`; that namespace is part of the URL. External hosts skip the
URL-namespace check.

- One broker (or dedicated external) namespace holds one broker: at most one claimed `KafkaInstance` and one claimed
  `RabbitInstance` per namespace. Partial unique index `(cr_namespace) WHERE cr_namespace <> ''`. Owner matching uses
  `cr_namespace` only.
- A row already has `cr_namespace` equal to this CR namespace and `cr_name` equal to this CR → UPDATE that id (keep id),
  even if the URL / host set changed.
- A row already has `cr_namespace` equal to this CR namespace and a different `cr_name` → `DuplicateNamespace`.
- Else URL set not in PG → INSERT `id` = CR namespace, set `cr_namespace`.
- Else URL set in PG and `cr_namespace` empty or other: if the host looks like k8s and URL namespace ≠ CR namespace →
  `InvalidSpec`; else ADOPT (keep id, set `cr_namespace`). Empty `cr_namespace` does not
  match. Join-by-URL is only this first bind, not later edits of a row we already own.
- `MAAS_*_INSTANCE_BODY` still Registers legacy rows; it is not a namespace allowlist.
- Rejected: old REST id as `metadata.name`; `GetById(name)` adopt; DNS as PG id.

Applying the CR **is** the migrate. After `cr_namespace` is set, manager REST Update/Unregister of that id is
rejected while the operator is enabled.

### Rabbit URL uniqueness

A DB migration adds a unique index on `rabbit_instances.api_url` and one on `amqp_url`, like Kafka `addresses`. It
applies to manager REST and CRs: Register/Update with a URL used by another id fails with `23505`. Strings are compared
as stored. Existing duplicates fail the upgrade: the migration creates no index and `maas-service` exits at startup
with one error listing each shared URL and the instance ids using it. Ops keep one id per URL, then restart.

### Migrate back to REST

Deleting the CR clears `cr_namespace` and `cr_name` before removing the finalizer,
so manager REST owns the row again. `OPERATOR_ENABLED=false` disables the REST lock for all rows without changing them.

### Update path

Documented install/upgrade is Argo CD Application Sync, not `helm upgrade`.

### Reconcile backoff

Error (SecretError, HealthCheckFailed, apiserver/network): workqueue exponential, base 1s, cap 5m, 10% jitter.
Stalled (`InvalidSpec`, `DuplicateInstanceName`, `DuplicateNamespace`): wait for a CR change (spec or refresh
annotation). Resync claimed CRs every 10m.

## Rejected alternatives

| Option | Why not |
| -------- | --------- |
| Sibling operator pod | Extra process, REST hop, auth surface. |
| Watch only MaaS namespace | Instance CRs live with brokers. |
| ClusterRole for Secrets | Broader than needed; Secret Roles stay per CR namespace. |
| Secret Watch | Needs `list`/`watch` (read of every Secret in the namespace) and one informer per CR namespace. Rotation latency is covered by resync and the refresh annotation. |
| `spec.default` / DefaultInstance CR | Defaults are install params; one default per kind in PG. |
| Unregister when operator disabled | Breaks topics/vhosts on rollback. |
| Own-NS watch as restricted-env mode | Restricted env is out-of-band ClusterRole, not a smaller watch. |
| Key mapping lists on Secret refs | Secret.data keys are a contract. |
| Several listeners in the CR | Admin and client share one address. |
| `deletionPolicy` | CR delete always keeps the PG row. |
| CRDs in the MaaS Helm chart | Zero wave installs `crds/` before MaaS. |
| Old REST id on `metadata.name` | CR lives in the broker NS. Join is the URL set. Identity is the namespace. |

## Risks

- CRDs + finalizers on downgrade: a leftover finalizer blocks CR delete until removed or the CRD is deleted.
- Restricted env without out-of-band ClusterRole: informers fail.
- Instance CRs applied before zero-wave CRDs: apply fails (install CRDs first).
- Existing Rabbit rows sharing `api_url` or `amqp_url`: the upgrade stops (new pods fail at startup, the previous
  version keeps serving) until ops fix the rows named in the error.
- REST lock race (accepted): the controller reads `cr_namespace`, then the DAO writes in a separate transaction,
  so a REST Update/Unregister can land just after a CR adopts the row. Rare (REST change and adopt at the same moment)
  and the next reconcile restores the row from the CR. Closing it means checking ownership in the DAO transaction
  (`SELECT … FOR UPDATE`).
