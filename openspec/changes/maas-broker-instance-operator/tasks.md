# Tasks

Implementation starts after this SPEC PR merges. Do not check these off from design-only work.

One section per Code PR, in merge order. Each PR ships the unit tests for its own code and keeps `OPERATOR_ENABLED`
default false, so `main` stays releasable after every merge.

## 1. SPEC PR (this change)

- [ ] 1.1 File or update Jira with AC matching proposal.md (success, errors, compatibility)
- [ ] 1.2 Close open questions: Rabbit URL uniqueness in scope; CR delete always hands the row back to REST;
      `spec.takeOver` and Blue/Green adopt dropped
- [ ] 1.3 Merge SPEC PR (openspec/ + HLA). No ProcessCR code in that PR

## 2. CRDs and chart

- [ ] 2.1 Add `KafkaInstance` / `RabbitInstance` CRDs (`maas.netcracker.com/v1`, short names, printer columns, CEL
      immutability on `operatorNamespace`, selectableFields, Kubernetes 1.32+, Kafka `credentials.admin` /
      `credentials.client`, CEL checks on addresses and credential keys, Rabbit URL schemes)
- [ ] 2.2 Helm values: `OPERATOR_ENABLED` (default false), `DEFAULT_KAFKA_INSTANCE`, `DEFAULT_RABBIT_INSTANCE`,
      `K8S_EVENTS_ENABLED` (default true), `restrictedEnvironment` (default false). `DEFAULT_*_INSTANCE` is the broker
      namespace; `values.schema.json` allows empty or a DNS-1123 label. The schema had every property nested one level
      too deep, so Helm validated nothing; flattened it, with string-or-boolean / string-or-integer for values that only
      become env text (`DB_POSTGRESQL_TLS_*`, `DB_POOL_SIZE`) and strict booleans for values used in `if`
- [ ] 2.3 ClusterRole: instance CRs watch/status/finalizers; no `secrets`; events create/patch when events enabled
- [ ] 2.4 Namespaced Lease Role for `maas-operator-leader`; document per-namespace Secret Role + RoleBinding
- [ ] 2.5 `restrictedEnvironment: true` skips ClusterRole in-chart

## 3. Persistence and REST lock (no Kubernetes dependencies)

- [ ] 3.1 Migration: PG columns `cr_namespace` and `cr_name` on Kafka and Rabbit instance tables (empty default);
      existing rows stay unmanaged; a non-empty `cr_namespace` means CR-managed
- [ ] 3.2 Migration: unique index on `rabbit_instances.api_url` and on `amqp_url`. If rows already share a URL, create
      nothing and stop startup with one error listing each shared URL and its instance ids
- [ ] 3.3 Map the Rabbit unique violation to a REST error (409, as Kafka `addresses` and duplicate ids); document it in
      `docs/rest_api.md` and the fix in `docs/troubleshooting.md`
- [ ] 3.4 `OPERATOR_ENABLED` in `application.yaml` (env override, default false)
- [ ] 3.5 Reject manager REST Update/Unregister when `cr_namespace` is set and `OPERATOR_ENABLED` is true;
      allow it when the operator is disabled
- [ ] 3.6 Old binary and new rows: columns have defaults, so a downgrade reads them as unmanaged and Unregisters nothing
- [ ] 3.7 Unit tests: migrations (fresh DB, existing rows, duplicate Rabbit URLs), REST lock on and off
- [ ] 3.8 Reject manager REST SetDefault (409) for a kind while `OPERATOR_ENABLED` is true and that kind's
      `DEFAULT_*_INSTANCE` param is set; allow it otherwise; document in `docs/rest_api.md`; unit tests for param set,
      param empty, and operator disabled
- [ ] 3.9 Migration 31: partial unique index on `cr_namespace` where `cr_namespace <> ''` for Kafka and Rabbit (one
      claimed CR of each kind per broker namespace); unit test that a second managed row in one namespace fails and an
      unmanaged one does not

