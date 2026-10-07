# AGENTS.md — qubership-maas

## Project overview

MaaS (Messaging as a Service) manages messaging entities on RabbitMQ and Kafka brokers for cloud-native platforms:

- **RabbitMQ** — vhosts, exchanges, queues, bindings
- **Kafka** — topics (including templates, lazy topics, tenant topics)

Entities can be created via declarative configuration or REST API. MaaS also supports Blue/Green application
deployments. It persists state in PostgreSQL and returns broker connection metadata/credentials to clients — it does
**not** proxy traffic to brokers.

Applications talk to MaaS through **maas-agent** (security proxy) with M2M tokens, not directly.

Module path: `github.com/netcracker/qubership-maas`

## Tech stack

- **Language:** Go 1.26.5
- **HTTP:** Fiber v3 (`github.com/gofiber/fiber/v3`)
- **DB:** PostgreSQL (`go-pg`, `pgx`), SQLite for some tests
- **Kafka client:** IBM Sarama
- **Config:** `application.yaml` + koanf / qubership-core-lib-go configloader
- **Vendoring:** dependencies are vendored under `maas/maas-service/vendor/`
- **Integration tests:** Maven (`maas-integration-tests/`)
- **Deploy:** Helm charts under `helm-templates/maas-service`

## Project structure

```text
maas/
  Dockerfile                 # Multi-stage Go build → qubership-core-base runtime
  maas-service/              # Main Go module (source of truth for the service)
    server.go                # Entry point
    application.yaml         # Runtime config
    controller/              # HTTP handlers
    service/                 # Business logic (kafka, rabbit, bg2, composite, …)
    dao/                     # Persistence
    model/                   # Domain models / DTOs
    router/                  # API routing
    docs/                    # Swagger (swagger.json / swagger.yaml)
    vendor/                  # Vendored Go deps
maas-integration-tests/      # Maven-based integration tests
helm-templates/maas-service/ # Helm chart
docs/                        # Product docs (REST API, BG, monitoring, …)
dev/                         # Local docker-compose and REST helpers
validation-image/            # Separate validation Docker image
pom.xml                      # Parent POM (integration-tests module only)
.github/workflows/           # CI (Go build, IT, Docker, linters)
```

## Build & test commands

Work from the Go module directory:

```bash
cd maas/maas-service

# Compile
go build -o maas-service .

# Unit tests
go test ./...

# Unit tests with coverage
go test ./... -covermode=atomic -coverprofile=coverage.out

# After dependency changes — tidy then refresh vendor
go mod tidy
go mod vendor
```

Docker image (from `maas/`):

```bash
cd maas
docker build -t maas-service .
```

Integration tests (Maven, repo root):

```bash
mvn -pl maas-integration-tests verify
```

CI for Go runs via `.github/workflows/maas---build-on-push.yaml` with `go-module-dir: maas/maas-service`. Prefer
matching that module path locally.

## Code style & conventions

- Prefer small, focused changes; match existing package layout (`controller` / `service` / `dao` / `model`).
- Keep Go version in sync: `go.mod` (`go 1.26.5`) and `maas/Dockerfile` builder image.
- After adding/updating Go deps: run `go mod tidy` and `go mod vendor` so CI and offline builds stay consistent.
- Do not edit files under `maas/maas-service/vendor/` by hand.
- Conventional Commits are enforced on PRs (`.github/workflows/pr-conventional-commits.yaml`).
- Product behavior docs live in `docs/` and `README.md` — update them when changing public API or operational behavior.

## Roles & API notes (for agents)

| Role      | Typical use                                      |
|-----------|--------------------------------------------------|
| `manager` | User accounts, register Rabbit/Kafka instances   |
| `agent`   | Create/delete vhosts, topics, exchanges, queues  |

Classifier identity for entities: `name` + `namespace` (required), optional `tenantId`.

## OpenSpec

Spec-driven changes live under `openspec/`. Main specs (`openspec/specs/`) are empty until a change is archived. Active
work is one folder per change under `openspec/changes/`.

Before implementing a change, read its folder under `openspec/changes/`. If it needs a SPEC PR, do not implement until
that PR merges.

Team path for larger changes: Jira → `/opsx-propose` artifacts → SPEC PR → Code PR(s) with `/opsx-apply` →
`/opsx-verify` → `/opsx-archive` in the completing Code PR.

Cursor commands (filename form `/opsx-propose`): explore, propose, apply, update, verify, archive, sync. They live in
`.cursor/commands/`. After Node is available, `npx @fission-ai/openspec init` / `update` can refresh generated tool
files; do not hand-edit a vendored CLI copy.

## Git workflow (strict)

**Never** create a git commit or push to a remote unless the user sends the exact text command:

```text
commit and push
```

- Phrases like “commit”, “push”, “ship it”, “create a PR”, or implied approval are **not** enough.
- Until that exact command appears, only edit files locally; show a diff summary and a proposed commit message if
  useful, then wait.
- Do not amend, force-push, or skip hooks unless the user explicitly asks in addition to `commit and push`.
