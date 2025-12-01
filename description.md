# Clean DDD Petstore snapshot

This repository reshapes the OpenAPI-generated Gin server into a Clean Architecture layout. The generated `go/` transport stays intact while bounded contexts live under `internal/`, and two processes (`cmd/api` and `cmd/worker`) compose the runtime.

## Layout
- `api/openapi.yaml`: Contract used to generate the Gin router/DTOs. Served at `/openapi.yaml`, `/openapi.json`, and `/swagger`.
- `cmd/api`: HTTP API composition root (observability, repositories, services, workflow orchestrator, router).
- `cmd/worker`: Temporal worker composition root for pet creation workflows.
- `go/`: Generated Gin transport that mounts routes and delegates to application services (`go/api_*.go`, `go/routers.go`).
- `internal/`: Domain/application code, adapters, and platform helpers.
  - `clients/http/partner`: Minimal client used by the partner mapper.
  - `durable/temporal`: Workflows, activities, and sequences for pet creation.
  - `pets`, `store`, `users`: Bounded contexts (domain, application, ports, adapters).
  - `platform`: Observability and Postgres helpers.
  - `shared`: Cross-cutting projection helpers.

## Runtime entrypoints
### HTTP API (`cmd/api/main.go`)
- Boots slog + OpenTelemetry via `internal/platform/observability.Init` (OTLP HTTP exporter or stdout fallback; resource attributes include `service.name` and `ENVIRONMENT`).
- Picks the pets repository: Postgres (`POSTGRES_DSN`) with automigrations, or in-memory fallback with warnings when DSN is missing/unusable.
- Builds the pets service with logger/tracer/meter, and selects a workflow orchestrator: Temporal client if reachable (`TEMPORAL_ADDRESS`, `TEMPORAL_NAMESPACE`, opt-out via `TEMPORAL_DISABLED=1`), otherwise inline execution.
- Wires store and user services with their in-memory repositories.
- Registers generated handlers (`ApiHandleFunctions`) and adds `otelgin` middleware. Binds to `:$PORT` (default `8080`) and serves OpenAPI/Swagger assets.

### Temporal worker (`cmd/worker/main.go`)
- Reuses observability bootstrap and the same repository selection logic.
- Creates the pets service and activity bundle.
- Dials Temporal with tracing interceptor and registers `pets.workflows.Creation` plus the `pets.activities.CreatePet` activity on task queue `PET_CREATION`; runs until interrupted.

## Transport layer (`go/`)
- Generated Gin router (`go/routers.go`) mounts all OpenAPI routes, serves spec files, and hosts Swagger UI.
- `go/api_pet.go`, `go/api_store.go`, `go/api_user.go` adapt HTTP payloads to application types via mappers and call into services/workflows. Grooming (`POST /v2/pet/:petId/groom`) and upload endpoints are wired here.

## Pets bounded context (`internal/pets`)
### Domain
- `domain/pet.go`: Pet aggregate with category, tags, external reference, hair length, and status (`available|pending|sold`). Invariants enforce non-empty name/photos, non-negative hair length, and grooming trims not exceeding the initial measurement.
### Application
- `application/service.go`: Use cases with OTEL tracing/metrics and slog logging. Operations: `AddPet`, `UpdatePet`, `UpdatePetWithForm`, `GroomPet`, `UploadImage` (metadata only), `FindByStatus`, `FindByTags`, `GetByID`, `List`, `Delete`. Metrics counters track created/updated/deleted/groomed pets.
- `application/types`: Command/query DTOs, partner import candidate with validation, projection DTO carrying persistence metadata.
### Ports
- `ports/repository.go`: CRUD + query interface returning projections (`ErrNotFound` on misses).
- `ports/workflows.go`: `CreatePet` orchestrator abstraction (Temporal or inline).
### Adapters
- `adapters/http/mapper`: Converts generated HTTP models to mutation inputs (preserving field presence), grooming DTOs, and back to transport projections.
- `adapters/memory`: Thread-safe in-memory repository used by default.
- `adapters/persistence/postgres`: GORM-backed repository with automigrate, storing tags as arrays and external attributes as JSON; maps to domain projections.
- `adapters/workflows`: Inline orchestrator and Temporal orchestrator that starts `pets.workflows.Creation`.
- `adapters/external/partner`: Mapper between domain pets and partner payloads/import candidates; uses `internal/clients/http/partner` (simple JSON POST with 5s timeout).

## Durable workflows (`internal/durable/temporal`)
- `workflows/pets/pet_creation_workflow.go`: Workflow `pets.workflows.Creation` on queue `PET_CREATION`; wraps the pet persistence sequence.
- `sequences/pet_persistence_sequence.go`: Executes the ordered `CreatePet` activity with retries/backoff.
- `activities/pets/pet.go`: Activity bundle that delegates to the pets service.

## Store bounded context (`internal/store`)
- Domain: `order.go` with status enum (`placed|approved|delivered`).
- Application: `service.go` supports `PlaceOrder`, `GetOrderByID`, `DeleteOrder`, and `Inventory` aggregation.
- Ports: Repository interface + `ErrNotFound`.
- Adapters: `adapters/memory` repository; `adapters/http/mapper` converts generated DTOs to domain orders.

## Users bounded context (`internal/users`)
- Domain: `user.go` entity.
- Application: `service.go` covers create (single/batch), update, delete, get by username, and a simple login/logout that stores tokens in-memory.
- Ports: Repository interface + `ErrNotFound`.
- Adapters: `adapters/memory` repository; `adapters/http/mapper` bridges transport models.

## Platform and shared
- `internal/platform/observability`: Slog JSON logger, OTLP HTTP exporter (configurable via `OTEL_EXPORTER_OTLP_ENDPOINT`/`OTEL_EXPORTER_OTLP_INSECURE`), tracer/meter providers, global propagator setup, and shutdown hook.
- `internal/platform/postgres`: `Connect` helper that opens a GORM connection and pings with timeout.
- `internal/shared/projection`: Generic projection wrapper carrying created/updated timestamps.

## Environment reference
- `PORT`: HTTP bind port (default `8080`).
- `POSTGRES_DSN`: Enables Postgres-backed pets repository; missing/invalid DSN falls back to memory with warnings.
- `TEMPORAL_ADDRESS` (default Temporal frontend), `TEMPORAL_NAMESPACE` (default `default`), `TEMPORAL_DISABLED=1` to force inline pet creation.
- `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_INSECURE`, `ENVIRONMENT`: Observability config.

## Running
- API: `go run ./cmd/api` (optionally set `PORT`, `POSTGRES_DSN`, `TEMPORAL_*`).
- Worker: `go run ./cmd/worker` when using Temporal workflows.
- Docs: Visit `/swagger` for interactive UI or `/openapi.(json|yaml)` for raw specs.
