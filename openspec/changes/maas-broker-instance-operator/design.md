# Design: MaaS broker-instance operator

Architecture diagrams, CRD YAML, and mermaid stay in [docs/operator_design.md](../../../docs/operator_design.md). This file is the decision record.

## Technical approach

ProcessCR runs in the same `maas-service` process as Fiber. A Kubernetes Lease `maas-operator-leader` in `CLOUD_NAMESPACE` elects the watcher. HPA still scales HTTP. Only the leader Watches instance CRs (cluster-wide) and referenced Secrets (namespaced Roles). Apply calls existing `KafkaInstanceService` / `RabbitInstanceService` with a mapped `model.KafkaInstance` / `RabbitInstance` (resolved credentials, never Secret names). PostgreSQL stays the source of runtime instance ids for topics/vhosts.

Register vs Update is chosen by `GetById`, not by Watch ADDED vs MODIFIED.

## Decisions

### Single process + Lease, not a sidecar operator

One Deployment. No extra basic-auth/M2M hop. Several replicas may serve HTTP; only the Lease holder Watches. Several watchers on the same CRs would dual-Register.

### Cluster Watch + `spec.operatorNamespace` claim

CRs may live in broker namespaces. Claim is immutable `spec.operatorNamespace == CLOUD_NAMESPACE`. Mismatch: skip, no status PATCH, no Register. Rejected: watch-only-own-NS, static watch-list, namespace annotation (see design notes).

### Namespaced Secret Roles, not ClusterRole `secrets`

ClusterRole: instance CRs `get/list/watch`, status PATCH, finalizers; `events` create/patch when `K8S_EVENTS_ENABLED`. No `secrets`. Each CR namespace grants Secret `get/watch` via Role + RoleBinding. v1: Secret is in the same namespace as the CR (`secretRef.namespace` out of scope).

### Application default params, not `spec.default`

`DEFAULT_KAFKA_INSTANCE` / `DEFAULT_RABBIT_INSTANCE` are CR `metadata.name`. Empty: first Register of that kind is `ForcedDefault`. Named CR becomes default when it appears. Instance mapper never sets `Default: true`. Rejected: `DefaultInstance` CR and `spec.default`.

### Optional operator and safe downgrade

`OPERATOR_ENABLED` default false. Flipping it false, or Syncing a chart without the operator, must not Unregister. Only CR deletion with `deletionPolicy: Unregister` deletes the PG row. Old binary ignores `managed_by_operator` (column defaults false).

### `deletionPolicy` Unregister (default) vs Orphan

Unregister: drop CR and PG row (400 + `InstanceInUse` if topics/vhosts remain; keep finalizer; `RequeueAfter 30s`). Orphan: drop CR, keep row. Finalizer `maas.netcracker.com/instance` after successful Register.

### Status Ready + Stalled; Events optional

`phase` is kubectl-only. Automate on conditions. Events in the CR namespace, same reasons as status except `ForcedDefault` (status-only). Do not emit on skip-apply, skip-claim, successful 10m resync, or a retry that already has that reason.

### `restrictedEnvironment`

Chart skips CRDs and ClusterRole; cluster-admin applies them out of band. Watch stays cluster-wide. This is not “watch only the MaaS namespace”.

### v1 name ↔ PG id (from HLA)

Do not rename an existing PG `id` (topics/vhosts FK it).

- New instance: PG `id` = CR `metadata.namespace`. Prefer `metadata.name` equal to the namespace.
- `GetById(namespace)` hit → Update/adopt that row.
- Else if `metadata.name` ≠ namespace and `GetById(name)` hits an unmanaged row → adopt (set `managed_by_operator`, store CR namespace).
- Same name already managed from another namespace → `DuplicateInstanceName`, do not Update.
- Kafka `addresses` unique (`23505`) is the “forgot old id” net. Rabbit has no URL unique in v1.

Applying the CR **is** the migrate. No `spec.takeOver` in v1. After `managed_by_operator` true, manager REST Update/Unregister of that id is rejected.

### Update path

Documented install/upgrade is Argo CD Application Sync, not `helm upgrade`.

### Reconcile backoff

Error (SecretError, HealthCheckFailed, apiserver/network): workqueue exponential, base 1s, cap 5m, 10% jitter. `InstanceInUse`: `RequeueAfter 30s`, not an error. Stalled (`InvalidSpec`, `DuplicateInstanceName`): wait for Watch. Resync claimed CRs every 10m.

## Rejected alternatives

| Option | Why not |
|--------|---------|
| Sibling operator pod | Extra process, REST hop, auth surface. |
| Watch only MaaS namespace | Instance CRs live with brokers. |
| ClusterRole for Secrets | Broader than needed; Secret Roles stay per CR namespace. |
| `spec.default` / DefaultInstance CR | Defaults are install params; one default per kind in PG. |
| Unregister when operator disabled | Breaks topics/vhosts on rollback. |
| Own-NS watch as restricted-env mode | Restricted env is out-of-band CRDs/ClusterRole, not a smaller watch. |

## Open questions (v1 non-goals unless SPEC PR closes them)

| Question | Owner until closed | v1 stance |
|----------|--------------------|-----------|
| Blue/Green adopt onto a new operator (`operatorNamespace`, finalizers, `origin_cr`) | SPEC review | Non-goal |
| Rabbit URL uniqueness (like Kafka addresses) | SPEC review | Non-goal; Kafka `23505` stays |
| Migrate back: Orphan sets `managed_by_operator` false vs REST-only after disable | SPEC review | Orphan keeps the row; lock behavior after Orphan is still TODO |
| `spec.takeOver` to refuse auto-adopt | Later | v1 auto-adopts |

## Risks

- CRDs + finalizers on downgrade: a leftover finalizer blocks CR delete until removed or the CRD is deleted.
- Restricted env without out-of-band ClusterRole: informers fail.
- Instance CRs applied before MaaS CRDs: apply fails (install MaaS first).
