# Clean DDD Petstore (Go + Gin + Temporal)

This repo keeps the OpenAPI-generated Gin transport intact while moving the business logic into a Clean Architecture layout. Bounded contexts for pets, store, and users live under `internal/`, and Temporal can orchestrate pet creation when available.

## Layout
```
go-gin-api-server/
- api/                       # OpenAPI contract served at /openapi.(json|yaml) and /swagger
- cmd/
  - api/main.go              # HTTP API composition root (observability, repos, services, router)
  - worker/main.go           # Temporal worker wiring for pet creation
- go/                        # Generated Gin router + DTOs delegating to application services
- internal/
  - clients/http/partner/    # Partner sync HTTP client used by the mapper
  - durable/temporal/        # Workflows, activities, sequences for pet creation
  - pets/                    # Pets bounded context (domain, application, ports, adapters)
  - platform/                # Shared platform concerns (OTEL, Postgres helpers)
  - shared/                  # Cross-cutting projection helpers
  - store/                   # Store/orders bounded context
  - users/                   # User management bounded context
- Dockerfile
- description.md
- project.md
- README.md
```

## Runtime entrypoints
- `cmd/api/main.go`: Boots slog + OpenTelemetry, selects the pets repository (Postgres via `POSTGRES_DSN` with automigrate, otherwise in-memory), builds services, and chooses the pet workflow orchestrator (Temporal client when reachable; inline when `TEMPORAL_DISABLED=1` or dialing fails). Wires generated handlers (`go/api_*.go`) into `go/routers.go` and listens on `:$PORT` (default `8080`). Serves `/openapi.(json|yaml)` and `/swagger`.
- `cmd/worker/main.go`: Shares the same repository selection and observability setup, registers the pet creation workflow and activity bundle on queue `PET_CREATION`, and runs against the Temporal frontend (`TEMPORAL_ADDRESS`, `TEMPORAL_NAMESPACE`).

## Bounded contexts
### Pets (`internal/pets`)
- `domain`: Pet aggregate with category, tags, external reference, hair length, and status invariants.
- `application`: Use cases with tracing/metrics/logging; command/query inputs live under `application/types/` (mutations, queries, imports, grooming, media).
- `ports`: Repository and workflow orchestrator interfaces plus shared errors.
- `adapters`: HTTP mapper (`adapters/http/mapper`), in-memory repository (`adapters/memory`), Postgres repository with automigrations and array/JSON mapping (`adapters/persistence/postgres`), workflow orchestrators (inline vs Temporal) under `adapters/workflows`, and an external partner mapper backed by `internal/clients/http/partner`.

### Store (`internal/store`)
- Order aggregate and statuses, application service with inventory calculation, repository interface, in-memory repository, and HTTP mappers.

### Users (`internal/users`)
- User entity, application service for CRUD/login, repository interface, in-memory repository, and HTTP mappers.

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
```

Environment knobs:
- `PORT`: HTTP bind port for the API (default `8080`).
- `POSTGRES_DSN`: Enables the Postgres-backed pets repository; falls back to memory if unset/invalid.
- `TEMPORAL_ADDRESS`, `TEMPORAL_NAMESPACE`: Temporal connection for API/worker; `TEMPORAL_DISABLED=1` forces inline pet creation.
- `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_INSECURE`, `ENVIRONMENT`: Observability config.

## OpenAPI/Swagger
- Contract lives at `api/openapi.yaml` and is served by the generated router at `/openapi.yaml` and `/openapi.json`.
- Interactive docs are available at `/swagger`; handlers in `go/api_*.go` delegate to the application services through mappers while preserving the generated DTOs.

## Integrating other Pet APIs
- Keep the domain model authoritative and treat providers as adapters: `internal/pets/adapters/external/partner` maps domain pets to partner payloads and uses `internal/clients/http/partner` to sync.
- Store external identities on the aggregate (`ExternalReference`) so you can reconcile records without leaking provider details elsewhere.
- Map provider-specific fields inside the adapter; persist only what you own, and round-trip untouched provider attributes via the mapper if needed.

## Handling transient operation data
- For flows that need temporary inputs (e.g., grooming trims), keep those values on the command DTO, let the application service compute the durable result, and update the aggregate via domain methods that enforce invariants (`internal/pets/domain/pet.go`).
- `POST /v2/pet/{petId}/groom` demonstrates this: the payload feeds the calculation while the stored `hairLengthCm` reflects only the resulting value.

## Next steps
- Swap in real storage for store/users, or extend the pets Postgres adapter with migrations/tests.
- Add acceptance tests per bounded context (e.g., `internal/pets/application/service_test.go`).
- Integrate auth/api-key middleware at the router level if required by your environment.
