# AGENTS.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project overview

Registry is a gRPC service that is the central data management service for the OpenKCM (CMK) landscape. It owns two top-level domains:

- **Tenants** — CMK tenant lifecycle (create / block / unblock / terminate). Driven by the Tenant Management API and consumed by the regional CMK layer.
- **Systems / RegionalSystems** — customer-exposed business systems and their per-region key assignment state. The crypto layer announces and terminates systems here; the CMK application reads them.

A separate `Auth` and `Mapping` gRPC API live alongside these. All four are registered on a single gRPC server in `cmd/registry/main.go`.

## Commands

Tests use `gotestsum` (auto-installed via the Makefile target into `$GOPATH/bin`).

```sh
# Unit tests + coverage report (writes cover.out, junit.xml)
make test
make test-cover                       # opens HTML coverage in browser

# Lint (golangci-lint, --fix enabled)
make lint

# Single test
go test -run TestName ./internal/service/...
go test -run TestName/subtest ./internal/...

# Integration tests — require Postgres + RabbitMQ + otel-collector + a running registry binary
make docker-compose-dependencies-up   # bring up deps only
make int-test-up-and-run              # builds registry, starts it, runs ./integration/... with -tags=integration, tears down
make integration-test                 # full pipeline: deps up → unit + integration with merged coverage → deps down

# Single integration test (deps and registry must already be up)
go test -tags=integration -run TestName ./integration/...

# Helm chart tests (needs a running k8s cluster)
make helm-test

# Run locally
make docker-compose-dependencies-up
go run ./cmd/registry/main.go         # reads ./config.yaml or /etc/registry/config.yaml
# OR full stack incl. Grafana on :3004 (admin/admin)
make docker-compose-up
```

`make integration-test` is what CI runs end-to-end; reach for it when changes touch service/repository/orbital wiring.

`make compile-servicetest-pb` regenerates the protobuf used only by `internal/interceptor/servicetest`. Domain protos live in the external `github.com/openkcm/api-sdk` module — not in this repo.

## Architecture

### Layered structure (`internal/`)

```
cmd/registry/main.go         wire-up: config → logger → OTLP → status server → DB → Orbital → services → gRPC
   │
   ├── internal/config/      typed config + Validate(); loaded via openkcm/common-sdk commoncfg
   ├── internal/model/       domain entities (Tenant, System, RegionalSystem, Auth) — also repository.Resource
   ├── internal/repository/  generic Repository interface (Create/List/Find/Patch/PatchAll/Delete/Transaction)
   │     └── sql/            GORM/Postgres implementation; resources are plain structs implementing TableName()
   ├── internal/validation/  field-level validators driven by `validations:` block in config.yaml — see internal/validation/README.md
   ├── internal/interceptor/ gRPC unary+stream interceptors: panic recovery and OTel metrics
   └── internal/service/     gRPC handlers (tenant/system/mapping/auth) + Orbital wrapper
```

The repository abstraction is deliberately generic — `Resource` only needs `TableName()` and `PaginationKey()`. Add new persisted types by implementing those on the model and using the existing CRUD methods; do not add per-entity repositories.

### Validation is config-driven

Field constraints (`non-empty`, `list` allowlist, `non-empty-keys`) are declared in `config.yaml` under `validations:` and matched against validation IDs declared as struct tags on the models. `cmd/registry/main.go::initValidation` registers the model set; missing IDs fail startup unless the field uses `skipIfNotExists: true` (used for dynamic map keys, e.g. `System.Labels.*`). When adding a model, register it there and read `internal/validation/README.md` for the tag syntax and constraint API.

### Orbital — async job processing

`service.Orbital` (`internal/service/orbital.go`) wraps `github.com/openkcm/orbital` to dispatch long-running jobs to per-region targets over AMQP. The pattern:

1. A service (e.g. `tenant.go`) calls `orbital.PrepareJob(ctx, payload, externalID, jobType)`.
2. Orbital persists the job (Postgres, via `orbsql`), confirms it, resolves it into per-target tasks, and sends them via the AMQP client for that region.
3. Lifecycle callbacks (`ConfirmJob`, `ResolveTasks`, `HandleJobDone`/`Canceled`/`Failed`) are dispatched to a `JobHandler` registered for that `jobType` via `Orbital.RegisterJobHandler`.

When adding a new async flow: define a job type constant, implement `service.JobHandler`, register it during service construction, and add the region target(s) to `config.yaml::orbital.targets`. The four orbital workers (`confirm-job`, `create-task`, `reconcile`, `notify-event`) are tunable per-deployment under `orbital.workers`.

### gRPC API surface

The four service implementations satisfy interfaces from `github.com/openkcm/api-sdk/proto/kms/api/cmk/registry/{tenant,system,mapping,auth}/v1`. Changes to request/response shapes happen in `api-sdk`, not here — bump the dependency, then regenerate consumers.

### Errors

`internal/service/error.go` defines the canonical service errors with codes (`ErrTenantSelect`, `ErrSystemUnavailable`, `ErrValidationFailed`, …). Use `ErrorWithParams(err, "key", value)` to attach context. Keep new error variables in this file — do not return `errors.New` from inside handlers.

## Conventions

- Logging uses `github.com/veqryn/slog-context` (`slogctx.Info(ctx, ...)`); always carry context.
- Imports are grouped by `gci` with these prefixes: standard / default / `github.com/openkcm/registry` / blank / dot / alias / localmodule. `make lint` enforces this.
- Tests are named `*_test.go`; integration tests are tagged `//go:build integration` and live in `./integration/`. Helm tests are tagged `helmtest` under `./helmtest/`.
- A handful of linters are off project-wide (`exhaustruct`, `wrapcheck`, `nlreturn`, `mnd`, `lll`, `wsl*`, `gochecknoglobals`, …) — see `.golangci.yaml`. Don't fight them; match surrounding style.
