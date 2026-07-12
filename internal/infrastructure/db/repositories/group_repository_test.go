package repositories

import (
	"context"
	"errors"
	"fmt"
	"testing"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/domain/group/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroupRepository_CRUD(t *testing.T) {
	ctx := context.Background()
	TestSuite.TruncateTable(t, &models.Group{})

	repo := NewGroupRepository(TestSuite.DbConn)

	created, err := repo.Create(ctx, &entities.Group{Name: "Grupo Jovens A"})
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.NotZero(t, created.ID)

	byID, err := repo.FindByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "Grupo Jovens A", byID.Name)

	assert.True(t, repo.ExistsByID(ctx, created.ID))
	assert.False(t, repo.ExistsByID(ctx, 9999999))
}

func TestGroupRepository_FindByNameExact(t *testing.T) {
	ctx := context.Background()
	TestSuite.TruncateTable(t, &models.Group{})

	repo := NewGroupRepository(TestSuite.DbConn)
	_, err := repo.Create(ctx, &entities.Group{Name: "Grupo Jovens A"})
	require.NoError(t, err)

	t.Run("case-insensitive exact match", func(t *testing.T) {
		found, err := repo.FindByNameExact(ctx, "grupo jovens a")
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, "Grupo Jovens A", found.Name)
	})

	t.Run("no match returns nil, nil", func(t *testing.T) {
		found, err := repo.FindByNameExact(ctx, "Grupo Inexistente")
		require.NoError(t, err)
		assert.Nil(t, found)
	})
}

func TestGroupRepository_Search(t *testing.T) {
	ctx := context.Background()
	TestSuite.TruncateTable(t, &models.Group{})

	repo := NewGroupRepository(TestSuite.DbConn)
	for i := range 25 {
		_, err := repo.Create(ctx, &entities.Group{Name: fmt.Sprintf("Grupo %02d", i)})
		require.NoError(t, err)
	}

	results, err := repo.Search(ctx, "Grupo", 20)
	require.NoError(t, err)
	assert.Len(t, results, 20)

	for i := 1; i < len(results); i++ {
		assert.LessOrEqual(t, results[i-1].Name, results[i].Name)
	}
}

func TestGroupRepository_Search_Partial(t *testing.T) {
	ctx := context.Background()
	TestSuite.TruncateTable(t, &models.Group{})

	repo := NewGroupRepository(TestSuite.DbConn)
	_, err := repo.Create(ctx, &entities.Group{Name: "Grupo Jovens A"})
	require.NoError(t, err)
	_, err = repo.Create(ctx, &entities.Group{Name: "Outro Grupo"})
	require.NoError(t, err)

	results, err := repo.Search(ctx, "jovens", 20)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "Grupo Jovens A", results[0].Name)
}

// ──── Error branches — go-sqlmock ────────────────────────────────────────────

func TestGroupRepository_Create_Error(t *testing.T) {
	gormDB, mock := newMockDB(t)
	repo := NewGroupRepository(gormDB)
	mock.ExpectBegin().WillReturnError(errors.New("conn failed"))

	_, err := repo.Create(context.Background(), &entities.Group{Name: "g"})
	assert.ErrorIs(t, err, appErrors.InternalError)
}

func TestGroupRepository_FindByID_Error(t *testing.T) {
	gormDB, mock := newMockDB(t)
	repo := NewGroupRepository(gormDB)
	mock.ExpectQuery(`SELECT`).WillReturnError(errors.New("db failure"))

	_, err := repo.FindByID(context.Background(), 1)
	assert.ErrorIs(t, err, appErrors.InternalError)
}

func TestGroupRepository_FindByNameExact_Error(t *testing.T) {
	gormDB, mock := newMockDB(t)
	repo := NewGroupRepository(gormDB)
	mock.ExpectQuery(`SELECT`).WillReturnError(errors.New("db failure"))

	_, err := repo.FindByNameExact(context.Background(), "x")
	assert.ErrorIs(t, err, appErrors.InternalError)
}

func TestGroupRepository_Search_Error(t *testing.T) {
	gormDB, mock := newMockDB(t)
	repo := NewGroupRepository(gormDB)
	mock.ExpectQuery(`SELECT`).WillReturnError(errors.New("db failure"))

	_, err := repo.Search(context.Background(), "x", 20)
	assert.ErrorIs(t, err, appErrors.InternalError)
}

func TestGroupRepository_ExistsByID_Error(t *testing.T) {
	gormDB, mock := newMockDB(t)
	repo := NewGroupRepository(gormDB)
	mock.ExpectQuery(`SELECT`).WillReturnError(errors.New("db failure"))

	assert.False(t, repo.ExistsByID(context.Background(), 1))
}