## 4. Operator skeleton (watch and claim, no writes to MaaS)

- [ ] 4.1 Add `sigs.k8s.io/controller-runtime` (with client-go); `go mod tidy` and `go mod vendor`
- [ ] 4.2 Go CR types for `KafkaInstance` / `RabbitInstance` (`api/v1` package), registered in a runtime scheme. The
      types are the source of the CRD schema: `+kubebuilder` markers carry descriptions, CEL rules, enums, short names,
      printer columns, selectable fields, and the status subresource. One `spec.address` (`KafkaAddress`: `protocol`
      enum, `saslMechanism`, `bootstrapServers`, `caCertSecretRef`). One `KafkaCredential` struct for both `admin` and
      `client` (`clientCert`, `credentialsSecretRef`); no `maasProtocol`, no `deletionPolicy`. Secret refs are
      `SecretNameRef {name}` only, shared by Kafka, Rabbit, and CA. Rabbit `apiURL` / `amqpURL`. `DEFAULT` printer
      column from the `Default` condition, no `isDefault`. Status as a shared `OperatorStatus` with
      `[]metav1.Condition`. CEL from design.md: `saslMechanism` / `clientCert` / `credentialsSecretRef` per
      `address.protocol`, admin and client both need a Secret or neither, and the client library compatibility rules
      (no `saslMechanism` on `PLAINTEXT`; reject `SASL_SSL` + `PLAIN` without `clientCert`,
      TLS without `caCertSecretRef`), each message naming the form that works. The top-level description says new
      instances get PG id = CR namespace; adopt is by URL set. `metadata.name` is not CEL-checked against namespace.
- [ ] 4.3 Generate the CRDs with `controller-gen`: Makefile with a pinned `CONTROLLER_TOOLS_VERSION`; `make generate`
      (DeepCopy), `make manifests` (raw CRD YAML into `crds/`, no Helm). Delete
      `helm-templates/maas-service/templates/crd-*.yaml`. The generated files replace the hand-written chart CRDs from
      2.1; they are never edited by hand (say so in `AGENTS.md`). CI fails when `make manifests` leaves a git diff. CI
      installs `crds/` into envtest, so the API server rejects bad metadata (for example a non-ASCII label key) before
      release
- [ ] 4.4 `docs/operator_design.md`: link the `#### CRD` sections to `crds/` (sample CRs and field tables stay)
- [ ] 4.5 Start the manager from `server.go` only when `OPERATOR_ENABLED`; Lease `maas-operator-leader` in
      `CLOUD_NAMESPACE`; only the leader watches; Fiber stays on all replicas; a manager failure does not stop HTTP
- [ ] 4.6 Cluster watch of instance CRs; skip when `operatorNamespace` ≠ `CLOUD_NAMESPACE` (no PATCH); informer not
      filtered on `generation`, so the `maas.netcracker.com/refresh` annotation enqueues a reconcile
- [ ] 4.7 Unit tests: claim skip, leader-only start, operator disabled starts nothing, env `OPERATOR_ENABLED` reaches
      `operator.enabled` (also turns the REST lock on); envtest applies the generated CRDs and checks valid and invalid
      sample CRs (CEL rules, immutable `operatorNamespace`, every Kafka credential rule and compatibility rejection)

## 5. Reconcile: register and update

- [ ] 5.1 Get Secrets on every reconcile (no Secret Watch); `SecretError` for missing Secret, missing or empty key, or
      forbidden; message names Secret and key, never the value
- [ ] 5.2 Spec: requirement for credential Secrets (fixed Secret.data keys: Kafka SASL `username`/`password`, mTLS
      also `clientKey`/`clientCert`, CA `ca.crt`, Rabbit `username`/`password`; value formats; any Secret type; Secret
      deleted after a good register keeps the PG row)
