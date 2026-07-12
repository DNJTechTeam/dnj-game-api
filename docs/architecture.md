# Architecture

Nucleus API follows a layered architecture with dependency injection. Each
layer depends only on the layer(s) below it and talks across boundaries
through interfaces.

## Layers

```
presentation/  ── HTTP: handlers + routers (Gin)
      │
app/           ── use cases: services, DTOs (messages), mappers, validators
      │
domain/        ── entities + repository interfaces (no framework imports)
      │
infrastructure ── concrete impls: GORM repositories, DB, email, runner, DI
```

- **domain** — pure Go structs (entities) and repository interfaces. No Gin, no
  GORM. `domain/common/interfaces` holds cross-cutting contracts
  (`TransactionManagerInterface`, `ModelInterface`).
- **app** — business logic. Services receive request DTOs (`app/messages`),
  return response DTOs, and depend on repository/service *interfaces*. Mappers
  (`app/mappers`) convert entities to response DTOs.
- **infrastructure** — concrete implementations: GORM models + repositories,
  the PostgreSQL connection, migrations, the email client, the Gin/Lambda
  runner, and the Wire providers.
- **presentation** — Gin handlers (parse request → call service → write
  response) and routers (URL → handler, with middleware).

## Request flow

```
HTTP request
  └─ router (internal/presentation/api/routers)
       └─ middleware: AuthenticationMiddleware  (for authProtected routes)
            └─ handler (internal/presentation/api/handlers)
                 └─ service (internal/app/services)
                      └─ repository (internal/infrastructure/db/repositories)
                           └─ GORM → PostgreSQL
```

The handler only deals with HTTP (binding, status codes). The service holds the
rules. The repository owns persistence and maps Model ↔ Entity.

## Dependency injection (Google Wire)

`cmd/api/di/wire.go` declares the provider set; `wire_gen.go` is the generated
constructor. Providers live in `internal/infrastructure/di/`:

- `di/db` — the `*gorm.DB` and the transaction manager.
- `di/db/repositories` — one provider per repository, returning the domain
  interface.
- `di/services` — one provider per service.
- `di/api` — the Gin engine and the router.

After changing providers, regenerate with `go generate ./cmd/api/di`
(or `make run-wire`).

## Transactions

`BaseService.WithTransaction(ctx, fn)` runs `fn` inside a DB transaction. The
transaction is stored in the context; repositories call `r.getDB(ctx)` which
returns the transaction when present, the base connection otherwise. Nested
`WithTransaction` calls join the outer transaction (only the outermost commits).

## Key building blocks

| Concern | File |
|---------|------|
| Generic CRUD repository | `internal/infrastructure/db/repositories/base_repository.go` |
| Transaction helper | `internal/app/services/base_service.go` |
| Typed errors | `internal/app/errors/` |
| String-encoded IDs | `internal/app/messages/uint64_string.go` |
| HTTP response helpers | `internal/presentation/api/handlers/functions.go` |
| Env / context helpers | `internal/infrastructure/common/helpers.go` |
| Local vs Lambda runner | `internal/infrastructure/api/runner.go` |

## The example resource: Task

`Task` is implemented across every layer and is the canonical pattern to copy.
See `new-resource-guide.md`.
