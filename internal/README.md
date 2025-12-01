# Internal Package Structure

This directory contains all private/internal packages for the Petstore API server.

## Directory Layout

```
internal/
├── pets/          # Pet domain feature module
├── users/         # User domain feature module
├── store/         # Store/Order domain feature module
├── infra/         # Infrastructure and cross-cutting concerns
│   ├── platform/  # Platform-level infrastructure (observability, database)
│   ├── durable/   # Durable execution (Temporal workflows)
│   └── clients/   # External HTTP clients
└── shared/        # Shared utilities and types used across features
```

## Module Categories

### Feature Modules (`pets/`, `users/`, `store/`)

Domain-specific modules following hexagonal/clean architecture:
- `domain/` - Core domain entities and business logic
- `application/` - Use cases and application services
- `ports/` - Interfaces for external dependencies
- `adapters/` - Implementations of ports (HTTP handlers, repositories, etc.)

### Infrastructure (`infra/`)

Cross-cutting infrastructure code that supports feature modules:
- **`platform/`** - Observability (OpenTelemetry tracing, logging) and database connectivity
- **`durable/`** - Temporal workflow definitions, activities, and sequences
- **`clients/`** - HTTP clients for external partner APIs

### Shared (`shared/`)

Generic utilities and types used across multiple feature modules:
- **`projection/`** - Generic projection types for persistence metadata

## Dependency Rules

The following dependency directions are allowed:

```
┌─────────────────────────────────────────────────┐
│                  Feature Modules                │
│           (pets, users, store)                  │
├─────────────────────────────────────────────────┤
                       │
                       ▼
┌─────────────────────────────────────────────────┐
│        shared/          │       infra/          │
│  (generic utilities)    │  (infrastructure)     │
└─────────────────────────────────────────────────┘
```

### Rules:
1. **Feature modules** may depend on `shared/` and `infra/` packages
2. **`shared/`** must NOT depend on feature modules or `infra/`
3. **`infra/`** packages may depend on `shared/` and other `infra/` packages
4. **`infra/`** must NOT depend on feature modules (except for types in `application/types`)

## Import Paths

Use the full import paths for moved packages:

```go
// Platform observability
import "github.com/GIT_USER_ID/GIT_REPO_ID/internal/infra/platform/observability"

// Platform postgres
import "github.com/GIT_USER_ID/GIT_REPO_ID/internal/infra/platform/postgres"

// Durable workflows
import "github.com/GIT_USER_ID/GIT_REPO_ID/internal/infra/durable/temporal/workflows/pets"

// External clients
import "github.com/GIT_USER_ID/GIT_REPO_ID/internal/infra/clients/http/partner"

// Shared projection
import "github.com/GIT_USER_ID/GIT_REPO_ID/internal/shared/projection"
```
