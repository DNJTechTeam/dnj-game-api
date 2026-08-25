WIRE_DIR=cmd/api/di

COVER_MIN ?= 90
COVER_MAINTAINED_MIN ?= 55
COVER_PROFILE ?= coverage.out
COVER_GATE_PROFILE ?= coverage.gate.out
COVER_MAINTAINED_PROFILE ?= coverage.maintained.out
ADMIN_COVER_PROFILE ?= /tmp/dnj-admin-coverage.out
ADMIN_SERVICE_COVER_MIN ?= 91.4
ADMIN_SLICE_COVER_MIN ?= 92.1
ITERATION5_COVER_PROFILE ?= /tmp/dnj-iteration5-coverage.out
ITERATION5_SERVICE_COVER_MIN ?= 97.4
ITERATION5_SLICE_COVER_MIN ?= 96.0
ITERATION6_COVER_PROFILE ?= /tmp/dnj-iteration6-coverage.out
ITERATION6_SERVICE_COVER_MIN ?= 90
ITERATION6_SLICE_COVER_MIN ?= 90
ITERATION7_COVER_PROFILE ?= /tmp/dnj-iteration7-coverage.out
ITERATION7_SERVICE_COVER_MIN ?= 90
ITERATION7_SLICE_COVER_MIN ?= 90
ITERATION8_COVER_PROFILE ?= /tmp/dnj-iteration8-coverage.out
ITERATION8_SERVICE_COVER_MIN ?= 90
ITERATION8_SLICE_COVER_MIN ?= 90
COVER_PKGS=./...
COVER_TEST_PKGS=./internal/...
COVER_IGNORE_REGEX=^github.com/dnjtechteam/dnj-game-api/cmd/|/internal/mocks/|/internal/infrastructure/di/|/internal/infrastructure/api/runner.go:|/internal/domain/.*/(entities|interfaces)/|/internal/infrastructure/db/models/|/internal/presentation/api/routers/
COVER_GATE_REGEX=^github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/(mappers|repositories)/

OPENAPI_DIR=docs/openapi

.PHONY: wire build run run-api media-worker vet tidy migrate openapi openapi-v1 openapi-v2 openapi-check validate \
        test test-cover test-cover-check test-cover-html coverage \
        test-services test-repos test-migrations test-race test-admin-cover-check test-iteration5-cover-check test-iteration6-cover-check test-iteration7-cover-check test-iteration8-cover-check \
        db-up db-down db-reset s3-up local-up local-down docker-build

# ── Build ──────────────────────────────────────────────────────────────────
wire:
	go generate ./$(WIRE_DIR)/...

build:
	go build ./...

# openapi-v1 regenerates docs/openapi/swagger.{json,yaml} (OpenAPI/Swagger 2.0 —
# the swaggo/swag CLI does not emit 3.x yet) from @-annotations on the HTTP
# handlers (see cmd/api/main.go for the general API annotations). The output
# is committed to the repo — CI fails the PR if it drifts from the code (see
# .github/workflows/pr.yml, job "openapi").
openapi-v1:
	go tool swag init -g cmd/api/main.go -o $(OPENAPI_DIR) \
		--outputTypes json,yaml --parseDependency --parseInternal

openapi-v2:
	go run ./cmd/openapi-check --write-json $(OPENAPI_DIR)/dnj-v2.openapi.json

openapi-check:
	go run ./cmd/openapi-check

openapi: openapi-v1 openapi-v2 openapi-check

vet:
	go vet ./...

tidy:
	go mod tidy

# ── Run ────────────────────────────────────────────────────────────────────
run:
	go run cmd/api/main.go

run-api: migrate run

media-worker:
	go run cmd/media-worker/main.go

migrate:
	go run cmd/migrate/main.go

# ── Test ───────────────────────────────────────────────────────────────────
test:
	go test ./... -count=1

test-race:
	TZ=UTC go test -race ./... -count=1

test-cover test-cover-check coverage:
	go test $(COVER_TEST_PKGS) -count=1 -coverprofile=$(COVER_PROFILE) -covermode=atomic -coverpkg=$(COVER_PKGS)
	@awk -v ignore='$(COVER_IGNORE_REGEX)' 'NR==1 || $$1 !~ ignore { print }' $(COVER_PROFILE) > $(COVER_MAINTAINED_PROFILE)
	@echo "Maintained-code coverage (generated/runtime glue excluded):"
	@go tool cover -func=$(COVER_MAINTAINED_PROFILE)
	@awk -v ignore='$(COVER_IGNORE_REGEX)' -v gate='$(COVER_GATE_REGEX)' 'NR==1 || ($$1 !~ ignore && $$1 ~ gate) { print }' $(COVER_PROFILE) > $(COVER_GATE_PROFILE)
	@echo "Coverage gate scope: $(COVER_GATE_REGEX)"
	@go tool cover -func=$(COVER_GATE_PROFILE)
	@total=$$(go tool cover -func=$(COVER_GATE_PROFILE) | awk '/^total:/ { gsub("%","",$$3); print $$3 }'); \
	awk -v total="$$total" -v min="$(COVER_MIN)" 'BEGIN { if (total + 0 < min + 0) { printf("coverage %.1f%% is below required %.1f%%\n", total, min); exit 1 } printf("coverage %.1f%% meets required %.1f%%\n", total, min) }'
	@maintained=$$(go tool cover -func=$(COVER_MAINTAINED_PROFILE) | awk '/^total:/ { gsub("%","",$$3); print $$3 }'); \
	awk -v total="$$maintained" -v min="$(COVER_MAINTAINED_MIN)" 'BEGIN { if (total + 0 < min + 0) { printf("maintained-code coverage %.1f%% is below required %.1f%%\n", total, min); exit 1 } printf("maintained-code coverage %.1f%% meets required %.1f%%\n", total, min) }'

