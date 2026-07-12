# Adding a New Resource — Step by Step

This is the single source of truth for adding a CRUD resource to Nucleus API.
It applies equally whether you are Claude, Codex, or a human.

The example resource **`Task`** already exists across every layer — keep it
open as a reference while you follow these steps. Below we add a fictional
resource called **`Widget`** owned by the authenticated user.

> Naming: package names are singular lowercase. Types are `Widget`,
> `WidgetService`, `WidgetHandler`. DTOs end in `RequestDTO` / `ResponseDTO`.

---

## 0. Define the resource

Decide the fields, the status/enum values (if any), and whether rows are
**user-owned** (scoped to the JWT user, like `Task`) or global. This guide
assumes user-owned — that is the common case and exercises the identity
context. For our example: `Widget { ID, UserID, Name, Color, CreatedAt, UpdatedAt }`.

## 1. Domain — entity

`internal/domain/widget/entities/widget_entity.go`

```go
package entities

import "time"

type Widget struct {
	ID        uint64
	UserID    uint64
	Name      string
	Color     string
	CreatedAt time.Time
	UpdatedAt time.Time
}
```

## 2. Domain — repository interface

`internal/domain/widget/interfaces/widget_repository_interface.go`

```go
package interfaces

import (
	"context"

	"github.com/dnjtechteam/dnj-game-api/internal/domain/widget/entities"
)

type WidgetRepositoryInterface interface {
	Create(ctx context.Context, widget *entities.Widget) (*entities.Widget, error)
	FindByIDAndUser(ctx context.Context, id uint64, userID uint64) (*entities.Widget, error)
	FindAllByUser(ctx context.Context, userID uint64) ([]*entities.Widget, error)
	Update(ctx context.Context, widget *entities.Widget) (*entities.Widget, error)
	Delete(ctx context.Context, id uint64, userID uint64) error
}
```

## 3. Infrastructure — GORM model

`internal/infrastructure/db/models/widget.go`

```go
package models

import "time"

type Widget struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement"`
	UserID    uint64    `gorm:"not null;index:idx_widgets_user_id"`
	Name      string    `gorm:"not null"`
	Color     string    `gorm:"default:null"`
	CreatedAt time.Time `gorm:"autoCreateTime:nano"`
	UpdatedAt time.Time `gorm:"autoUpdateTime:nano"`
}

func (Widget) TableName() string { return "widgets" }
```

Add a `gorm` index for every column you will filter or join on later.

## 4. Infrastructure — Model ↔ Entity mapper

`internal/infrastructure/db/mappers/widget_mapper.go` — two pure functions
`MapWidgetToEntity` and `MapWidgetEntityToModel`. Copy `task_mapper.go`.

## 5. Infrastructure — repository

`internal/infrastructure/db/repositories/widget_repository.go` — embed
`*BaseRepository[models.Widget]`, implement the interface, map Entity ↔ Model,
use `r.getDB(ctx)` for owner-scoped queries. Copy `task_repository.go`.

Note `Delete` uses `RowsAffected == 0` to return `appErrors.ErrNotFound`.

## 6. Migration (idempotent)

In `internal/infrastructure/db/migrations/model_migrations.go`, append inside
`RegisterModelMigrations`:

```go
registry.Register(createModelMigration("create_widgets_table", "1.1.0", &models.Widget{}))
```

For later column additions, write a guarded `ALTER TABLE ... IF NOT EXISTS`
migration — see `docs/migrations.md`.

## 7. Application — request/response DTOs

`internal/app/messages/widget_dto.go` — `CreateWidgetRequestDTO`,
`UpdateWidgetRequestDTO`, `WidgetResponseDTO`. JSON tags are camelCase; expose
IDs as `Uint64String`. Copy `task_dto.go`.

## 8. Application — Entity → DTO mapper

`internal/app/mappers/widget_mapper.go` — `MapWidgetToResponseDTO` and a slice
helper. Copy `task_mapper.go`.

## 9. Application — service interface

`internal/app/interfaces/widget_service_interface.go` — `List`, `Get`,
`Create`, `Update`, `Delete` (add `UpdateStatus`-style methods as needed).

## 10. Application — service

`internal/app/services/widget_service.go` — embed `*BaseService`, depend on
`WidgetRepositoryInterface`. Get the caller with
`common.ExtractUserIdFromContext(ctx)` and scope every call to it. Validate
input and return typed errors (`appErrors.NewError(...)`). Use
`s.WithTransaction(ctx, ...)` when an operation spans multiple writes. Copy
`task_service.go`.

## 11. Presentation — handler

`internal/presentation/api/handlers/widget_handler.go` — one struct with the
service as a field, one method per route. Parse the body with `ParseRequest`,
parse `:id` with `strconv.ParseUint`, respond with the helpers in
`functions.go`. Map `appErrors.ErrNotFound` via `errors.Is`. Copy
`task_handler.go`.

Then register the handler in `internal/presentation/api/handlers/handlers.go`:

```go
type Handlers struct {
	HealthcheckHandler *HealthcheckHandler
	UserAuthHandler    *UserAuthHandler
	TaskHandler        *TaskHandler
	WidgetHandler      *WidgetHandler   // <-- add
}
```

## 12. Presentation — router

`internal/presentation/api/routers/widget_router.go`:

```go
package routers

