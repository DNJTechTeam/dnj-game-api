package services

import (
	"context"
	"sync"
	"testing"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	userEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupGroupInviteServiceTest(t *testing.T) *GroupInviteService {
	t.Helper()
	TestSuite.DefaultSetup(t)
	resetIteration3GroupTables(t)
	service := NewGroupInviteService(TestSuite.BaseService, TestSuite.UserRepository, TestSuite.GroupRepository, TestSuite.GroupMembershipRepository, TestSuite.GroupInviteRepository).(*GroupInviteService)
	service.now = func() time.Time { return time.Date(2026, 8, 22, 15, 0, 0, 0, time.UTC) }
	return service
}

func seedInviteUser(t *testing.T, email string, role userEntities.UserRole) (*userEntities.User, context.Context) {
	t.Helper()
	user, err := TestSuite.UserRepository.Create(TestSuite.Ctx, &userEntities.User{Email: email, Name: email, MobilePhone: "5541999990000", DocumentHash: "hash-" + email, DocumentLast4: "4725", Role: role})
	require.NoError(t, err)
	return user, TestSuite.ContextWithUser(user.ID)
}

func apiErrorCode(t *testing.T, err error) string {
	t.Helper()
	var apiErr *appErrors.APIServiceError
	require.ErrorAs(t, err, &apiErr)
	return apiErr.Code
}

func TestGroupInviteService_AdminLifecycleAndHashStorage(t *testing.T) {
	// given
	service := setupGroupInviteServiceTest(t)
	admin, adminCtx := seedInviteUser(t, "admin@example.com", userEntities.RoleAdmin)
	_, participantCtx := seedInviteUser(t, "participant@example.com", userEntities.RoleDefault)
	group := seedGroup(t, "Grupo Convites")

	// when
	created, createErr := service.Create(adminCtx, group.ID)
	var stored models.GroupInvite
	storedErr := TestSuite.DbConn.First(&stored, created.ID.Uint64()).Error
	_, forbiddenErr := service.Create(participantCtx, group.ID)
	listed, listErr := service.List(adminCtx, group.ID, &messages.ListGroupInvitesFilterDTO{})
	renewed, renewErr := service.Renew(adminCtx, group.ID, created.ID.Uint64())
	var old models.GroupInvite
	oldErr := TestSuite.DbConn.First(&old, created.ID.Uint64()).Error
	revokeErr := service.Revoke(adminCtx, group.ID, renewed.ID.Uint64())
	secondRevokeErr := service.Revoke(adminCtx, group.ID, renewed.ID.Uint64())

	// then
	require.NoError(t, createErr)
	require.NoError(t, storedErr)
	require.NoError(t, listErr)
	require.NoError(t, renewErr)
	require.NoError(t, oldErr)
	require.NoError(t, revokeErr)
	require.NoError(t, secondRevokeErr)
	assert.Equal(t, admin.ID, created.CreatedByUserID.Uint64())
	assert.NotEmpty(t, created.Code)
	assert.NotEqual(t, created.Code, stored.CodeHash)
	assert.Equal(t, tokenHash(created.Code), stored.CodeHash)
	assert.Len(t, stored.CodeHash, 64)
	assert.Empty(t, listed.Data[0].Code)
	assert.NotEqual(t, created.Code, renewed.Code)
	assert.NotNil(t, old.RevokedAt)
	assert.Equal(t, created.ID.Uint64(), renewed.ReplacesInviteID.Uint64())
	assert.Equal(t, "FORBIDDEN", apiErrorCode(t, forbiddenErr))
}

func TestGroupInviteService_ConsumeIsSingleUseAndIdempotentForConsumer(t *testing.T) {
	// given
	service := setupGroupInviteServiceTest(t)
	_, adminCtx := seedInviteUser(t, "admin-consume@example.com", userEntities.RoleAdmin)
	consumer, consumerCtx := seedInviteUser(t, "consumer@example.com", userEntities.RoleDefault)
	_, otherCtx := seedInviteUser(t, "other@example.com", userEntities.RoleDefault)
	group := seedGroup(t, "Grupo Consumo")
	created, err := service.Create(adminCtx, group.ID)
	require.NoError(t, err)

	// when
	first, firstErr := service.Consume(consumerCtx, &messages.ConsumeGroupInviteRequestDTO{Code: created.Code})
	retry, retryErr := service.Consume(consumerCtx, &messages.ConsumeGroupInviteRequestDTO{Code: created.Code})
	_, otherErr := service.Consume(otherCtx, &messages.ConsumeGroupInviteRequestDTO{Code: created.Code})
	storedUser, userErr := TestSuite.UserRepository.FindByID(TestSuite.Ctx, consumer.ID)
	membership, membershipErr := TestSuite.GroupMembershipRepository.FindByUser(TestSuite.Ctx, consumer.ID)
	var storedInvite models.GroupInvite
	inviteErr := TestSuite.DbConn.First(&storedInvite, created.ID.Uint64()).Error

	// then
	require.NoError(t, firstErr)
	require.NoError(t, retryErr)
	require.NoError(t, userErr)
	require.NoError(t, membershipErr)
	require.NoError(t, inviteErr)
	assert.Equal(t, first, retry)
	assert.Equal(t, group.ID, first.Group.ID.Uint64())
	assert.Equal(t, group.ID, *storedUser.GroupID)
	assert.Equal(t, group.ID, membership.GroupID)
	assert.True(t, storedUser.OnboardingComplete)
	assert.Equal(t, consumer.ID, *storedInvite.ConsumedByUserID)
	assert.Equal(t, "INVITE_NOT_FOUND_OR_UNAVAILABLE", apiErrorCode(t, otherErr))
}

func TestGroupInviteService_UnavailableStatesAreIndistinguishable(t *testing.T) {
	// given
	service := setupGroupInviteServiceTest(t)
	_, adminCtx := seedInviteUser(t, "admin-states@example.com", userEntities.RoleAdmin)
	_, userCtx := seedInviteUser(t, "states@example.com", userEntities.RoleDefault)
	group := seedGroup(t, "Grupo Estados")
	expired, err := service.Create(adminCtx, group.ID)
	require.NoError(t, err)
	require.NoError(t, TestSuite.DbConn.Model(&models.GroupInvite{}).Where("id = ?", expired.ID.Uint64()).Update("expires_at", service.now().Add(-time.Minute)).Error)
	revoked, err := service.Create(adminCtx, group.ID)
	require.NoError(t, err)
	require.NoError(t, service.Revoke(adminCtx, group.ID, revoked.ID.Uint64()))

	// when
	_, unknownErr := service.Consume(userCtx, &messages.ConsumeGroupInviteRequestDTO{Code: "unknown-high-entropy-code"})
	_, expiredErr := service.Consume(userCtx, &messages.ConsumeGroupInviteRequestDTO{Code: expired.Code})
	_, revokedErr := service.Consume(userCtx, &messages.ConsumeGroupInviteRequestDTO{Code: revoked.Code})

	// then
	for _, err := range []error{unknownErr, expiredErr, revokedErr} {
		assert.Equal(t, "INVITE_NOT_FOUND_OR_UNAVAILABLE", apiErrorCode(t, err))
	}
}

func TestGroupInviteService_ConcurrentConsumptionHasOneWinner(t *testing.T) {
	// given
	service := setupGroupInviteServiceTest(t)
	_, adminCtx := seedInviteUser(t, "admin-race@example.com", userEntities.RoleAdmin)
	first, firstCtx := seedInviteUser(t, "race-first@example.com", userEntities.RoleDefault)
	second, secondCtx := seedInviteUser(t, "race-second@example.com", userEntities.RoleDefault)
	group := seedGroup(t, "Grupo Corrida")
	created, err := service.Create(adminCtx, group.ID)
	require.NoError(t, err)
	contexts := []context.Context{firstCtx, secondCtx}
	errs := make(chan error, 2)
	start := make(chan struct{})
	var wait sync.WaitGroup
	wait.Add(2)

	// when
	for _, ctx := range contexts {
		go func(ctx context.Context) {
			defer wait.Done()
			<-start
			_, consumeErr := service.Consume(ctx, &messages.ConsumeGroupInviteRequestDTO{Code: created.Code})
			errs <- consumeErr
		}(ctx)
	}
	close(start)
	wait.Wait()
	close(errs)
	var success, unavailable int
	for consumeErr := range errs {
		if consumeErr == nil {
			success++
		} else if apiErrorCode(t, consumeErr) == "INVITE_NOT_FOUND_OR_UNAVAILABLE" {
			unavailable++
		}
	}
	var memberships int64
	countErr := TestSuite.DbConn.Model(&models.GroupMembership{}).Where("user_id IN ?", []uint64{first.ID, second.ID}).Count(&memberships).Error
	var stored models.GroupInvite
	inviteErr := TestSuite.DbConn.First(&stored, created.ID.Uint64()).Error

	// then
	require.NoError(t, countErr)
	require.NoError(t, inviteErr)
	assert.Equal(t, 1, success)
	assert.Equal(t, 1, unavailable)
	assert.Equal(t, int64(1), memberships)
	assert.NotNil(t, stored.ConsumedAt)
}

func TestGroupInviteService_ListIsDeterministicallyPaginated(t *testing.T) {
	// given
	service := setupGroupInviteServiceTest(t)
	_, adminCtx := seedInviteUser(t, "admin-pages@example.com", userEntities.RoleAdmin)
	group := seedGroup(t, "Grupo Páginas")
	createdIDs := make([]uint64, 11)
	for index := range createdIDs {
		created, err := service.Create(adminCtx, group.ID)
		require.NoError(t, err)
		createdIDs[index] = created.ID.Uint64()
	}

	// when
	first, firstErr := service.List(adminCtx, group.ID, &messages.ListGroupInvitesFilterDTO{})
	secondFilter := &messages.ListGroupInvitesFilterDTO{}
	secondFilter.SetPage(1)
	second, secondErr := service.List(adminCtx, group.ID, secondFilter)

	// then
	require.NoError(t, firstErr)
	require.NoError(t, secondErr)
	assert.Len(t, first.Data, 10)
	assert.True(t, first.Pagination.HasNextPage)
	assert.Equal(t, createdIDs[10], first.Data[0].ID.Uint64())
	assert.Len(t, second.Data, 1)
	assert.Equal(t, createdIDs[0], second.Data[0].ID.Uint64())
	assert.False(t, second.Pagination.HasNextPage)
}
