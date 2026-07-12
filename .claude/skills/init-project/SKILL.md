---
name: init-project
description: Initialize a project created from the nucleus-api template — rename every "nucleus-api" placeholder to the real project name (module path, app name, DB, Docker, CI), then verify the build. Use right after cloning the template.
---

# Initialize project from the nucleus-api template

This skill turns the `nucleus-api` template into a real project by replacing
every placeholder. Run it once, in a freshly-cloned copy of the template.

## Step 1 — Confirm this is a fresh template

Check that the Go module is still `github.com/scienceandcode/nucleus-api`:

```bash
head -1 go.mod
```

If it is already renamed, tell the user the project looks initialized and stop
(unless they explicitly ask to rename again).

## Step 2 — Gather the target names

Ask the user (use the AskUserQuestion tool) for:

1. **Module path** — the new Go module, e.g. `github.com/acme/billing-api`.
2. **Display name** — human-readable app name, e.g. `Billing API`
   (becomes `APP_NAME`).
3. **Short name** — a lowercase token for the database and Docker container,
   e.g. `billing` (used as `<short>` and `<short>_db`).
4. **Container image repository** (optional) — the ECR/registry repo for CI,
   e.g. `acme/billing-api`. Default: derive from the module path
   (`<owner>/<repo>`).

Derive the **kebab project name** from the last path segment of the module
(e.g. `billing-api`).

## Step 3 — Apply the replacements (ORDER MATTERS)

Replace from the most specific string to the least specific, so a bare
`nucleus` never corrupts a longer match.

1. Module path — every `.go` file plus `go.mod` and `.mockery.yml`:
   ```bash
   grep -rl 'scienceandcode/nucleus-api' --include='*.go' . \
     | xargs sed -i 's#github.com/scienceandcode/nucleus-api#<MODULE_PATH>#g'
   sed -i 's#github.com/scienceandcode/nucleus-api#<MODULE_PATH>#' go.mod .mockery.yml
   ```
2. CI image repo — `.github/workflows/*.yml`:
   `scienceandcode/nucleus-api` → `<IMAGE_REPO>`.
3. Docker binary name — `build/docker/Dockerfile`: `nucleus-api` → `<KEBAB_NAME>`.
4. DB container name — `docker-compose.yml` and
   `internal/infrastructure/di/test/{provider.go,db.go}`:
   `nucleus_db` → `<SHORT>_db`.
5. App display name — `.env` and `.env.example`: `Nucleus API` → `<DISPLAY_NAME>`.
6. Database name/credentials — `.env` and `.env.example`: set `DB_NAME`,
   `DB_USER`, `DB_PASSWORD` to `<SHORT>` (or values the user prefers).
7. Docs — update the obvious `nucleus-api` / `Nucleus API` mentions in
   `README.md`, `CLAUDE.md`, `AGENTS.md` so they read naturally. The placeholder
   table in `README.md` can be removed once initialization is done.

Do NOT blind-replace a standalone `nucleus`; only touch it in `.env*` and
`docker-compose.yml` as described above.

## Step 4 — Verify

```bash
go mod tidy
go generate ./cmd/api/di
go build ./...
```

Optionally, if Docker is available: `make test`.

## Step 5 — Finish

- Confirm `head -1 go.mod` shows the new module path.
- Tell the user the project is initialized and they can start with
  `docs/new-resource-guide.md`.
- Mention that this `init-project` skill can now be deleted
  (`rm -rf .claude/skills/init-project`).
