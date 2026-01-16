# Clean DDD Petstore (Go + Gin + Temporal)

This repo keeps the OpenAPI-generated Gin transport intact while moving the business logic into a Clean Architecture layout. Bounded contexts for pets, store, and users live under `internal/domains/`, and Temporal can orchestrate pet creation when available.

## Layout
```
go-gin-api-server/
├── api/                            # OpenAPI contract served at /openapi.(json|yaml) and /swagger
├── bin/                            # Local build artifacts (ignored in VCS)
├── cmd/                            # Entry points
│   ├── api/                        # HTTP API composition root (observability, repos, services, router)
│   ├── worker/                     # Temporal worker wiring for pet creation
│   └── session-purger/             # CLI to purge expired sessions
├── docs/                           # Architecture notes and diagrams
├── generated/go/                   # Generated Gin router + DTOs delegating to application services
├── internal/                       # Domain/application code, adapters, and platform helpers
│   ├── app/                        # Process wiring (config, run logic)
│   ├── clients/http/partner/       # Partner sync HTTP client used by the mapper/sync adapter
│   ├── domains/                    # Bounded contexts (pets, store, users)
│   ├── platform/                   # OTEL, Postgres helpers, migrations, Temporal workflows
│   └── shared/                     # Cross-cutting projection helpers
├── pacts/                          # Generated Pact contracts
├── test/pact/                      # Pact consumer/provider tests and helpers
├── Dockerfile
├── Makefile
├── go.mod
├── go.sum
├── description.md
├── project.md
└── README.md
```

**Domain slices** (bounded contexts): `internal/domains/pets`, `internal/domains/store`, `internal/domains/users`. Everything else under `internal/` supports those domains (platform, integrations, workflows).

## Runtime entrypoints
- `cmd/api/main.go`: Boots slog + OpenTelemetry, loads config from env, selects repositories (Postgres via `POSTGRES_DSN`, otherwise in-memory), runs schema migrations (`internal/platform/migrations`), builds services (optionally wiring partner sync when `PARTNER_API_BASE_URL` is set), and chooses the pet workflow orchestrator (Temporal client when reachable; inline when `TEMPORAL_DISABLED=1`). Wires generated handlers (`go/api_*.go`) into `go/routers.go` and listens on `:$PORT` (default `8080`). Serves `/openapi.(json|yaml)` and `/swagger`. Health endpoints: `/healthz`, `/readyz` (checks DB + Temporal when enabled), and `/debug/config` (sanitized view). Optional session purge ticker runs when `SESSION_PURGE_INTERVAL_MINUTES` is set.
- `cmd/worker/main.go`: Shares the same repository selection and observability setup, registers the pet creation workflow and activity bundle on queue `PET_CREATION`, and runs against the Temporal frontend (`TEMPORAL_ADDRESS`, `TEMPORAL_NAMESPACE`).
- `cmd/session-purger/main.go`: One-off CLI to purge expired user sessions using `POSTGRES_DSN`; respects `SESSION_TTL_HOURS` for expiry.

## Bounded contexts
### Pets (`internal/domains/pets`)
- `domain`: Pet aggregate with category, tags, external reference, hair length, and status invariants.
- `application`: Use cases with tracing/metrics/logging; command/query inputs live under `application/types/` (mutations, queries, imports, grooming, media).
- `ports`: Repository and workflow orchestrator interfaces plus shared errors.
- `adapters`: HTTP mapper (`adapters/http/mapper`), in-memory repository (`adapters/memory`), Postgres repository with array/JSON mapping (`adapters/persistence/postgres`; schema managed via `internal/platform/migrations`), workflow orchestrators (inline vs Temporal) under `adapters/workflows`, an idempotency store (`pet_idempotency_keys` table in Postgres or in-memory), and an external partner adapter that maps payloads and syncs via `internal/clients/http/partner` when enabled.
- Idempotency: `POST /v2/pet` accepts `Idempotency-Key`; identical payloads replay the stored projection, mismatches return HTTP 409. Temporal workflow IDs are derived from the key to dedupe runs.

### Store (`internal/domains/store`)
- Order aggregate and statuses, application service with inventory calculation, repository interface, in-memory repository, Postgres repository (schema via `internal/platform/migrations`), and HTTP mappers.

