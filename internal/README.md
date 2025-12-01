# Internal Package Structure

This document describes the internal package organization following the **Vertical Modules with Internal Sub-layers** pattern (Option 3).

## Overview

The `internal/` directory is organized into:

1. **Feature Modules** - Vertical slices representing business domains (`pets`, `users`, `store`)
2. **Platform** - Infrastructure and cross-cutting technical concerns
3. **Shared** - Generic utilities and types used across modules

## Directory Layout

```
internal/
├── pets/                    # Pet management feature module
│   ├── domain/              # Core domain types and business logic
│   ├── http/                # HTTP transport layer (mappers, DTOs)
│   ├── ports/               # Interface definitions (Repository, Workflow)
│   ├── repository/          # Data persistence implementations
│   │   ├── memory/          # In-memory storage for dev/tests
│   │   └── postgres/        # PostgreSQL persistence
│   ├── service/             # Application/use-case layer
│   │   └── types/           # Service-level DTOs and input types
│   ├── external/            # External system adapters
│   │   └── partner/         # Partner API mapper
│   └── workflows/           # Workflow orchestration adapters
│
├── users/                   # User management feature module
│   ├── domain/              # User domain types
│   ├── http/                # HTTP transport layer
│   ├── ports/               # Interface definitions
│   ├── repository/          # Data persistence
│   │   └── memory/          # In-memory storage
│   └── service/             # Application layer
│
├── store/                   # Store/Order feature module
│   ├── domain/              # Order domain types
│   ├── http/                # HTTP transport layer
│   ├── ports/               # Interface definitions
│   ├── repository/          # Data persistence
│   │   └── memory/          # In-memory storage
│   └── service/             # Application layer
│
├── platform/                # Infrastructure/technical concerns
│   ├── db/                  # Database connectivity (PostgreSQL)
│   ├── observability/       # OpenTelemetry, logging, metrics
│   ├── temporal/            # Temporal workflow infrastructure
│   │   ├── activities/      # Activity implementations
│   │   │   └── pets/        # Pet-related activities
│   │   ├── sequences/       # Activity sequences
│   │   └── workflows/       # Workflow definitions
│   │       └── pets/        # Pet-related workflows
│   └── clients/             # External HTTP clients
│       └── http/
│           └── partner/     # Partner API client
│
└── shared/                  # Cross-cutting utilities
    └── projection/          # Generic projection types with metadata
```

## Feature Module Structure

Each feature module follows a consistent internal structure:

| Sub-package | Purpose |
|-------------|---------|
| `domain/` | Core domain entities, value objects, and business rules |
| `http/` | HTTP transport: request/response DTOs and mappers |
| `ports/` | Interface definitions for repositories and external dependencies |
| `repository/` | Concrete implementations of persistence interfaces |
| `service/` | Application layer: use cases, orchestration, and service types |
| `external/` | Adapters for external systems (optional, feature-specific) |
| `workflows/` | Workflow orchestration adapters (optional, feature-specific) |

## Dependency Rules

The following dependency rules ensure proper layering and prevent circular imports:

### Allowed Dependencies

```
┌─────────────────────────────────────────────────────────────┐
│                        Feature Modules                       │
│  (pets, users, store)                                        │
│                                                              │
│  http/ ──────► service/ ──────► domain/                     │
│    │              │                │                         │
│    │              ▼                ▼                         │
│    │           ports/ ◄──────── repository/                  │
│    │              │                                          │
│    ▼              ▼                                          │
│  ┌────────────────────────────────────────────────────────┐ │
│  │                    shared/                              │ │
│  └────────────────────────────────────────────────────────┘ │
│  ┌────────────────────────────────────────────────────────┐ │
│  │                   platform/                             │ │
│  └────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

### Rules

1. **Feature modules** (`pets/`, `users/`, `store/`) may import from:
   - `shared/` - Generic utilities and cross-cutting types
   - `platform/` - Infrastructure services (db, observability, clients)
   - Their own sub-packages

2. **`shared/`** must NOT depend on:
   - Any feature module
   - `platform/` (to keep it truly generic)

3. **`platform/`** must NOT depend on:
   - Any feature module's domain or service layers
   - Note: `platform/temporal/` may reference feature module types for workflow definitions

4. **Within a feature module**:
   - `domain/` has no internal dependencies (pure domain logic)
   - `ports/` depends only on `domain/` and `service/types/`
   - `service/` depends on `domain/`, `ports/`, and `shared/`
   - `http/` depends on `service/types/` and `domain/`
   - `repository/` depends on `domain/`, `ports/`, and `shared/`

## Key Packages

### platform/db
PostgreSQL database connectivity using GORM.

### platform/observability
OpenTelemetry setup for distributed tracing, metrics, and structured logging.

### platform/temporal
Temporal workflow infrastructure including activities and workflow definitions.

### platform/clients
HTTP clients for external APIs (e.g., partner integrations).

### shared/projection
Generic projection type with metadata (CreatedAt, UpdatedAt) used across repositories.

## Migration Notes

This structure was refactored from a previous hexagonal architecture layout:
- `adapters/http/` → `http/`
- `adapters/memory/` → `repository/memory/`
- `adapters/persistence/postgres/` → `repository/postgres/`
- `adapters/external/` → `external/`
- `adapters/workflows/` → `workflows/`
- `application/` → `service/`
- `durable/temporal/` → `platform/temporal/`
- `clients/` → `platform/clients/`
- `platform/postgres/` → `platform/db/`

All import paths have been updated accordingly.
