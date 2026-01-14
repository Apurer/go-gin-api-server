# Project map

Updated layout for the Clean DDD Petstore that wraps the OpenAPI-generated Gin server.

```
go-gin-api-server/
- api/                       # OpenAPI contract (source of generated transport)
- cmd/
  - api/main.go              # HTTP API composition root
  - worker/main.go           # Temporal worker wiring
  - session-purger/main.go   # One-off session purge CLI
- go/                        # Generated Gin router + DTOs that call internal services
- internal/                  # Domain/application code by bounded context
  - domains/                 # Domain slices (bounded contexts)
    - pets/                  # Domain: Pets bounded context
    - store/                 # Domain: Store/orders bounded context
    - users/                 # Domain: Users bounded context
  - clients/                 # HTTP client stubs for partner integrations
  - platform/temporal/       # Temporal workflows/activities/sequences
  - platform/                # Shared platform concerns (OTEL, Postgres)
  - shared/                  # Cross-cutting helpers (projections)
- Dockerfile
- README.md
- description.md
- project.md                 # This file
```

**Domain slices** live at `internal/domains/pets`, `internal/domains/store`, and `internal/domains/users`; the other `internal/` packages provide platform, workflow, or integration support for those bounded contexts.

## Processes and transport

- `cmd/api/main.go` boots the HTTP API: loads OTEL instruments, builds repositories (Postgres when `POSTGRES_DSN` is set, otherwise memory), wires services into the generated handlers (`go/api_pet.go`, `go/api_store.go`, `go/api_user.go`), mounts middleware (otelgin), and listens on `:$PORT` (default `8080`). Pet creation can run inline or via Temporal if reachable. Optional session purge ticker when `SESSION_PURGE_INTERVAL_MINUTES` is set.
- `cmd/worker/main.go` registers the pet creation workflow and activities with Temporal and reuses the same pets service and repository wiring.
- `cmd/session-purger/main.go` is a one-off CLI to purge expired user sessions (Postgres only).
- `api/openapi.yaml` is the contract used by the generator. The router in `go/routers.go` also serves `/openapi.(json|yaml)` plus `/swagger` for UI.
- `go/` holds the generated Gin transport. Handlers delegate to application services and adapters; routes are bound in `go/routers.go`.

## Bounded contexts (internal)

### Pets (`internal/domains/pets`)
- `domain/`: Pet aggregate, value objects (category, tags, external reference), and invariants for grooming or hair length.
- `application/`: Use-case service with OTEL metrics and tracing; command/query inputs live in `application/types/`.
- `ports/`: Interfaces for repository and workflow orchestrator plus shared errors.
- `adapters/http/mapper`: Translates generated HTTP DTOs to application inputs and back.
- `adapters/memory`: In-memory repository used by default.
- `adapters/persistence/postgres`: GORM-backed repository with automigrations and projection mapping.
- `adapters/external/partner`: Mapper between domain pets and a sample partner schema.
- `adapters/workflows`: Workflow orchestrators (inline versus Temporal client).

### Store (`internal/domains/store`)
- `domain/`: Order aggregate and statuses.
- `application/`: Order service with inventory calculation.
- `ports/`: Repository interface and errors.
- `adapters/memory`: In-memory repository.
- `adapters/http/mapper`: Maps generated DTOs to domain orders.

### Users (`internal/domains/users`)
- `domain/`: User entity.
- `application/`: User service for CRUD and login flows.
- `ports/`: Repository interface and session store abstraction.
- `adapters/memory`: In-memory repository and session store.
- `adapters/persistence/postgres`: Postgres repository and session store (TTL via `SESSION_TTL_HOURS`, purge via ticker/CLI).
- `adapters/http/mapper`: Maps generated DTOs to domain users.

## Cross-cutting

- `internal/platform/temporal`: Pet creation workflow definition (`workflows/pets`), activity bundle (`activities/pets`), and the activity sequence used by the workflow (`sequences/`).
- `internal/clients/http/partner`: Minimal HTTP client for syncing pets to a partner API.
- `internal/platform/observability`: Slog and OpenTelemetry bootstrap (`Init`) exposing tracers and meters.
- `internal/platform/postgres`: Postgres connector used by repositories and processes.
- `internal/shared/projection`: Projection wrapper carrying metadata timestamps for repositories.

## Environment knobs

- `PORT`: HTTP bind port for the API process.
- `POSTGRES_DSN`: Enables Postgres-backed repositories/session store when set; otherwise defaults to memory.
- `SESSION_TTL_HOURS`: TTL for user sessions (default 24h).
- `SESSION_PURGE_INTERVAL_MINUTES`: When set, API process purges expired sessions on a ticker.
- `TEMPORAL_ADDRESS`, `TEMPORAL_NAMESPACE`: Temporal connection for workflows and worker (defaults to local frontend plus the `default` namespace).
- `TEMPORAL_DISABLED`: Set to `1` to force inline pet creation without Temporal.
- `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_INSECURE`, `ENVIRONMENT`: Observability exporter and metadata used by platform instrumentation.
