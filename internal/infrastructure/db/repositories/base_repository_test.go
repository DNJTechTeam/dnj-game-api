package repositories

import (
	"context"
	"errors"
	"fmt"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	appErrors "github.com/scienceandcode/nucleus-api/internal/app/errors"
	dbPkg "github.com/scienceandcode/nucleus-api/internal/infrastructure/db"
	"github.com/scienceandcode/nucleus-api/internal/infrastructure/db/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// newMockDB returns a GORM DB backed by go-sqlmock so tests can control which
// queries succeed or fail without touching the real Postgres container.
func newMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })
	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	require.NoError(t, err)
	return gormDB, mock
}

// ──── handleRepositoryError ──────────────────────────────────────────────────

func TestHandleRepositoryError(t *testing.T) {
	t.Run("nil returns nil", func(t *testing.T) {
		assert.Nil(t, handleRepositoryError(nil))
	})

	t.Run("ErrRecordNotFound returns ErrNotFound", func(t *testing.T) {
		assert.ErrorIs(t, handleRepositoryError(gorm.ErrRecordNotFound), appErrors.ErrNotFound)
	})

	t.Run("arbitrary error returns InternalError", func(t *testing.T) {
		assert.ErrorIs(t, handleRepositoryError(errors.New("arbitrary")), appErrors.InternalError)
	})
}

// ──── getDB ──────────────────────────────────────────────────────────────────

func TestBaseRepository_GetDB(t *testing.T) {
	repo := &BaseRepository[models.Task]{DB: TestSuite.DbConn}

	t.Run("without tx returns repo.DB", func(t *testing.T) {
		got := repo.getDB(context.Background())
		assert.Equal(t, TestSuite.DbConn, got)
	})

	t.Run("with tx in context returns the transaction", func(t *testing.T) {
		tx := TestSuite.DbConn.Begin()
		require.NoError(t, tx.Error)
		t.Cleanup(func() { tx.Rollback() })

		ctx := dbPkg.WithTransaction(context.Background(), tx)
		assert.Equal(t, tx, repo.getDB(ctx))
	})
}

// ──── Happy paths — real Postgres ────────────────────────────────────────────

func TestBaseRepository_FindAll(t *testing.T) {
	ctx := context.Background()
	TestSuite.TruncateTable(t, &models.Task{})
	TestSuite.TruncateTable(t, &models.User{})

	user := seedUser(t, ctx, "findall@example.com")
	repo := NewBaseRepository[models.Task](TestSuite.DbConn)

	require.NoError(t, repo.Create(ctx, &models.Task{UserID: user.ID, Title: "t1", Status: "pending"}))
	require.NoError(t, repo.Create(ctx, &models.Task{UserID: user.ID, Title: "t2", Status: "pending"}))

	all, err := repo.FindAll(ctx)
	require.NoError(t, err)
	assert.Len(t, all, 2)
}

func TestBaseRepository_DeleteBy(t *testing.T) {
	ctx := context.Background()
	TestSuite.TruncateTable(t, &models.Task{})
	TestSuite.TruncateTable(t, &models.User{})

	user := seedUser(t, ctx, "deleteby@example.com")
	repo := NewBaseRepository[models.Task](TestSuite.DbConn)

	require.NoError(t, repo.Create(ctx, &models.Task{UserID: user.ID, Title: "del", Status: "pending"}))

	err := repo.DeleteBy(ctx, map[string]any{"user_id": user.ID})
	require.NoError(t, err)

	all, _ := repo.FindAll(ctx)
	assert.Empty(t, all)
}

func TestBaseRepository_Count(t *testing.T) {
	ctx := context.Background()
	TestSuite.TruncateTable(t, &models.Task{})
	TestSuite.TruncateTable(t, &models.User{})

	user := seedUser(t, ctx, "count@example.com")
	repo := NewBaseRepository[models.Task](TestSuite.DbConn)

	count, err := repo.Count(ctx)
	require.NoError(t, err)
	assert.Zero(t, count)

	require.NoError(t, repo.Create(ctx, &models.Task{UserID: user.ID, Title: "t", Status: "pending"}))

	count, err = repo.Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestBaseRepository_FindAllBy(t *testing.T) {
	ctx := context.Background()
	TestSuite.TruncateTable(t, &models.Task{})
	TestSuite.TruncateTable(t, &models.User{})

	user := seedUser(t, ctx, "findallby@example.com")
	repo := NewBaseRepository[models.Task](TestSuite.DbConn)

	require.NoError(t, repo.Create(ctx, &models.Task{UserID: user.ID, Title: "match", Status: "pending"}))
	require.NoError(t, repo.Create(ctx, &models.Task{UserID: user.ID, Title: "other", Status: "done"}))

	results, err := repo.FindAllBy(ctx, map[string]any{"status": "pending"})
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "match", results[0].Title)
}

