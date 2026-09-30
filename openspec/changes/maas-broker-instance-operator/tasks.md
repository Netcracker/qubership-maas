# Tasks

Implementation starts after this SPEC PR merges. Do not check these off from design-only work.

## 1. SPEC PR (this change)

- [ ] 1.1 File or update Jira with AC matching proposal.md (success, errors, compatibility)
- [ ] 1.2 Review delta specs vs [docs/operator_design.md](../../../docs/operator_design.md)
- [ ] 1.3 Close or confirm v1 non-goals: BG adopt, Rabbit URL uniqueness, `spec.takeOver`, Orphan lock
- [ ] 1.4 Merge SPEC PR (openspec/ + HLA). No ProcessCR code in that PR

## 2. CRDs and chart

- [ ] 2.1 Add `KafkaInstance` / `RabbitInstance` CRDs (`maas.netcracker.com/v1`, short names, printer columns, CEL
      immutability on `operatorNamespace`, selectableFields, Kubernetes 1.32+)
- [ ] 2.2 Helm values: `OPERATOR_ENABLED` (default false), `DEFAULT_KAFKA_INSTANCE`, `DEFAULT_RABBIT_INSTANCE`,
      `K8S_EVENTS_ENABLED` (default true), `restrictedEnvironment` (default false)
- [ ] 2.3 ClusterRole: instance CRs watch/status/finalizers; no `secrets`; events create/patch when events enabled
- [ ] 2.4 Namespaced Lease Role for `maas-operator-leader`; document per-namespace Secret Role + RoleBinding
- [ ] 2.5 `restrictedEnvironment: true` skips CRDs and ClusterRole in-chart

## 3. ProcessCR runtime

- [ ] 3.1 Lease election; only leader Watches; Fiber stays on all replicas
- [ ] 3.2 Cluster Watch of instance CRs; skip when `operatorNamespace` ≠ `CLOUD_NAMESPACE` (no PATCH)
- [ ] 3.3 Namespaced Secret Watch; enqueue CRs on Secret `resourceVersion` change
- [ ] 3.4 Mapper CR + Secrets → `model.KafkaInstance` / `RabbitInstance` (`Default: false`); call InstanceService
- [ ] 3.5 Finalizer `maas.netcracker.com/instance`; `deletionPolicy` Unregister/Orphan; `InstanceInUse` + RequeueAfter
      30s
- [ ] 3.6 Status `Ready` + `Stalled` and reasons from the broker-instances spec
- [ ] 3.7 Events in CR namespace; honor `K8S_EVENTS_ENABLED`; no Event for `ForcedDefault` or unchanged reason
- [ ] 3.8 Backoff (error 1s→5m) and 10m resync; skip apply when Ready and unchanged

## 4. Persistence and REST

- [ ] 4.1 PG columns `managed_by_operator`, `namespace`, `origin_cr`; existing rows default unmanaged
- [ ] 4.2 Name/id mapping from design.md (id = namespace; adopt by namespace or old REST id; never rename id)
- [ ] 4.3 Reject manager REST Update/Unregister when `managed_by_operator` is true
- [ ] 4.4 Default params `DEFAULT_*_INSTANCE` and `ForcedDefault` rules from the spec
- [ ] 4.5 `OPERATOR_ENABLED=false` and old-chart downgrade do not Unregister

## 5. Verify and archive

- [ ] 5.1 Unit tests for claim skip, SecretError, HealthCheckFailed, InstanceInUse, DuplicateInstanceName, REST lock,
      default params
- [ ] 5.2 `/opsx:verify` against tasks, specs, and Jira AC
- [ ] 5.3 `/opsx:archive` in the completing Code PR (sync main specs, move change to archive)
- [ ] 5.4 Product docs: point operators at CRDs/values; keep HLA for architecture
