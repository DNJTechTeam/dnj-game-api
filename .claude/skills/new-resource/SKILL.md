---
name: new-resource
description: Scaffold a new CRUD resource in this Go API across every layer (domain, infrastructure, app, presentation), wire it with Google Wire, add a migration and tests — following the existing Task resource pattern.
---

# Scaffold a new resource

Use this skill to add a CRUD resource to the API. It follows the project's
layered architecture and the worked example in `docs/new-resource-guide.md`.

## Step 1 — Read the guide and the reference resource

1. Read `docs/new-resource-guide.md` end to end — it is the authoritative
   step-by-step.
2. Read the existing `Task` resource; it is the pattern to copy:
   - `internal/domain/task/{entities,interfaces}`
   - `internal/infrastructure/db/{models,mappers,repositories}/task*`
   - `internal/app/{messages,mappers,interfaces,services}/task*`
   - `internal/presentation/api/handlers/task_handler.go`
   - `internal/presentation/api/routers/task_router.go`

## Step 2 — Clarify the resource

Confirm with the user:

- Resource name (singular, e.g. `Invoice`).
- Fields and types; any status/enum values.
- Whether rows are **user-owned** (scoped to the JWT user, like `Task`) or
  global.
- Which routes are needed and which require authentication.

## Step 3 — Implement every layer

Follow `docs/new-resource-guide.md` steps 1–15, copying the `Task` files and
adapting them. Do not skip the migration (idempotent — see `docs/migrations.md`)
or the Wire providers.

## Step 4 — Regenerate and verify

```bash
go generate ./cmd/api/di     # after editing Wire providers
mockery                      # if you added a new mocked interface
go build ./...
make test
```

All must be clean. Use the checklist at the bottom of
`docs/new-resource-guide.md` to confirm nothing was missed.