func TestBaseRepository_ExistsBy(t *testing.T) {
	ctx := context.Background()
	TestSuite.TruncateTable(t, &models.Task{})
	TestSuite.TruncateTable(t, &models.User{})

	user := seedUser(t, ctx, "existsby@example.com")
	repo := NewBaseRepository[models.Task](TestSuite.DbConn)

	assert.False(t, repo.ExistsBy(ctx, map[string]any{"user_id": user.ID}))

	require.NoError(t, repo.Create(ctx, &models.Task{UserID: user.ID, Title: "exists", Status: "pending"}))

	assert.True(t, repo.ExistsBy(ctx, map[string]any{"user_id": user.ID}))
}

func TestBaseRepository_PaginateWithQuery(t *testing.T) {
	ctx := context.Background()
	TestSuite.TruncateTable(t, &models.Task{})
	TestSuite.TruncateTable(t, &models.User{})

	user := seedUser(t, ctx, "paginate@example.com")
	repo := NewBaseRepository[models.Task](TestSuite.DbConn)

	for i := range 11 {
		title := fmt.Sprintf("task-%02d", i)
		require.NoError(t, repo.Create(ctx, &models.Task{UserID: user.ID, Title: title, Status: "pending"}))
	}

	getDB := func(ctx context.Context) *gorm.DB { return TestSuite.DbConn }
	noFilter := func(db *gorm.DB) *gorm.DB { return db }

	t.Run("page 0 returns 10 items and hasNextPage true", func(t *testing.T) {
		result, err := repo.PaginateWithQuery(ctx, getDB, noFilter, 0)
		require.NoError(t, err)
		assert.Len(t, result.Data, 10)
		assert.True(t, result.Pagination.HasNextPage)
		assert.Equal(t, 10, result.Pagination.Limit)
	})

	t.Run("page 1 returns remaining item and hasNextPage false", func(t *testing.T) {
		result, err := repo.PaginateWithQuery(ctx, getDB, noFilter, 1)
		require.NoError(t, err)
		assert.Len(t, result.Data, 1)
		assert.False(t, result.Pagination.HasNextPage)
	})
}

// ──── Error branches — go-sqlmock ────────────────────────────────────────────

func TestBaseRepository_Create_Error(t *testing.T) {
	gormDB, mock := newMockDB(t)
	repo := NewBaseRepository[models.Task](gormDB)
	mock.ExpectBegin().WillReturnError(errors.New("conn failed"))

	err := repo.Create(context.Background(), &models.Task{Title: "t"})
	assert.ErrorIs(t, err, appErrors.InternalError)
}

func TestBaseRepository_FindByID_RecordNotFound(t *testing.T) {
	gormDB, mock := newMockDB(t)
	repo := NewBaseRepository[models.Task](gormDB)
	mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrRecordNotFound)

	_, err := repo.FindByID(context.Background(), uint64(1))
	assert.ErrorIs(t, err, appErrors.ErrNotFound)
}

func TestBaseRepository_FindByID_InternalError(t *testing.T) {
	gormDB, mock := newMockDB(t)
	repo := NewBaseRepository[models.Task](gormDB)
	mock.ExpectQuery(`SELECT`).WillReturnError(errors.New("db failure"))

	_, err := repo.FindByID(context.Background(), uint64(1))
	assert.ErrorIs(t, err, appErrors.InternalError)
}

func TestBaseRepository_FindAll_Error(t *testing.T) {
	gormDB, mock := newMockDB(t)
	repo := NewBaseRepository[models.Task](gormDB)
	mock.ExpectQuery(`SELECT`).WillReturnError(errors.New("db failure"))

	_, err := repo.FindAll(context.Background())
	assert.ErrorIs(t, err, appErrors.InternalError)
}