func (r *Router) RegisterWidgetRoutes() {
	group := r.group.Group("/widgets")

	group.GET("", append(r.authProtected(), r.handlers.WidgetHandler.List)...)
	group.GET("/:id", append(r.authProtected(), r.handlers.WidgetHandler.Get)...)
	group.POST("", append(r.authProtected(), r.handlers.WidgetHandler.Create)...)
	group.PUT("/:id", append(r.authProtected(), r.handlers.WidgetHandler.Update)...)
	group.DELETE("/:id", append(r.authProtected(), r.handlers.WidgetHandler.Delete)...)
}
```

Then call it from `RegisterRoutes()` in `router.go`:

```go
func (r *Router) RegisterRoutes() *gin.Engine {
	r.RegisterHealthcheckRoutes()
	r.RegisterAuthRoutes()
	r.RegisterTaskRoutes()
	r.RegisterWidgetRoutes()   // <-- add
	return r.engine
}
```

Use `r.authProtected()` for routes that require a logged-in user; omit it for
public routes.

## 13. Wire — providers

Add a repository provider in
`internal/infrastructure/di/db/repositories/provider.go`:

```go
func ProvideWidgetRepository(db *gorm.DB) widgetInterfaces.WidgetRepositoryInterface {
	return repositories.NewWidgetRepository(db)
}
```

Add a service provider in `internal/infrastructure/di/services/provider.go`:

```go
func ProvideWidgetService(
	baseService *services.BaseService,
	widgetRepository widgetInterfaces.WidgetRepositoryInterface,
) appInterfaces.WidgetServiceInterface {
	return services.NewWidgetService(baseService, widgetRepository)
}
```

Register both — plus the handler struct — in `cmd/api/di/wire.go`:

```go
repositories.ProvideWidgetRepository,
services.ProvideWidgetService,
wire.Struct(new(handlers.WidgetHandler), "*"),
```

Regenerate:

```bash
go generate ./cmd/api/di    # or: make run-wire
```

## 14. Mocks

Add the new interface package(s) to `.mockery.yml` if you want generated mocks
for them, then run:

```bash
mockery
```

## 15. Tests

Add `internal/app/services/widget_service_test.go`. See `docs/TESTING.md` for
the full conventions. Required steps:

**a) Wire the repository into TestSuiteType** (`main_test.go`):

```go
type TestSuiteType struct {
    // existing fields ...
    WidgetRepository widgetInterfaces.WidgetRepositoryInterface  // <-- add
}

func initializeTestSuite() {
    // ...
    TestSuite.WidgetRepository = repositories.NewWidgetRepository(TestSuite.DbConn)
}
```

**b) Add a setup function**:

```go
func setupWidgetServiceTest(t *testing.T) interfaces.WidgetServiceInterface {
    t.Helper()
    TestSuite.DefaultSetup(t)
    TestSuite.TruncateTable(t, &models.Widget{})
    TestSuite.TruncateTable(t, &models.User{})
    t.Cleanup(func() {
        TestSuite.TruncateTable(t, &models.Widget{})
        TestSuite.TruncateTable(t, &models.User{})
    })
    return NewWidgetService(TestSuite.BaseService, TestSuite.WidgetRepository)
}
```

**c) Write the tests** — minimum cases required:

```go
func TestWidgetService_Create(t *testing.T) {
    t.Run("creates widget for authenticated user", func(t *testing.T) {
        // given
        svc := setupWidgetServiceTest(t)
        _, ctx := seedTaskUser(t, "owner@example.com")  // reuse or create seed helper

        // when
        result, err := svc.Create(ctx, &messages.CreateWidgetRequestDTO{Name: "My Widget"})

        // then
        require.NoError(t, err)
        assert.Equal(t, "My Widget", result.Name)
        assert.NotZero(t, result.ID.Uint64())
    })

    t.Run("returns validation error for blank name", func(t *testing.T) {
        // given
        svc := setupWidgetServiceTest(t)
        _, ctx := seedTaskUser(t, "user@example.com")

        // when
        _, err := svc.Create(ctx, &messages.CreateWidgetRequestDTO{Name: ""})

        // then
        require.Error(t, err)
    })
}

