# Pet Domain Abstraction Layers

Text-only map of how the pets bounded context is structured and wired.

```text
API layer (handlers) 
  -> HTTP mapper (internal/domains/pets/adapters/http/mapper)
  -> Observability decorator (internal/domains/pets/adapters/observability)
  -> Application service (internal/domains/pets/application/service.go)
        - orchestrates use cases
        - maps domain errors to application errors
        - triggers partner sync
        - delegates persistence via ports.Repository
        - optional workflow delegation via ports.WorkflowOrchestrator
  -> Domain model (internal/domains/pets/domain/pet.go)
        - Pet aggregate + invariants (name/photos/status/hair/tags/category/external ref)
        - value objects and helpers
  -> Outbound ports (internal/domains/pets/ports)
        - Repository (persistence)
        - PartnerSync (external system)
        - WorkflowOrchestrator (durable orchestration)
        - Service (inbound API surface)
  -> Adapters (internal/domains/pets/adapters)
        - persistence/postgres: GORM/PostgreSQL with projections
        - persistence/memory: in-memory repo for tests/dev
        - workflows: Temporal client or inline
        - external/partner: HTTP client wrapper
        - observability: tracing/logging/metrics decorator
```

### Typical request flow
- Inbound (create): HTTP handler reads optional `Idempotency-Key`, fingerprints the payload, replays from the idempotency store when present, and rejects mismatched payloads with HTTP 409; otherwise it maps JSON → `mapper.ToMutationInput` → observability decorator → **workflow orchestrator** (Temporal by default, inline when `TEMPORAL_DISABLED=true`) → Temporal `PersistPet` activity (persist only) → Temporal `SyncPetWithPartner` activity → return projection. Workflow IDs are derived from the idempotency key to avoid duplicate runs.
- Inbound (other ops): HTTP handler → mapper → observability decorator → `application.Service` → repo/partner sync as needed.
- Mutations may also invoke `PartnerSync.Sync` and/or `WorkflowOrchestrator.CreatePet` depending on wiring.
- Reads: repository port returns projections wrapping domain aggregates (`application/types/projections.go`).

### Key abstractions by layer
- Domain: `domain.Pet` + invariants, normalization for status/tags/category (`internal/domains/pets/domain/pet.go`).
- Application: use cases and error mapping (`internal/domains/pets/application/service.go`, `errors.go`); command/query DTOs live in `internal/domains/pets/application/types`.
- Ports: inbound (`ports.Service`) and outbound (`ports.Repository`, `ports.PartnerSync`, `ports.WorkflowOrchestrator`) in `internal/domains/pets/ports`.
- Adapters:
  - Persistence: `adapters/persistence/postgres` (GORM, arrays/JSON), `adapters/memory` (tests/dev).
  - Idempotency store: Postgres table `pet_idempotency_keys` or in-memory fallback for the `Idempotency-Key` header.
  - Observability: `adapters/observability` decorator.
  - Workflows: `adapters/workflows` (Temporal or inline).
  - External partner client: `adapters/external/partner`.
  - HTTP mapping: `adapters/http/mapper`.
- Platform plumbing: PostgreSQL connection helper (`internal/platform/postgres`), migrations (`internal/platform/migrations`), Temporal workflow definitions (`internal/platform/temporal/workflows/pets`).

### Testing hooks
- Unit/service coverage: `internal/domains/pets/application/service_test.go`.
- Integration (DB): `internal/domains/pets/adapters/persistence/postgres/repository_integration_test.go` (arrays/JSON, tag search).
- In-memory repo and inline workflows simplify isolated tests.

### End-to-end create flow (happy path)
1) User sends `POST /pet` with JSON body (id, name, photos, optional tags/status/category/externalRef) and optional `Idempotency-Key`.
2) HTTP handler fingerprints the payload; if the key exists with the same hash, it reloads the stored pet and returns it; if the hash differs, it returns 409.
3) When proceeding, handler maps JSON → `mapper.MutationPet` → `mapper.ToMutationInput` producing `application/types.AddPetInput` and passes the idempotency key along.
4) Observability decorator starts span/logs/metrics, then calls `WorkflowOrchestrator.CreatePet`.
5) Temporal path (default unless `TEMPORAL_DISABLED=true`):
   - Orchestrator starts `PetCreationWorkflow` with the command and trace context.
   - Workflow runs `RunPetPersistenceSequence`:
     - `PersistPet` activity: uses a service instance without partner sync to validate/build the aggregate and `repo.Save`.
     - `SyncPetWithPartner` activity: reloads the pet and calls the partner API (POST) with a separate retry policy; uses Temporal activity heartbeat to avoid re-sending after a successful attempt; skipped if no partner configured.
6) Inline path (fallback when disabled): orchestrator directly calls `application.Service.AddPet`.
7) Service builds domain aggregate via `buildPetFromMutation`:
   - `domain.NewPet` enforces name/non-empty photos.
   - Applies status/category/tags/hair/externalRef with validation/normalization.
8) Service calls `repo.Save` (ports.Repository):
   - In prod, `adapters/persistence/postgres.Repository` upserts via GORM; arrays/JSON columns for photos/tags/external attributes; returns projection with timestamps.
   - In tests/dev, `adapters/memory.Repository` stores in-memory with metadata.
9) Idempotency store captures key → (request hash, pet id) after persistence to enable retries without reprocessing.
10) Partner sync:
    - Temporal path: separate `SyncPetWithPartner` activity reloads and syncs via POST; heartbeat prevents duplicate sends after success; skipped if no partner.
    - Inline path: `application.Service.AddPet` calls `PartnerSync.Sync` directly (also POST).
11) Observability decorator records success metrics/logs and returns the projection to the workflow/inline caller.
12) HTTP handler maps projection → API response JSON (id, name, status, photos, tags, category, externalRef, createdAt/updatedAt).
13) User receives 200/201 with the created pet payload.

Error variants:
- Domain validation (empty name/photos, invalid status/hair/grooming) → mapped to `ErrInvalidInput` → 400-level response.
- Repo not found (during updates) → `ports.ErrNotFound` → 404.
- Idempotency conflict (same key, different payload) → `ErrIdempotencyConflict` → 409; returns stored projection when hashes match.
- Partner sync failure after save → response includes saved pet; sync error surfaces (e.g., 502) while persistence is kept.