test-cover-html: test-cover
	go tool cover -html=$(COVER_PROFILE) -o coverage.html

test-services:
	go test ./internal/app/services/... -count=1 -v

test-repos:
	go test ./internal/infrastructure/db/repositories/... -count=1 -v

test-migrations:
	go test ./internal/infrastructure/db/migrations/... -count=1 -v -timeout 180s

test-admin-cover-check:
	go test ./internal/app/services ./internal/presentation/api/handlers ./internal/infrastructure/db/mappers ./internal/infrastructure/db/repositories \
		-run 'TestAdminInstallation|TestAdminOperationMappers|TestIteration4AdminRepositories' -count=1 \
		-coverprofile=$(ADMIN_COVER_PROFILE) \
		-coverpkg=./internal/app/services,./internal/presentation/api/handlers,./internal/infrastructure/db/mappers,./internal/infrastructure/db/repositories
	bash scripts/check-admin-coverage.sh $(ADMIN_COVER_PROFILE) $(ADMIN_SERVICE_COVER_MIN) $(ADMIN_SLICE_COVER_MIN)

test-iteration5-cover-check:
	TZ=UTC go test ./internal/app/services ./internal/app/mappers ./internal/presentation/api/handlers ./internal/infrastructure/db/mappers ./internal/infrastructure/db/repositories \
		-run 'TestIteration5' -count=1 \
		-coverprofile=$(ITERATION5_COVER_PROFILE) \
		-coverpkg=./internal/app/services,./internal/app/mappers,./internal/presentation/api/handlers,./internal/infrastructure/db/mappers,./internal/infrastructure/db/repositories
	bash scripts/check-iteration5-coverage.sh $(ITERATION5_COVER_PROFILE) $(ITERATION5_SERVICE_COVER_MIN) $(ITERATION5_SLICE_COVER_MIN)

test-iteration6-cover-check:
	go test ./internal/app/services ./internal/app/mappers ./internal/presentation/api/handlers ./internal/infrastructure/db/mappers ./internal/infrastructure/db/repositories \
		-run 'TestIteration6' -count=1 \
		-coverprofile=$(ITERATION6_COVER_PROFILE) \
		-coverpkg=./internal/app/services,./internal/app/mappers,./internal/presentation/api/handlers,./internal/infrastructure/db/mappers,./internal/infrastructure/db/repositories
	bash scripts/check-iteration6-coverage.sh $(ITERATION6_COVER_PROFILE) $(ITERATION6_SERVICE_COVER_MIN) $(ITERATION6_SLICE_COVER_MIN)

test-iteration7-cover-check:
	TZ=UTC go test ./internal/app/services ./internal/presentation/api/handlers ./internal/infrastructure/db/mappers ./internal/infrastructure/db/repositories \
		-run 'TestMediaMoments' -count=1 \
		-coverprofile=$(ITERATION7_COVER_PROFILE) \
		-coverpkg=./internal/app/services,./internal/presentation/api/handlers,./internal/infrastructure/db/mappers,./internal/infrastructure/db/repositories
	bash scripts/check-iteration7-coverage.sh $(ITERATION7_COVER_PROFILE) $(ITERATION7_SERVICE_COVER_MIN) $(ITERATION7_SLICE_COVER_MIN)

test-iteration8-cover-check:
	TZ=UTC go test ./internal/app/services ./internal/presentation/api/handlers ./internal/infrastructure/db/mappers ./internal/infrastructure/db/repositories ./internal/app/mappers \
		-run 'TestNotifications|TestNotificationService|TestNotificationMapper|TestMediaMoments_AwardReverseAndModerationRepositoryLifecycle|TestIteration6_FinalizeResultsAwardsExactlyOnceAndRollsBackInvalidSets' -count=1 \
		-coverprofile=$(ITERATION8_COVER_PROFILE) \
		-coverpkg=./internal/app/services,./internal/presentation/api/handlers,./internal/infrastructure/db/mappers,./internal/infrastructure/db/repositories,./internal/app/mappers
	bash scripts/check-iteration8-coverage.sh $(ITERATION8_COVER_PROFILE) $(ITERATION8_SERVICE_COVER_MIN) $(ITERATION8_SLICE_COVER_MIN)

validate: wire build vet test-race test-cover-check test-admin-cover-check test-iteration5-cover-check test-iteration6-cover-check test-iteration7-cover-check test-iteration8-cover-check test-migrations openapi

# ── Docker / Database ──────────────────────────────────────────────────────
db-up:
	docker compose up -d db

db-down:
	docker compose down

db-reset:
	docker compose down -v && docker compose up -d db

s3-up:
	docker compose up -d s3

local-up:
	docker compose up -d --wait db s3

local-down:
	docker compose down

docker-build:
	docker build -f build/docker/Dockerfile .
