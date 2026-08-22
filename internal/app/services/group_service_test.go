package services

import (
	"fmt"
	"testing"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	membershipEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/groupmembership/entities"
	userEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newGroupService() interfaces.GroupServiceInterface {
	return NewGroupService(TestSuite.BaseService, TestSuite.GroupRepository, TestSuite.UserRepository, TestSuite.GroupMembershipRepository)
}

func resetIteration3GroupTables(t *testing.T) {
	t.Helper()
	TestSuite.TruncateTable(t, &models.GroupInvite{})
	TestSuite.TruncateTable(t, &models.GroupMembership{})
	TestSuite.TruncateTable(t, &models.User{})
	TestSuite.TruncateTable(t, &models.Group{})
}

func TestGroupService_CurrentUpdateAndMembers(t *testing.T) {
	t.Run("joins changes and leaves while mirroring the legacy group id", func(t *testing.T) {
		// given
		TestSuite.DefaultSetup(t)
		resetIteration3GroupTables(t)
		service := newGroupService()
		first := seedGroup(t, "Grupo A")
		second := seedGroup(t, "Grupo B")
		user, err := TestSuite.UserRepository.Create(TestSuite.Ctx, &userEntities.User{Email: "member@example.com", Name: "Member", MobilePhone: "5541999990000", DocumentHash: "hash", DocumentLast4: "4725", Role: userEntities.RoleDefault})
		require.NoError(t, err)
		ctx := TestSuite.ContextWithUser(user.ID)

		// when
		joined, joinErr := service.UpdateCurrent(ctx, &messages.UpdateCurrentGroupRequestDTO{GroupID: messages.NullableUint64String{Set: true, Valid: true, Value: first.ID}})
		changed, changeErr := service.UpdateCurrent(ctx, &messages.UpdateCurrentGroupRequestDTO{GroupID: messages.NullableUint64String{Set: true, Valid: true, Value: second.ID}})
		current, currentErr := service.Current(ctx)
		left, leaveErr := service.UpdateCurrent(ctx, &messages.UpdateCurrentGroupRequestDTO{GroupID: messages.NullableUint64String{Set: true, Valid: false}})
		stored, storedErr := TestSuite.UserRepository.FindByID(TestSuite.Ctx, user.ID)
		_, membershipErr := TestSuite.GroupMembershipRepository.FindByUser(TestSuite.Ctx, user.ID)

		// then
		require.NoError(t, joinErr)
		require.NoError(t, changeErr)
		require.NoError(t, currentErr)
		require.NoError(t, leaveErr)
		require.NoError(t, storedErr)
		assert.Equal(t, first.ID, joined.Group.ID.Uint64())
		assert.Equal(t, second.ID, changed.Group.ID.Uint64())
		assert.Equal(t, second.ID, current.Group.ID.Uint64())
		assert.Nil(t, left.Group)
		assert.Nil(t, stored.GroupID)
		assert.False(t, stored.OnboardingComplete)
		assert.ErrorIs(t, membershipErr, appErrors.ErrNotFound)
	})

	t.Run("lists only current group members with deterministic pagination and no pii", func(t *testing.T) {
		// given
		TestSuite.DefaultSetup(t)
		resetIteration3GroupTables(t)
		service := newGroupService()
		ownGroup := seedGroup(t, "Grupo Próprio")
		otherGroup := seedGroup(t, "Grupo Alheio")
		var owner *userEntities.User
		for i := range 12 {
			groupID := ownGroup.ID
			created, err := TestSuite.UserRepository.Create(TestSuite.Ctx, &userEntities.User{Email: fmt.Sprintf("member-%02d@example.com", i), Name: fmt.Sprintf("Member %02d", i), Role: userEntities.RoleDefault, GroupID: &groupID})
			require.NoError(t, err)
			_, err = TestSuite.GroupMembershipRepository.UpsertForUser(TestSuite.Ctx, &membershipEntities.GroupMembership{UserID: created.ID, GroupID: groupID, JoinedAt: time.Date(2026, 8, 22, 12, i, 0, 0, time.UTC)})
			require.NoError(t, err)
			if i == 0 {
				owner = created
			}
		}
		otherID := otherGroup.ID
		other, err := TestSuite.UserRepository.Create(TestSuite.Ctx, &userEntities.User{Email: "secret@example.com", Name: "Secret Member", Role: userEntities.RoleAdmin, GroupID: &otherID})
		require.NoError(t, err)
		_, err = TestSuite.GroupMembershipRepository.UpsertForUser(TestSuite.Ctx, &membershipEntities.GroupMembership{UserID: other.ID, GroupID: otherID, JoinedAt: time.Now().UTC()})
		require.NoError(t, err)
		ctx := TestSuite.ContextWithUser(owner.ID)

		// when
		firstPage, firstErr := service.Members(ctx, &messages.ListGroupMembersFilterDTO{})
		secondFilter := &messages.ListGroupMembersFilterDTO{}
		secondFilter.SetPage(1)
		secondPage, secondErr := service.Members(ctx, secondFilter)

		// then
		require.NoError(t, firstErr)
		require.NoError(t, secondErr)
		assert.Len(t, firstPage.Data, 10)
		assert.True(t, firstPage.Pagination.HasNextPage)
		assert.Len(t, secondPage.Data, 2)
		for _, member := range append(firstPage.Data, secondPage.Data...) {
			assert.NotEqual(t, "Secret Member", member.Name)
		}
	})
}

func TestGroupService_ErrorsDoNotEnumerateAcrossGroups(t *testing.T) {
	// given
	TestSuite.DefaultSetup(t)
	resetIteration3GroupTables(t)
	service := newGroupService()
	user, err := TestSuite.UserRepository.Create(TestSuite.Ctx, &userEntities.User{Email: "nogroup@example.com", Name: "No Group", Role: userEntities.RoleDefault})
	require.NoError(t, err)
	ctx := TestSuite.ContextWithUser(user.ID)

	// when
	_, missingFieldErr := service.UpdateCurrent(ctx, &messages.UpdateCurrentGroupRequestDTO{})
	_, unknownGroupErr := service.UpdateCurrent(ctx, &messages.UpdateCurrentGroupRequestDTO{GroupID: messages.NullableUint64String{Set: true, Valid: true, Value: 999999}})
	_, noMembershipErr := service.Members(ctx, &messages.ListGroupMembersFilterDTO{})

	// then
	for _, err := range []error{missingFieldErr, unknownGroupErr, noMembershipErr} {
		var apiErr *appErrors.APIServiceError
		require.ErrorAs(t, err, &apiErr)
	}
}

func TestGroupService_Search(t *testing.T) {
	TestSuite.DefaultSetup(t)
	TestSuite.TruncateTable(t, &models.Group{})

	service := newGroupService()

	t.Run("less than 3 characters returns validation error", func(t *testing.T) {
		response, err := service.Search(TestSuite.Ctx, "ab", &messages.ListGroupsFilterDTO{})
		require.Error(t, err)
		assert.Nil(t, response)
		assert.IsType(t, &appErrors.APIServiceError{}, err)
	})

	t.Run("returns matching results ordered by name", func(t *testing.T) {
		seedGroup(t, "Grupo Jovens A")
		seedGroup(t, "Grupo Jovens B")
		seedGroup(t, "Outro Grupo")

		response, err := service.Search(TestSuite.Ctx, "jovens", &messages.ListGroupsFilterDTO{})
		require.NoError(t, err)
		require.Len(t, response.Data, 2)
		assert.Equal(t, "Grupo Jovens A", response.Data[0].GroupName)
		assert.Equal(t, "Grupo Jovens B", response.Data[1].GroupName)
	})

	t.Run("limited to 20 results", func(t *testing.T) {
		TestSuite.TruncateTable(t, &models.Group{})
		for i := range 25 {
			seedGroup(t, fmt.Sprintf("Limitado %02d", i))
		}

		response, err := service.Search(TestSuite.Ctx, "Limitado", &messages.ListGroupsFilterDTO{})
		require.NoError(t, err)
		assert.Len(t, response.Data, 20)
		assert.True(t, response.Pagination.HasNextPage)
	})
}