### Users (`internal/domains/users`)
- User entity, application service for CRUD/login, repository interface, in-memory repository, HTTP mappers, Postgres repository, and Postgres session store (schema via `internal/platform/migrations`, TTL via `SESSION_TTL_HOURS`, purge via ticker or CLI).

## Platform and shared pieces
- `internal/platform/observability`: Slog JSON logger plus OTLP HTTP exporter (fallback to stdout), tracer/meter providers, and global propagator setup. Configured via `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_INSECURE`, and `ENVIRONMENT`.
- `internal/platform/postgres`: GORM connector used by repositories and processes.
- `internal/shared/projection`: Projection wrapper carrying created/updated timestamps.

## Running locally
```powershell
# API with defaults (in-memory pets repo, inline workflows)
go run ./cmd/api

# API with Postgres repository
$env:POSTGRES_DSN="postgres://user:pass@localhost:5432/pets?sslmode=disable"; go run ./cmd/api

# Temporal worker (run alongside the API when using Temporal)
$env:TEMPORAL_ADDRESS="127.0.0.1:7233"; go run ./cmd/worker

# Manual session purge (cron/one-off)
$env:POSTGRES_DSN="postgres://user:pass@localhost:5432/pets?sslmode=disable"; go run ./cmd/session-purger
```

Environment knobs:
- `PORT`: HTTP bind port for the API (default `8080`).
- `POSTGRES_DSN`: Enables Postgres-backed repositories/session store; falls back to memory if unset/invalid.
- `PARTNER_API_BASE_URL`: Enables outbound partner sync after pet mutations; leave unset to disable.
- `SESSION_TTL_HOURS`: TTL for user sessions (default 24h).
- `SESSION_PURGE_INTERVAL_MINUTES`: When set, API runs a background ticker to purge expired sessions.
- `TEMPORAL_ADDRESS`, `TEMPORAL_NAMESPACE`: Temporal connection for API/worker; `TEMPORAL_DISABLED=1` forces inline pet creation.
- `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_INSECURE`, `ENVIRONMENT`: Observability config.

## OpenAPI/Swagger
- Contract lives at `api/openapi.yaml` and is served by the generated router at `/openapi.yaml` and `/openapi.json`.
- Interactive docs are available at `/swagger`; handlers in `go/api_*.go` delegate to the application services through mappers while preserving the generated DTOs.

## Contract testing (Pact)
- Generate consumer pacts (writes to `./pacts`): `make pact-consumer` (requires `libpact_ffi` from the Pact standalone bundle or Homebrew `pact-ruby-standalone`).
- Verify provider against local pacts: `make pact-provider` (starts the API in-memory for verification).
- End-to-end: `make pact-contracts` runs both steps; ensure pacts exist before verifying in CI.
- Coverage: consumer tests exercise pets (CRUD + grooming/form), store orders (place/get/delete/inventory), and user CRUD/login/logout; provider states live in `test/pact/pacttest.go`.
- If your environment blocks the default Go build cache path, set `GOCACHE=/tmp/go-build` when running the Pact suites.
- Optional broker: `docker-compose up pact-broker` brings up a Pact Broker on `http://localhost:9292` backed by the `pactbroker-db` Postgres service.

## Integrating other Pet APIs
- Keep the domain model authoritative and treat providers as adapters: `internal/domains/pets/adapters/external/partner` maps domain pets to partner payloads and uses `internal/clients/http/partner` to sync.
- Store external identities on the aggregate (`ExternalReference`) so you can reconcile records without leaking provider details elsewhere.
- Map provider-specific fields inside the adapter; persist only what you own, and round-trip untouched provider attributes via the mapper if needed.

## Handling transient operation data
- For flows that need temporary inputs (e.g., grooming trims), keep those values on the command DTO, let the application service compute the durable result, and update the aggregate via domain methods that enforce invariants (`internal/domains/pets/domain/pet.go`).
- `POST /v2/pet/{petId}/groom` demonstrates this: the payload feeds the calculation while the stored `hairLengthCm` reflects only the resulting value.

## Next steps
- Add acceptance tests per bounded context (e.g., `internal/domains/pets/application/service_test.go`) including Postgres paths and Temporal inline execution.
- Replace AutoMigrate with managed migrations tooling if you need strict schema control across environments.
- Integrate auth/api-key middleware at the router level if required by your environment.
