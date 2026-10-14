# Tasks

Implementation starts after this SPEC PR merges. Do not check these off from design-only work.

One section per Code PR, in merge order. Each PR ships the unit tests for its own code and keeps `OPERATOR_ENABLED`
default false, so `main` stays releasable after every merge.

## 1. SPEC PR (this change)

- [x] 1.1 File or update Jira with AC matching proposal.md (success, errors, compatibility)
- [x] 1.2 Close open questions: Rabbit URL uniqueness in scope; Orphan hands the row back to REST; `spec.takeOver` and
      Blue/Green adopt dropped
- [x] 1.3 Merge SPEC PR (openspec/ + HLA). No ProcessCR code in that PR

## 2. CRDs and chart

- [x] 2.1 Add `KafkaInstance` / `RabbitInstance` CRDs (`maas.netcracker.com/v1`, short names, printer columns, CEL
      immutability on `operatorNamespace`, selectableFields, Kubernetes 1.32+, Kafka `credentials.admin` /
      `credentials.client`, CEL checks on addresses and credential keys, Rabbit URL schemes)
- [x] 2.2 Helm values: `OPERATOR_ENABLED` (default false), `DEFAULT_KAFKA_INSTANCE`, `DEFAULT_RABBIT_INSTANCE`,
      `K8S_EVENTS_ENABLED` (default true), `restrictedEnvironment` (default false). `DEFAULT_*_INSTANCE` is the broker
      namespace; `values.schema.json` allows empty or a DNS-1123 label. The schema had every property nested one level
      too deep, so Helm validated nothing; flattened it, with string-or-boolean / string-or-integer for values that only
      become env text (`DB_POSTGRESQL_TLS_*`, `DB_POOL_SIZE`) and strict booleans for values used in `if`
- [x] 2.3 ClusterRole: instance CRs watch/status/finalizers; no `secrets`; events create/patch when events enabled
- [x] 2.4 Namespaced Lease Role for `maas-operator-leader`; document per-namespace Secret Role + RoleBinding
- [x] 2.5 `restrictedEnvironment: true` skips CRDs and ClusterRole in-chart

## 3. Persistence and REST lock (no Kubernetes dependencies)

- [x] 3.1 Migration: PG columns `managed_by_operator` (default false), `cr_namespace`, `cr_name` on Kafka and Rabbit
      instance tables; existing rows stay unmanaged
- [x] 3.2 Migration: unique index on `rabbit_instances.api_url` and on `amqp_url`. If rows already share a URL, create
      nothing and stop startup with one error listing each shared URL and its instance ids
- [x] 3.3 Map the Rabbit unique violation to a REST error (409, as Kafka `addresses` and duplicate ids); document it in
      `docs/rest_api.md` and the fix in `docs/troubleshooting.md`
- [x] 3.4 `OPERATOR_ENABLED` in `application.yaml` (env override, default false)
- [x] 3.5 Reject manager REST Update/Unregister when `managed_by_operator` is true and `OPERATOR_ENABLED` is true;
      allow it when the operator is disabled
- [x] 3.6 Old binary and new rows: columns have defaults, so a downgrade reads them as unmanaged and Unregisters nothing
- [x] 3.7 Unit tests: migrations (fresh DB, existing rows, duplicate Rabbit URLs), REST lock on and off
- [x] 3.8 Reject manager REST SetDefault (409) for a kind while `OPERATOR_ENABLED` is true and that kind's
      `DEFAULT_*_INSTANCE` param is set; allow it otherwise; document in `docs/rest_api.md`; unit tests for param set,
      param empty, and operator disabled
- [x] 3.9 Migration 31: partial unique index on `cr_namespace` where `managed_by_operator` for Kafka and Rabbit (one
      claimed CR of each kind per broker namespace); unit test that a second managed row in one namespace fails and an
      unmanaged one does not

## 4. Operator skeleton (watch and claim, no writes to MaaS)

- [ ] 4.1 Add `sigs.k8s.io/controller-runtime` (with client-go); `go mod tidy` and `go mod vendor`
- [ ] 4.2 Go CR types for `KafkaInstance` / `RabbitInstance` (`api/v1` package), registered in a runtime scheme. The
      types are the source of the CRD schema: `+kubebuilder` markers carry descriptions, CEL rules, enums, short names,
      printer columns, selectable fields, the status subresource and the `helm.sh/resource-policy: keep` /
      `argocd.argoproj.io/sync-options: Prune=false` annotations. One credential entry type for Kafka `admin` and
      `client`; status as a shared `OperatorStatus` with `[]metav1.Condition`. The top-level description says new
      instances get PG id = CR namespace, not the CR name