func TestWidgetService_OwnerScoped(t *testing.T) {
    t.Run("returns not found when user reads another user's widget", func(t *testing.T) {
        // given
        svc := setupWidgetServiceTest(t)
        _, ctxA := seedTaskUser(t, "a@example.com")
        _, ctxB := seedTaskUser(t, "b@example.com")
        created, _ := svc.Create(ctxA, &messages.CreateWidgetRequestDTO{Name: "Private"})

        // when
        _, err := svc.Get(ctxB, created.ID.Uint64())

        // then
        assert.ErrorIs(t, err, appErrors.ErrNotFound)
    })
}
```

Copy `task_service_test.go` as a complete reference.

## 16. Verify

```bash
go build ./...
go generate ./cmd/api/di
make vet
make test-cover-check
```

All four must be clean before you are done.

---

## 17. Pagination

Apply this when the list endpoint needs cursor-based paging instead of returning all rows at once. It replaces the flat `List` with a paginated version across **all four layers**. The infrastructure uses `BaseRepository.PaginateWithQuery` — fixed page size of 10, 0-indexed pages, `hasNextPage` flag.

### Wire: query param on the wire

The client sends `GET /widgets?page=1` (1-indexed). The handler translates to 0-indexed internally.

```
page=1 → internal page 0 (first page)
page=2 → internal page 1 (second page)
page=0 or absent → internal page 0 (first page)
```

### Step 1 — Domain: update repository interface

Add `List` to `internal/domain/widget/interfaces/widget_repository_interface.go`:

```go
import "github.com/dnjtechteam/dnj-game-api/internal/app/messages"