- [ ] 5.3 Mapper CR + Secrets → `model.KafkaInstance` / `RabbitInstance` (`Default: false`; `MaasProtocol` =
      `address.protocol`; auth type derived per the table in `docs/operator_design.md`; one-entry `admin` / `client`
      lists, or nil when neither needs a Secret; `plain:` password prefix; Rabbit Secret `username` copied to REST
      `User`; `spec.address` → one-key MaaS `addresses` map). Unit test: the topic response of a CR-managed instance
      has the same shape as for a REST-registered one (client library compatibility)
- [ ] 5.4 Name/id rules from design.md. Identity is `metadata.namespace` (`cr_namespace`, new PG id); do not use
      `metadata.name` as a join key. Claimed-namespace first: row with this `cr_namespace` and this CR name → Update
      (including URL change). Other CR name in that namespace → `DuplicateNamespace`. Else find the row by Kafka host
      set
      or Rabbit AMQP URL (set compare, ignore order). No row → Register `id` = namespace. Else if that row is already
      CR-owned → `InvalidSpec`. Else adopt: keep id, set `cr_namespace` (and `cr_name`). Do not compare host DNS to the
      CR namespace. Never rename an id. Empty `cr_namespace` is not a match.
- [ ] 5.5 Register or Update through InstanceService chosen by URL / host set; `HealthCheckFailed` keeps a previous
      good row;
      unique violation → `InvalidSpec`, except the `cr_namespace` index (a race past the 5.4 check) →
      `DuplicateNamespace`. The DAOs return duplicate id / Rabbit URL / Kafka addresses as `msg.Conflict`
      (REST 409), the only `msg.Conflict` from InstanceService, so ProcessCR can map it to `InvalidSpec`; keep it
      distinct
      from `HealthCheckFailed` if InstanceService adds other conflicts. Recheck the accepted REST lock race in design.md
      Risks once the operator write path exists
- [ ] 5.6 Update when `resourceVersion` differs from `status.secretRevisions`; skip apply when `Ready` and spec, Secrets
      and refresh annotation are unchanged
- [ ] 5.7 Status `Ready` + `Stalled`, `observedGeneration`, `lastRequestId`, reasons from the broker-instances spec
- [ ] 5.8 Backoff (error 1s→5m with jitter) and 10m resync
- [ ] 5.9 Unit tests: SecretError, HealthCheckFailed, DuplicateInstanceName, DuplicateNamespace, InvalidSpec, adopt,
      rotation, skip apply, same-owner URL change
- [ ] 5.10 `status.alias` is the bound PostgreSQL instance id (designators, topic/vhost `instanceId`, manager REST);
      printer column `ALIAS`; empty until first bind; docs and unit tests for Register, adopt, skip-apply, first
      SecretError

## 6. Delete, defaults, and events

- [ ] 6.1 Finalizer `maas.netcracker.com/instance` after a successful register
- [ ] 6.2 CR delete always keeps the PG row (clear `cr_namespace` and `cr_name`, then
      remove the finalizer); no Unregister; no `deletionPolicy`
- [ ] 6.3 `OPERATOR_ENABLED=false` and old-chart downgrade do not Unregister
- [ ] 6.4 Default params `DEFAULT_*_INSTANCE` (compared with the CR `metadata.namespace`) and `ForcedDefault` rules
      from the spec; `Default` condition from PG (`Configured` / `ForcedDefault` / `NotDefault`, none before the
      first Register)
- [ ] 6.5 Events in CR namespace; honor `K8S_EVENTS_ENABLED`; no Event for the `Default` condition or an unchanged
      reason
- [ ] 6.6 Unit tests: CR-delete hand-back (row remains, REST can Update), default params (same CR name in two
      namespaces does not flip the default), events on and off

## 7. Verify and archive (in the PR that completes section 6)

- [ ] 7.1 Product docs: point operators at `crds/` and chart values; keep HLA for architecture
- [ ] 7.2 `/opsx:verify` against tasks, specs, and Jira AC
- [ ] 7.3 `/opsx:archive` (sync main specs, move change to archive)
