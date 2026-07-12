---
name: nucleus-api-project
description: Codex project skill for Nucleus API. Use when initializing this Go API template, renaming nucleus-api placeholders, or scaffolding a new REST CRUD resource across domain, infrastructure, app, presentation, Wire, migrations, mocks and tests.
---

# Nucleus API Project

Use this skill when the user asks Codex to initialize this template or create a
new API resource in this repository. Prefer the repository's local docs over
memory.

Always read `AGENTS.md` and `CLAUDE.md` first. Then read the workflow-specific
guide:

- Initialization: `README.md`, section "Initializing the project".
- New resource: `docs/new-resource-guide.md`.
- Migration work: `docs/migrations.md`.

## Codex Workflow Rules

- Use `rg` or `rg --files` for discovery.
- Use `apply_patch` for manual edits.
- Keep `.claude/skills` intact; those files are for Claude-compatible workflows.
- Preserve unrelated user changes.
- After editing Wire providers, run `go generate ./cmd/api/di`.
- After adding or changing a mocked interface, run `mockery`.
- Before finishing, run `go build ./...` and `make test` when feasible.
- If `make test` cannot run because Docker/Testcontainers is unavailable,
  report that explicitly.

## Initialize Project

Run this once on a fresh copy of the template.

1. Check whether the project is still uninitialized:

   ```bash
   head -1 go.mod
   ```

   If the module is no longer `github.com/scienceandcode/nucleus-api`, tell the
   user the project already appears initialized and stop unless they requested a
   second rename.

2. Ask for:

   - target Go module path, for example `github.com/acme/billing-api`;
   - display app name, for example `Billing API`;
   - short lowercase name for database and container values, for example
     `billing`;
   - image repository, optional, defaulting to `<owner>/<repo>` from the module;
   - DB name/user/password, optional, defaulting to the short name.

3. Derive the kebab project name from the module's final path segment.

4. Replace placeholders from most specific to least specific:

   - `github.com/scienceandcode/nucleus-api` -> module path in Go imports,
     `go.mod`, and `.mockery.yml`.
   - `scienceandcode/nucleus-api` -> image repository in
     `.github/workflows/*.yml`.
   - `nucleus-api` -> kebab project name in binary, Docker, CI, and natural docs
     text.
   - `nucleus_db` -> `<short>_db` in `docker-compose.yml` and
     `internal/infrastructure/di/test/{provider.go,db.go}`.
   - `Nucleus API` -> display name in `.env`, `.env.example`, and natural docs
     text.
   - DB values in `.env` and `.env.example`: set `DB_NAME`, `DB_USER`, and
     `DB_PASSWORD` to the chosen values.

   Do not blind-replace every standalone `nucleus`.

5. Verify:

   ```bash
   go mod tidy
   go generate ./cmd/api/di
   go build ./...
   make test
   ```

## Add API Resource

Use `Task` as the source pattern. Copy its shape through every layer and adapt
names, fields, ownership, validation, and routes.

1. Clarify before editing:

   - singular resource name;
   - fields, Go types, JSON names, required fields, enum/status values;
   - whether rows are user-owned like `Task` or global;
   - route list and which routes require `r.authProtected()`;
   - validation and business rules.

2. Implement every layer:

   - `internal/domain/<resource>/entities/<resource>_entity.go`
   - `internal/domain/<resource>/interfaces/<resource>_repository_interface.go`
   - `internal/infrastructure/db/models/<resource>.go`
   - `internal/infrastructure/db/mappers/<resource>_mapper.go`
   - `internal/infrastructure/db/repositories/<resource>_repository.go`
   - migration registration in
     `internal/infrastructure/db/migrations/model_migrations.go`
   - `internal/app/messages/<resource>_dto.go`
   - `internal/app/mappers/<resource>_mapper.go`
   - `internal/app/interfaces/<resource>_service_interface.go`
   - `internal/app/services/<resource>_service.go`
   - `internal/presentation/api/handlers/<resource>_handler.go`
   - `internal/presentation/api/routers/<resource>_router.go`

3. Update registries and DI:

   - Add the handler field in `internal/presentation/api/handlers/handlers.go`.
   - Register routes in `internal/presentation/api/routers/router.go`.
   - Add repository and service providers under `internal/infrastructure/di`.
   - Add providers and `wire.Struct(new(handlers.<Resource>Handler), "*")` in
     `cmd/api/di/wire.go`.
   - Update `.mockery.yml` if a new interface needs generated mocks.

4. Preserve conventions:

   - package names are singular lowercase;
   - JSON fields are camelCase;
   - external IDs use `messages.Uint64String`;
   - repositories use `r.getDB(ctx)`;
   - user-owned services use `common.ExtractUserIdFromContext(ctx)`;
   - multi-write operations use `s.WithTransaction(ctx, ...)`;
   - migrations are idempotent;
   - handlers use `ParseRequest`, `strconv.ParseUint`, and response helpers;
   - handlers map not found with `errors.Is(err, appErrors.ErrNotFound)`.

5. Add focused service tests. For user-owned resources, include a test proving
   one user cannot access another user's rows.

6. Verify:

   ```bash
   go generate ./cmd/api/di
   mockery
   go build ./...
   make test
   ```

   Skip `mockery` only when no mocked interface changed.
