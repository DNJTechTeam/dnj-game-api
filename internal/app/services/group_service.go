package services

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/app/mappers"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	groupEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/group/entities"
	groupInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/group/interfaces"
	membershipEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/groupmembership/entities"
	membershipInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/groupmembership/interfaces"
	userEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	userInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/user/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/common"
)

type GroupService struct {
	*BaseService
	groups      groupInterfaces.GroupRepositoryInterface
	users       userInterfaces.UserRepositoryInterface
	memberships membershipInterfaces.GroupMembershipRepositoryInterface
	now         func() time.Time
}

func NewGroupService(base *BaseService, groups groupInterfaces.GroupRepositoryInterface, users userInterfaces.UserRepositoryInterface, memberships membershipInterfaces.GroupMembershipRepositoryInterface) interfaces.GroupServiceInterface {
	return &GroupService{BaseService: base, groups: groups, users: users, memberships: memberships, now: time.Now}
}

func authenticatedUserID(ctx context.Context) (uint64, error) {
	userID := common.ExtractUserIdFromContext(ctx)
	if userID == 0 {
		return 0, identityError(http.StatusUnauthorized, "UNAUTHENTICATED", "Autenticação necessária.")
	}
	return userID, nil
}

func (s *GroupService) Search(ctx context.Context, query string, filter *messages.ListGroupsFilterDTO) (*messages.PaginatedResponse[messages.GroupSummaryDTO], error) {
	trimmed := strings.TrimSpace(query)
	if trimmed != "" && len([]rune(trimmed)) < 3 {
		return nil, identityError(http.StatusBadRequest, "INVALID_REQUEST", "A busca deve ter pelo menos 3 caracteres.")
	}
	result, err := s.groups.SearchPage(ctx, trimmed, filter.GetPage())
	if err != nil {
		return nil, appErrors.InternalError
	}
	items := make([]messages.GroupSummaryDTO, len(result.Data))
	for i := range result.Data {
		items[i] = *mappers.MapGroupToSummaryDTO(&result.Data[i])
	}
	return &messages.PaginatedResponse[messages.GroupSummaryDTO]{Data: items, Pagination: result.Pagination}, nil
}

func (s *GroupService) currentForUser(ctx context.Context, user *userEntities.User) (*messages.CurrentGroupResponseDTO, error) {
	if user.GroupID == nil {
		return &messages.CurrentGroupResponseDTO{}, nil
	}
	group, err := s.groups.FindByID(ctx, *user.GroupID)
	if err != nil {
		return nil, appErrors.InternalError
	}
	membership, err := s.memberships.FindByUser(ctx, user.ID)
	if errors.Is(err, appErrors.ErrNotFound) {
		membership, err = s.memberships.UpsertForUser(ctx, &membershipEntities.GroupMembership{UserID: user.ID, GroupID: *user.GroupID, JoinedAt: user.UpdatedAt.UTC()})
	}
	if err != nil {
		return nil, appErrors.InternalError
	}
	return &messages.CurrentGroupResponseDTO{Group: mappers.MapGroupToSummaryDTO(group), Membership: mappers.MapMembershipToResponseDTO(membership)}, nil
}

func (s *GroupService) Current(ctx context.Context) (*messages.CurrentGroupResponseDTO, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return nil, identityError(http.StatusUnauthorized, "UNAUTHENTICATED", "Autenticação necessária.")
	}
	return s.currentForUser(ctx, user)
}

func (s *GroupService) UpdateCurrent(ctx context.Context, request *messages.UpdateCurrentGroupRequestDTO) (*messages.CurrentProfileResponseDTO, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	if !request.GroupID.Set || (request.GroupID.Valid && request.GroupID.Value == 0) {
		return nil, identityError(http.StatusBadRequest, "INVALID_REQUEST", "groupId deve ser um identificador válido ou null.")
	}
	var selected *groupEntities.Group
	if request.GroupID.Valid {
		selected, err = s.groups.FindByID(ctx, request.GroupID.Value)
		if errors.Is(err, appErrors.ErrNotFound) {
			return nil, identityError(http.StatusNotFound, "GROUP_NOT_FOUND", "Grupo não encontrado.")
		}
		if err != nil {
			return nil, appErrors.InternalError
		}
	}
	var updated *userEntities.User
	err = s.WithTransaction(ctx, func(txCtx context.Context) error {
		user, findErr := s.users.FindByIDForUpdate(txCtx, userID)
		if findErr != nil {
			return findErr
		}
		if selected == nil {
			if deleteErr := s.memberships.DeleteByUser(txCtx, userID); deleteErr != nil {
				return deleteErr
			}
			user.GroupID = nil
		} else {
			joinedAt := s.now().UTC()
			current, membershipErr := s.memberships.FindByUser(txCtx, userID)
			if membershipErr == nil && current.GroupID == selected.ID {
				joinedAt = current.JoinedAt
			}
			if membershipErr != nil && !errors.Is(membershipErr, appErrors.ErrNotFound) {
				return membershipErr
			}
			if _, upsertErr := s.memberships.UpsertForUser(txCtx, &membershipEntities.GroupMembership{UserID: userID, GroupID: selected.ID, JoinedAt: joinedAt}); upsertErr != nil {
				return upsertErr
			}
			groupID := selected.ID
			user.GroupID = &groupID
		}
		user.OnboardingComplete = user.DocumentHash != "" && common.ValidatePhone(user.MobilePhone, true) && user.GroupID != nil
		updated, findErr = s.users.Update(txCtx, user)
		return findErr
	})
	if errors.Is(err, appErrors.ErrNotFound) {
		return nil, identityError(http.StatusUnauthorized, "UNAUTHENTICATED", "Autenticação necessária.")
	}
	if err != nil {
		return nil, appErrors.InternalError
	}
	rank, err := s.users.RankPosition(ctx, updated.ID, updated.Points)
	if err != nil {
		return nil, appErrors.InternalError
	}
	return mappers.MapUserToCurrentProfileDTO(updated, selected, rank), nil
}

func (s *GroupService) Members(ctx context.Context, filter *messages.ListGroupMembersFilterDTO) (*messages.PaginatedResponse[messages.GroupMemberResponseDTO], error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return nil, identityError(http.StatusUnauthorized, "UNAUTHENTICATED", "Autenticação necessária.")
	}
	if user.GroupID == nil {
		return nil, identityError(http.StatusNotFound, "GROUP_MEMBERSHIP_NOT_FOUND", "Você não pertence a um grupo.")
	}
	result, err := s.memberships.ListMembers(ctx, *user.GroupID, filter.GetPage())
	if err != nil {
		return nil, appErrors.InternalError
	}
	items := make([]messages.GroupMemberResponseDTO, len(result.Data))
	for i := range result.Data {
		items[i] = mappers.MapMemberToResponseDTO(&result.Data[i])
	}
	return &messages.PaginatedResponse[messages.GroupMemberResponseDTO]{Data: items, Pagination: result.Pagination}, nil
}
