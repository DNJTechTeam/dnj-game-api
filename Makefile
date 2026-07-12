WIRE_DIR=cmd/api/di

COVER_MIN ?= 90
COVER_PROFILE ?= coverage.out
COVER_GATE_PROFILE ?= coverage.gate.out
COVER_PKGS=./...
COVER_TEST_PKGS=./internal/app/services ./internal/app/messages ./internal/domain/common/interfaces ./internal/app/errors ./internal/infrastructure/common ./internal/infrastructure/db/repositories ./internal/infrastructure/db/mappers
COVER_IGNORE_REGEX=^github.com/dnjtechteam/dnj-game-api/cmd/|/internal/infrastructure/di/|/internal/infrastructure/api/runner.go:|/internal/presentation/api/|/internal/infrastructure/db/(db.go|transaction.go):|/internal/infrastructure/db/migrations/
COVER_GATE_REGEX=^github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/(mappers|repositories)/

.PHONY: wire build run run-api vet tidy migrate \
        test test-cover test-cover-check test-cover-html coverage \
        test-services test-repos test-migrations \
        db-up db-down db-reset docker-build

# ── Build ──────────────────────────────────────────────────────────────────
wire:
	go generate ./$(WIRE_DIR)/...

build:
	go build ./...

vet:
	go vet ./...

tidy:
	go mod tidy

# ── Run ────────────────────────────────────────────────────────────────────
run:
	go run cmd/api/main.go

run-api: run

migrate:
	go run cmd/migrate/main.go

# ── Test ───────────────────────────────────────────────────────────────────
test:
	go test ./... -count=1

test-cover test-cover-check coverage:
	go test $(COVER_TEST_PKGS) -count=1 -coverprofile=$(COVER_PROFILE) -covermode=atomic -coverpkg=$(COVER_PKGS)
	go tool cover -func=$(COVER_PROFILE)
	@awk -v ignore='$(COVER_IGNORE_REGEX)' -v gate='$(COVER_GATE_REGEX)' 'NR==1 || ($$1 !~ ignore && $$1 ~ gate) { print }' $(COVER_PROFILE) > $(COVER_GATE_PROFILE)
	@echo "Coverage gate scope: $(COVER_GATE_REGEX)"
	@go tool cover -func=$(COVER_GATE_PROFILE)
	@total=$$(go tool cover -func=$(COVER_GATE_PROFILE) | awk '/^total:/ { gsub("%","",$$3); print $$3 }'); \
	awk -v total="$$total" -v min="$(COVER_MIN)" 'BEGIN { if (total + 0 < min + 0) { printf("coverage %.1f%% is below required %.1f%%\n", total, min); exit 1 } printf("coverage %.1f%% meets required %.1f%%\n", total, min) }'

test-cover-html: test-cover
	go tool cover -html=$(COVER_PROFILE) -o coverage.html

test-services:
	go test ./internal/app/services/... -count=1 -v

test-repos:
	go test ./internal/infrastructure/db/repositories/... -count=1 -v

test-migrations:
	go test ./internal/app/services/... -count=1 -v -run TestMigrations -timeout 120s

# ── Docker / Database ──────────────────────────────────────────────────────
db-up:
	docker-compose up -d

db-down:
	docker-compose down

db-reset:
	docker-compose down -v && docker-compose up -d

docker-build:
	docker build -f build/docker/Dockerfile .
