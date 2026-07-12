package repositories

import (
	"context"
	"errors"
	"testing"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/domain/subscriptionwebhook/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubscriptionWebhookRepository_Create(t *testing.T) {
	ctx := context.Background()
	TestSuite.TruncateTable(t, &models.SubscriptionWebhook{})

	repo := NewSubscriptionWebhookRepository(TestSuite.DbConn)

	created, err := repo.Create(ctx, &entities.SubscriptionWebhook{Payload: `{"email":"a@a.com"}`})
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.NotZero(t, created.ID)
	assert.Equal(t, `{"email":"a@a.com"}`, created.Payload)
}

func TestSubscriptionWebhookRepository_Create_Error(t *testing.T) {
	gormDB, mock := newMockDB(t)
	repo := NewSubscriptionWebhookRepository(gormDB)
	mock.ExpectBegin().WillReturnError(errors.New("conn failed"))

	_, err := repo.Create(context.Background(), &entities.SubscriptionWebhook{Payload: "{}"})
	assert.ErrorIs(t, err, appErrors.InternalError)
}