type WidgetRepositoryInterface interface {
    Create(ctx context.Context, widget *entities.Widget) (*entities.Widget, error)
    FindByIDAndUser(ctx context.Context, id uint64, userID uint64) (*entities.Widget, error)
    Update(ctx context.Context, widget *entities.Widget) (*entities.Widget, error)
    Delete(ctx context.Context, id uint64, userID uint64) error
    List(ctx context.Context, userID uint64, page uint64) (*messages.PaginatedResponse[entities.Widget], error)
}
```

### Step 2 — Infrastructure: implement in repository

In `internal/infrastructure/db/repositories/widget_repository.go`:

```go
func (r *WidgetRepository) List(ctx context.Context, userID uint64, page uint64) (*messages.PaginatedResponse[entities.Widget], error) {
    result, err := r.PaginateWithQuery(
        ctx,
        r.getDB,
        func(db *gorm.DB) *gorm.DB {
            return db.Where("user_id = ?", userID).Order("created_at DESC")
        },
        page,
    )
    if err != nil {
        return nil, err
    }

    items := make([]entities.Widget, len(result.Data))
    for i := range result.Data {
        items[i] = *mappers.MapWidgetToEntity(&result.Data[i])
    }
    return &messages.PaginatedResponse[entities.Widget]{
        Data:       items,
        Pagination: result.Pagination,
    }, nil
}
```

### Step 3 — App: filter DTO

In `internal/app/messages/widget_dto.go`, add the filter type (embed `PaginationFilter` — it carries `Page uint64` with `GetPage()`/`SetPage()`):

```go
// ListWidgetsFilterDTO is populated from query params by ParsePaginationFromQuery.
// Embed additional filter fields here as the resource evolves.
type ListWidgetsFilterDTO struct {
    messages.PaginationFilter
}
```

### Step 4 — App: service interface

In `internal/app/interfaces/widget_service_interface.go`, replace the flat `List` signature:

```go
List(ctx context.Context, filter *messages.ListWidgetsFilterDTO) (*messages.PaginatedResponse[messages.WidgetResponseDTO], error)
```

### Step 5 — App: service implementation

In `internal/app/services/widget_service.go`:

```go
func (s *WidgetService) List(ctx context.Context, filter *messages.ListWidgetsFilterDTO) (*messages.PaginatedResponse[messages.WidgetResponseDTO], error) {
    userID, err := common.ExtractUserIdFromContext(ctx)
    if err != nil {
        return nil, appErrors.InternalError
    }

    result, err := s.widgetRepo.List(ctx, userID, filter.GetPage())
    if err != nil {
        return nil, err
    }

    dtos := make([]messages.WidgetResponseDTO, len(result.Data))
    for i := range result.Data {
        dtos[i] = *appMappers.MapWidgetToResponseDTO(&result.Data[i])
    }
    return &messages.PaginatedResponse[messages.WidgetResponseDTO]{
        Data:       dtos,
        Pagination: result.Pagination,
    }, nil
}
```

### Step 6 — Presentation: handler

In `internal/presentation/api/handlers/widget_handler.go`, replace the `List` method:

```go
func (h *WidgetHandler) List(c *gin.Context) {
    filter := &messages.ListWidgetsFilterDTO{}
    ParsePaginationFromQuery(c, filter) // reads ?page=N, converts to 0-indexed

    result, err := h.widgetService.List(c.Request.Context(), filter)
    if err != nil {
        ResponseInternalServerError(c, err)
        return
    }
    ResponseSuccess(c, http.StatusOK, result)
}
```

The route registration in the router stays the same (`GET ""`).

### Response shape

```json
{
  "data": [
    { "id": "1", "name": "Widget A", ... }
  ],
  "pagination": {
    "currentPage": "1",
    "hasNextPage": true,
    "limit": 10
  }
}
```

### Test cases to add

```go
func TestWidgetService_List_Paginated(t *testing.T) {
    t.Run("returns first page with hasNextPage true when more than 10 items exist", func(t *testing.T) {
        // given
        svc := setupWidgetServiceTest(t)
        _, ctx := seedWidgetUser(t, "owner@example.com")
        for i := range 11 {
            _, err := svc.Create(ctx, &messages.CreateWidgetRequestDTO{Name: fmt.Sprintf("w%d", i)})
            require.NoError(t, err)
        }

        // when
        result, err := svc.List(ctx, &messages.ListWidgetsFilterDTO{})

        // then
        require.NoError(t, err)
        assert.Len(t, result.Data, 10)
        assert.True(t, result.Pagination.HasNextPage)
        assert.Equal(t, "1", result.Pagination.CurrentPage.String())
    })

    t.Run("returns second page with remaining items and hasNextPage false", func(t *testing.T) {
        // given
        svc := setupWidgetServiceTest(t)
        _, ctx := seedWidgetUser(t, "owner2@example.com")
        for i := range 11 {
            _, _ = svc.Create(ctx, &messages.CreateWidgetRequestDTO{Name: fmt.Sprintf("w%d", i)})
        }
        filter := &messages.ListWidgetsFilterDTO{}
        filter.SetPage(1)

        // when
        result, err := svc.List(ctx, filter)

        // then
        require.NoError(t, err)
        assert.Len(t, result.Data, 1)
        assert.False(t, result.Pagination.HasNextPage)
    })
}
```

---

## Checklist

- [ ] `internal/domain/widget/entities/widget_entity.go`
- [ ] `internal/domain/widget/interfaces/widget_repository_interface.go`
- [ ] `internal/infrastructure/db/models/widget.go`
- [ ] `internal/infrastructure/db/mappers/widget_mapper.go`
- [ ] `internal/infrastructure/db/repositories/widget_repository.go`
- [ ] Migration registered in `model_migrations.go` (idempotent)
- [ ] `internal/app/messages/widget_dto.go`
- [ ] `internal/app/mappers/widget_mapper.go`
- [ ] `internal/app/interfaces/widget_service_interface.go`
- [ ] `internal/app/services/widget_service.go`
- [ ] `internal/presentation/api/handlers/widget_handler.go` + `handlers.go`
- [ ] `internal/presentation/api/routers/widget_router.go` + `router.go`
- [ ] Wire providers + `wire.Struct`, then `go generate ./cmd/api/di`
- [ ] `.mockery.yml` updated + `mockery` run (if new interfaces)
- [ ] `widget_service_test.go` + `TestSuite` wiring (following `docs/TESTING.md`)
- [ ] If list is paginated: step 17 applied across all layers + pagination test cases added
- [ ] `go build ./...`, `make vet`, and `make test-cover-check` green
