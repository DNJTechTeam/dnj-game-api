package repositories

import (
	"context"
	"errors"
	"testing"

	appErrors "github.com/scienceandcode/nucleus-api/internal/app/errors"
	"github.com/scienceandcode/nucleus-api/internal/domain/user/entities"
	"github.com/scienceandcode/nucleus-api/internal/infrastructure/db/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserRepository_CreateAndFind(t *testing.T) {
	ctx := context.Background()
	TestSuite.TruncateTable(t, &models.User{})

	created, err := TestSuite.UserRepository.Create(ctx, &entities.User{
		Email:       "repo@example.com",
		Name:        "Repo User",
		Password:    "Password@#1493",
		MobilePhone: "41992395568",
	})
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.NotZero(t, created.ID)

	byID, err := TestSuite.UserRepository.FindByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "repo@example.com", byID.Email)

	byEmail, err := TestSuite.UserRepository.FindByEmail(ctx, "repo@example.com")
	require.NoError(t, err)
	require.NotNil(t, byEmail)
	assert.Equal(t, created.ID, byEmail.ID)

	assert.True(t, TestSuite.UserRepository.ExistsByID(ctx, created.ID))
	assert.False(t, TestSuite.UserRepository.ExistsByID(ctx, 9999999))
}

func TestUserRepository_FindByEmailNotFound(t *testing.T) {
	ctx := context.Background()
	TestSuite.TruncateTable(t, &models.User{})

	user, err := TestSuite.UserRepository.FindByEmail(ctx, "missing@example.com")
	require.NoError(t, err)
	assert.Nil(t, user)
}

// ──── Error branches — go-sqlmock ────────────────────────────────────────────

func TestUserRepository_Create_Error(t *testing.T) {
	gormDB, mock := newMockDB(t)
	repo := NewUserRepository(gormDB)
	mock.ExpectBegin().WillReturnError(errors.New("conn failed"))

	_, err := repo.Create(context.Background(), &entities.User{Email: "e@e.com", Name: "N", Password: "p"})
	assert.ErrorIs(t, err, appErrors.InternalError)
}

func TestUserRepository_FindByID_Error(t *testing.T) {
	gormDB, mock := newMockDB(t)
	repo := NewUserRepository(gormDB)
	mock.ExpectQuery(`SELECT`).WillReturnError(errors.New("db failure"))

	_, err := repo.FindByID(context.Background(), 1)
	assert.ErrorIs(t, err, appErrors.InternalError)
}

func TestUserRepository_FindByEmail_InternalError(t *testing.T) {
	gormDB, mock := newMockDB(t)
	repo := NewUserRepository(gormDB)
	mock.ExpectQuery(`SELECT`).WillReturnError(errors.New("db failure"))

	_, err := repo.FindByEmail(context.Background(), "x@x.com")
	assert.ErrorIs(t, err, appErrors.InternalError)
}

func TestUserRepository_Update_Error(t *testing.T) {
	gormDB, mock := newMockDB(t)
	repo := NewUserRepository(gormDB)
	mock.ExpectBegin().WillReturnError(errors.New("conn failed"))

	_, err := repo.Update(context.Background(), &entities.User{})
	assert.ErrorIs(t, err, appErrors.InternalError)
}
