# Nucleus API

A production-shaped **starter template** for REST APIs in Go. It ships a clean
layered architecture, dependency injection, PostgreSQL, identity-only JWT auth,
an AWS Lambda-ready runner, CI, and an example `Task` CRUD to copy from.

> `nucleus-api` is a **placeholder name**. After cloning, rename it to your real
> project — see [Initializing the project](#initializing-the-project).

## Stack

- **Go 1.24** · **Gin** (HTTP) · **GORM** (PostgreSQL)
- **Google Wire** for dependency injection
- **golang-jwt** for identity tokens (header *and* cookie)
- **Testcontainers** for integration tests against a real Postgres
- Runs as a local HTTP server **or** as an AWS Lambda (`SERVER_RUNNER`)

## Requirements

- Go 1.24+
- Docker (for the local DB and for the test suite)
- Optional: [`wire`](https://github.com/google/wire) and
  [`mockery`](https://github.com/vektra/mockery) for codegen

## Quick start

```bash
cp .env.example .env            # adjust values as needed
docker compose up -d db         # start PostgreSQL
make run-api                    # serve on http://localhost:8080
```

Smoke test:

```bash
curl http://localhost:8080/v1/healthcheck
# {"status":"ok"}
```

Register → confirm → log in → use the Task API:

```bash
# 1. Register
curl -X POST http://localhost:8080/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"me@example.com","name":"Me","password":"Passw0rd@2026","mobilePhone":"41999999999"}'

# On localhost the confirmation email is not sent; confirm directly in the DB:
#   UPDATE users SET email_confirmed_at = now() WHERE email = 'me@example.com';

# 2. Log in — returns the identity_token cookie AND accepts it as a header
curl -i -X POST http://localhost:8080/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"me@example.com","password":"Passw0rd@2026"}'

# 3. Use the token (header form)
curl http://localhost:8080/v1/tasks -H "Authorization: <token>"
```

## Running the tests

```bash
make test
```

The suite spins up a throwaway PostgreSQL container via Testcontainers, so
Docker must be running. No local database is touched.

## Initializing the project

This template uses the distinctive name **`nucleus-api`** everywhere so it can
be safely find-and-replaced. To turn the template into your project:

- **With Claude Code**: run the `init-project` skill — it asks for your names
  and applies every replacement, then builds.
- **Manually**: replace the placeholders below, then run
  `go mod tidy && go generate ./cmd/api/di && go build ./...`.

| Placeholder | Where it appears | Replace with |
|-------------|------------------|--------------|
| `github.com/scienceandcode/nucleus-api` | every Go import + `go.mod` + `.mockery.yml` | your module path |
| `nucleus-api` | directory name, `build/docker/Dockerfile` binary | your kebab-case project name |
| `scienceandcode/nucleus-api` | `IMAGE_REPOSITORY_NAME` in `.github/workflows/*.yml` | your ECR repository |
| `Nucleus API` | `APP_NAME` in `.env` / `.env.example` | your display name |
| `nucleus_db` | `docker-compose.yml`, `internal/infrastructure/di/test` (`DbContainerName`) | `<shortname>_db` |
| `nucleus` (standalone) | `DB_*` values in `.env` / `.env.example` | your database name/credentials |

> Replace the **longest** strings first (`github.com/scienceandcode/nucleus-api`,
> then `nucleus-api`, then `nucleus_db`) so a bare `nucleus` replacement never
> corrupts the longer ones.

## Environment variables

See `.env.example` for the full list with comments. Key ones:

- `SERVER_RUNNER` — `default` (local HTTP server) or `lambda` (AWS Lambda).
- `SERVER_ENVIRONMENT` — `localhost` | `development` | `production` | `test`.
- `JWT_IDENTITY_SECRET` — signs the identity JWT.
- `CORS_ALLOWED_ORIGINS` — comma-separated allowed origins.
- `DB_*` — PostgreSQL connection.

## The Lambda runner

`internal/infrastructure/api/runner.go` selects the runtime from `SERVER_RUNNER`:

- `default` → `gin.Engine.Run()` on `SERVER_PORT`.
- `lambda` → wraps the engine with `aws-lambda-go-api-proxy` and serves API
  Gateway HTTP API (payload v2) events.

The same binary works both ways — the CI builds a container image and updates
an AWS Lambda function from it (`.github/workflows/`).

## Documentation

- `CLAUDE.md` / `AGENTS.md` — guides for AI agents working in the repo.
- `docs/architecture.md` — layers and request flow.
- `docs/auth-and-tokens.md` — JWT, header vs cookie, middleware.
- `docs/migrations.md` — the migration system and idempotency rules.
- `docs/new-resource-guide.md` — step-by-step for adding a new resource.

## Project layout

```
cmd/api/            Application entrypoint + Wire DI
internal/
  app/              Use cases: services, DTOs, mappers, validators, errors
  domain/           Entities + repository interfaces (task, user, common)
  infrastructure/   Gin/runner, DB/GORM, migrations, DI providers, email
  presentation/     HTTP handlers and routers
  mocks/            Generated mocks (mockery)
build/docker/       Dockerfile
.github/workflows/  CI: test, build image, deploy to Lambda
```

## License

Use freely as a starting point for your own projects.