func TestBaseRepository_Update_Error(t *testing.T) {
	gormDB, mock := newMockDB(t)
	repo := NewBaseRepository[models.Task](gormDB)
	mock.ExpectBegin().WillReturnError(errors.New("conn failed"))

	err := repo.Update(context.Background(), &models.Task{})
	assert.ErrorIs(t, err, appErrors.InternalError)
}

func TestBaseRepository_Delete_Error(t *testing.T) {
	gormDB, mock := newMockDB(t)
	repo := NewBaseRepository[models.Task](gormDB)
	mock.ExpectBegin().WillReturnError(errors.New("conn failed"))

	err := repo.Delete(context.Background(), uint64(1))
	assert.ErrorIs(t, err, appErrors.InternalError)
}

func TestBaseRepository_DeleteBy_Error(t *testing.T) {
	gormDB, mock := newMockDB(t)
	repo := NewBaseRepository[models.Task](gormDB)
	mock.ExpectBegin().WillReturnError(errors.New("conn failed"))

	err := repo.DeleteBy(context.Background(), map[string]any{"status": "pending"})
	assert.ErrorIs(t, err, appErrors.InternalError)
}

func TestBaseRepository_Count_Error(t *testing.T) {
	gormDB, mock := newMockDB(t)
	repo := NewBaseRepository[models.Task](gormDB)
	mock.ExpectQuery(`SELECT`).WillReturnError(errors.New("db failure"))

	_, err := repo.Count(context.Background())
	assert.ErrorIs(t, err, appErrors.InternalError)
}

func TestBaseRepository_FindOneBy_RecordNotFound(t *testing.T) {
	gormDB, mock := newMockDB(t)
	repo := NewBaseRepository[models.Task](gormDB)
	mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrRecordNotFound)

	_, err := repo.FindOneBy(context.Background(), map[string]any{"id": 1})
	assert.ErrorIs(t, err, appErrors.ErrNotFound)
}

func TestBaseRepository_FindOneBy_InternalError(t *testing.T) {
	gormDB, mock := newMockDB(t)
	repo := NewBaseRepository[models.Task](gormDB)
	mock.ExpectQuery(`SELECT`).WillReturnError(errors.New("db failure"))

	_, err := repo.FindOneBy(context.Background(), map[string]any{"id": 1})
	assert.ErrorIs(t, err, appErrors.InternalError)
}

func TestBaseRepository_FindAllBy_Error(t *testing.T) {
	gormDB, mock := newMockDB(t)
	repo := NewBaseRepository[models.Task](gormDB)
	mock.ExpectQuery(`SELECT`).WillReturnError(errors.New("db failure"))

	_, err := repo.FindAllBy(context.Background(), map[string]any{"status": "pending"})
	assert.ErrorIs(t, err, appErrors.InternalError)
}

func TestBaseRepository_ExistsBy_Error(t *testing.T) {
	gormDB, mock := newMockDB(t)
	repo := NewBaseRepository[models.Task](gormDB)
	mock.ExpectQuery(`SELECT`).WillReturnError(errors.New("db failure"))

	result := repo.ExistsBy(context.Background(), map[string]any{"id": 1})
	assert.False(t, result)
}

func TestBaseRepository_PaginateWithQuery_Error(t *testing.T) {
	gormDB, mock := newMockDB(t)
	repo := NewBaseRepository[models.Task](gormDB)
	mock.ExpectQuery(`SELECT`).WillReturnError(errors.New("db failure"))

	getDB := func(ctx context.Context) *gorm.DB { return gormDB }
	_, err := repo.PaginateWithQuery(context.Background(), getDB, func(db *gorm.DB) *gorm.DB { return db }, 0)
	assert.ErrorIs(t, err, appErrors.InternalError)
}

func TestBaseRepository_PaginateWithQuery_ErrRecordNotFound(t *testing.T) {
	gormDB, mock := newMockDB(t)
	repo := NewBaseRepository[models.Task](gormDB)
	mock.ExpectQuery(`SELECT`).WillReturnError(gorm.ErrRecordNotFound)

	getDB := func(ctx context.Context) *gorm.DB { return gormDB }
	result, err := repo.PaginateWithQuery(context.Background(), getDB, func(db *gorm.DB) *gorm.DB { return db }, 0)
	require.NoError(t, err)
	assert.Empty(t, result.Data)
	assert.False(t, result.Pagination.HasNextPage)
}