- [ ] 4.3 Generate the CRDs with `controller-gen` as dbaas-operator does: Makefile with a pinned
      `CONTROLLER_TOOLS_VERSION`; `make generate` (DeepCopy), `make manifests` (CRDs into `config/crd/bases`),
      `make sync-helm-crds` (each CRD wrapped in `{{- if not .Values.restrictedEnvironment }}` into
      `helm-templates/maas-service/templates/crd-*.yaml`). The generated templates replace the hand-written ones from 2.1;
      they are never edited by hand (say so in `AGENTS.md`). `sync-helm-crds` also adds the chart labels the
      hand-written templates have (such as `deployment.netcracker.com/sessionId`), since controller-gen output holds no
      Helm expressions. CI fails when `make sync-helm-crds` leaves a git diff. CI also renders the chart with
      `helm template` and installs the rendered CRDs into envtest, so the API server rejects bad metadata (for example
      a non-ASCII label key) before release
- [ ] 4.4 `docs/operator_design.md`: drop the full CRD YAML, keep the sample CRs and field tables, link to the generated
      templates
- [ ] 4.5 Start the manager from `server.go` only when `OPERATOR_ENABLED`; Lease `maas-operator-leader` in
      `CLOUD_NAMESPACE`; only the leader watches; Fiber stays on all replicas; a manager failure does not stop HTTP
- [ ] 4.6 Cluster watch of instance CRs; skip when `operatorNamespace` ≠ `CLOUD_NAMESPACE` (no PATCH); informer not
      filtered on `generation`, so the `maas.netcracker.com/refresh` annotation enqueues a reconcile
- [ ] 4.7 Unit tests: claim skip, leader-only start, operator disabled starts nothing, env `OPERATOR_ENABLED` reaches
      `operator.enabled` (also turns the REST lock on); envtest applies the generated CRDs and checks valid and invalid
      sample CRs (CEL rules, immutable `operatorNamespace`, Kafka credential keys)

## 5. Reconcile: register and update

- [ ] 5.1 Get Secrets on every reconcile (no Secret Watch); `SecretError` for missing Secret, missing or empty key, or
      forbidden; message names Secret and key, never the value
- [ ] 5.2 Spec: requirement for credential Secrets (value formats, any Secret type, Secret deleted after a good
      register keeps the PG row); fix the stale Secret sample in `docs/operator_design.md`
- [ ] 5.3 Mapper CR + Secrets → `model.KafkaInstance` / `RabbitInstance` (`Default: false`; Kafka admin/client roles,
      `plain:` password prefix)
- [ ] 5.4 Name/id rules from design.md. First, if another managed row of this kind has `cr_namespace` = this CR
      namespace and a different `cr_name` → `DuplicateNamespace` naming that CR, no write. Then id = namespace; adopt
      by namespace or by old REST id; never rename an id;
      `DuplicateInstanceName` when another CR owns the id; set `managed_by_operator`, `cr_namespace` (CR
      `metadata.namespace`), `cr_name` (CR `metadata.name`)
- [ ] 5.5 Register or Update through InstanceService chosen by `GetById`; `HealthCheckFailed` keeps a previous good row;
      unique violation → `InvalidSpec`, except the `cr_namespace` index (a race past the 5.4 check) →
      `DuplicateNamespace`. The DAOs return duplicate id / Rabbit URL / Kafka addresses as `msg.Conflict`
      (REST 409), the only `msg.Conflict` from InstanceService, so ProcessCR can map it to `InvalidSpec`; keep it distinct
      from `HealthCheckFailed` if InstanceService adds other conflicts. Recheck the accepted REST lock race in design.md
      Risks once the operator write path exists
- [ ] 5.6 Update when `resourceVersion` differs from `status.secretRevisions`; skip apply when `Ready` and spec, Secrets
      and refresh annotation are unchanged
- [ ] 5.7 Status `Ready` + `Stalled`, `observedGeneration`, `lastRequestId`, reasons from the broker-instances spec
- [ ] 5.8 Backoff (error 1s→5m with jitter) and 10m resync
- [ ] 5.9 Unit tests: SecretError, HealthCheckFailed, DuplicateInstanceName, DuplicateNamespace, InvalidSpec, adopt,
      rotation, skip apply

## 6. Delete, defaults, and events

- [ ] 6.1 Finalizer `maas.netcracker.com/instance` after a successful register
- [ ] 6.2 `deletionPolicy` Unregister / Orphan (Orphan sets `managed_by_operator` false, clears `cr_namespace` and
      `cr_name`); `InstanceInUse` + RequeueAfter 30s
- [ ] 6.3 `OPERATOR_ENABLED=false` and old-chart downgrade do not Unregister
- [ ] 6.4 Default params `DEFAULT_*_INSTANCE` (compared with the CR `metadata.namespace`) and `ForcedDefault` rules
      from the spec; `status.isDefault` from PG
- [ ] 6.5 Events in CR namespace; honor `K8S_EVENTS_ENABLED`; no Event for `ForcedDefault` or unchanged reason
- [ ] 6.6 Unit tests: InstanceInUse, Orphan hand-back, default params (same CR name in two namespaces does not flip the
      default), events on and off

## 7. Verify and archive (in the PR that completes section 6)

- [ ] 7.1 Product docs: point operators at CRDs/values; keep HLA for architecture
- [ ] 7.2 `/opsx:verify` against tasks, specs, and Jira AC
- [ ] 7.3 `/opsx:archive` (sync main specs, move change to archive)
