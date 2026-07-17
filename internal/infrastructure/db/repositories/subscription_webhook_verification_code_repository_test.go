package repositories

import (
	"context"
	"errors"
	"testing"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	swEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/subscriptionwebhook/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/domain/subscriptionwebhookverificationcode/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedWebhook(t *testing.T, ctx context.Context) *swEntities.SubscriptionWebhook {
	t.Helper()
	webhook, err := TestSuite.SubscriptionWebhookRepository.Create(ctx, &swEntities.SubscriptionWebhook{Payload: "{}"})
	require.NoError(t, err)
	return webhook
}

func TestSubscriptionWebhookVerificationCodeRepository_CRUD(t *testing.T) {
	ctx := context.Background()
	TestSuite.TruncateTable(t, &models.SubscriptionWebhookVerificationCode{})
	TestSuite.TruncateTable(t, &models.SubscriptionWebhook{})

	repo := NewSubscriptionWebhookVerificationCodeRepository(TestSuite.DbConn)
	webhook := seedWebhook(t, ctx)

	created, err := repo.Create(ctx, &entities.SubscriptionWebhookVerificationCode{
		SubscriptionWebhookID: webhook.ID,
		Email:                 "sub@example.com",
		Name:                  "Sub",
		VerificationCode:      "123456",
	})
	require.NoError(t, err)
	assert.NotZero(t, created.ID)

	created.Name = "Sub Updated"
	updated, err := repo.Update(ctx, created)
	require.NoError(t, err)
	assert.Equal(t, "Sub Updated", updated.Name)
}

func TestSubscriptionWebhookVerificationCodeRepository_FindByEmail(t *testing.T) {
	ctx := context.Background()
	TestSuite.TruncateTable(t, &models.SubscriptionWebhookVerificationCode{})
	TestSuite.TruncateTable(t, &models.SubscriptionWebhook{})

	repo := NewSubscriptionWebhookVerificationCodeRepository(TestSuite.DbConn)
	webhook := seedWebhook(t, ctx)

	_, err := repo.Create(ctx, &entities.SubscriptionWebhookVerificationCode{
		SubscriptionWebhookID: webhook.ID,
		Email:                 "findme@example.com",
		VerificationCode:      "111111",
	})
	require.NoError(t, err)

	t.Run("found", func(t *testing.T) {
		found, err := repo.FindByEmail(ctx, "findme@example.com")
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, "111111", found.VerificationCode)
	})

	t.Run("not found returns nil, nil", func(t *testing.T) {
		found, err := repo.FindByEmail(ctx, "missing@example.com")
		require.NoError(t, err)
		assert.Nil(t, found)
	})
}

func TestSubscriptionWebhookVerificationCodeRepository_FindByDocument(t *testing.T) {
	ctx := context.Background()
	TestSuite.TruncateTable(t, &models.SubscriptionWebhookVerificationCode{})
	TestSuite.TruncateTable(t, &models.SubscriptionWebhook{})

	repo := NewSubscriptionWebhookVerificationCodeRepository(TestSuite.DbConn)
	webhook := seedWebhook(t, ctx)

	_, err := repo.Create(ctx, &entities.SubscriptionWebhookVerificationCode{
		SubscriptionWebhookID: webhook.ID,
		Email:                 "finddocument@example.com",
		Document:              "12345678900",
		VerificationCode:      "222222",
	})
	require.NoError(t, err)

	t.Run("found", func(t *testing.T) {
		found, err := repo.FindByDocument(ctx, "12345678900")
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, "222222", found.VerificationCode)
	})

	t.Run("not found returns nil, nil", func(t *testing.T) {
		found, err := repo.FindByDocument(ctx, "00000000000")
		require.NoError(t, err)
		assert.Nil(t, found)
	})
}

func TestSubscriptionWebhookVerificationCodeRepository_FindByEmailAndCode(t *testing.T) {
	ctx := context.Background()
	TestSuite.TruncateTable(t, &models.SubscriptionWebhookVerificationCode{})
	TestSuite.TruncateTable(t, &models.SubscriptionWebhook{})

	repo := NewSubscriptionWebhookVerificationCodeRepository(TestSuite.DbConn)
	webhook := seedWebhook(t, ctx)

	_, err := repo.Create(ctx, &entities.SubscriptionWebhookVerificationCode{
		SubscriptionWebhookID: webhook.ID,
		Email:                 "code@example.com",
		VerificationCode:      "654321",
	})
	require.NoError(t, err)

	t.Run("matches both fields", func(t *testing.T) {
		found, err := repo.FindByEmailAndCode(ctx, "code@example.com", "654321")
		require.NoError(t, err)
		require.NotNil(t, found)
	})

	t.Run("wrong code returns nil, nil", func(t *testing.T) {
		found, err := repo.FindByEmailAndCode(ctx, "code@example.com", "000000")
		require.NoError(t, err)
		assert.Nil(t, found)
	})
}

// ──── Error branches — go-sqlmock ────────────────────────────────────────────

func TestSubscriptionWebhookVerificationCodeRepository_Create_Error(t *testing.T) {
	gormDB, mock := newMockDB(t)
	repo := NewSubscriptionWebhookVerificationCodeRepository(gormDB)
	mock.ExpectBegin().WillReturnError(errors.New("conn failed"))

	_, err := repo.Create(context.Background(), &entities.SubscriptionWebhookVerificationCode{Email: "e@e.com"})
	assert.ErrorIs(t, err, appErrors.InternalError)
}

func TestSubscriptionWebhookVerificationCodeRepository_FindByEmail_Error(t *testing.T) {
	gormDB, mock := newMockDB(t)
	repo := NewSubscriptionWebhookVerificationCodeRepository(gormDB)
	mock.ExpectQuery(`SELECT`).WillReturnError(errors.New("db failure"))

	_, err := repo.FindByEmail(context.Background(), "e@e.com")
	assert.ErrorIs(t, err, appErrors.InternalError)
}

func TestSubscriptionWebhookVerificationCodeRepository_FindByDocument_Error(t *testing.T) {
	gormDB, mock := newMockDB(t)
	repo := NewSubscriptionWebhookVerificationCodeRepository(gormDB)
	mock.ExpectQuery(`SELECT`).WillReturnError(errors.New("db failure"))

	_, err := repo.FindByDocument(context.Background(), "12345678900")
	assert.ErrorIs(t, err, appErrors.InternalError)
}

func TestSubscriptionWebhookVerificationCodeRepository_Update_Error(t *testing.T) {
	gormDB, mock := newMockDB(t)
	repo := NewSubscriptionWebhookVerificationCodeRepository(gormDB)
	mock.ExpectBegin().WillReturnError(errors.New("conn failed"))

	_, err := repo.Update(context.Background(), &entities.SubscriptionWebhookVerificationCode{})
	assert.ErrorIs(t, err, appErrors.InternalError)
}

func TestSubscriptionWebhookVerificationCodeRepository_FindByEmailAndCode_Error(t *testing.T) {
	gormDB, mock := newMockDB(t)
	repo := NewSubscriptionWebhookVerificationCodeRepository(gormDB)
	mock.ExpectQuery(`SELECT`).WillReturnError(errors.New("db failure"))

	_, err := repo.FindByEmailAndCode(context.Background(), "e@e.com", "123456")
	assert.ErrorIs(t, err, appErrors.InternalError)
}
