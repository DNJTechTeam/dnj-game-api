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
	inviteEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/groupinvite/entities"
	inviteInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/groupinvite/interfaces"
	membershipEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/groupmembership/entities"
	membershipInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/groupmembership/interfaces"
	userEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	userInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/user/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/common"
)

const GroupInviteTTL = 7 * 24 * time.Hour

var unavailableInviteError = appErrors.NewAPIServiceError(http.StatusNotFound, "INVITE_NOT_FOUND_OR_UNAVAILABLE", "Convite não encontrado ou indisponível.", nil)

type GroupInviteService struct {
	*BaseService
	users       userInterfaces.UserRepositoryInterface
	groups      groupInterfaces.GroupRepositoryInterface
	memberships membershipInterfaces.GroupMembershipRepositoryInterface
	invites     inviteInterfaces.GroupInviteRepositoryInterface
	now         func() time.Time
}

func NewGroupInviteService(base *BaseService, users userInterfaces.UserRepositoryInterface, groups groupInterfaces.GroupRepositoryInterface, memberships membershipInterfaces.GroupMembershipRepositoryInterface, invites inviteInterfaces.GroupInviteRepositoryInterface) interfaces.GroupInviteServiceInterface {
	return &GroupInviteService{BaseService: base, users: users, groups: groups, memberships: memberships, invites: invites, now: time.Now}
}

func (s *GroupInviteService) requireAdmin(ctx context.Context) (*userEntities.User, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return nil, identityError(http.StatusUnauthorized, "UNAUTHENTICATED", "Autenticação necessária.")
	}
	if user.Role != userEntities.RoleAdmin {
		return nil, identityError(http.StatusForbidden, "FORBIDDEN", "Operação permitida somente para administradores.")
	}
	return user, nil
}

func (s *GroupInviteService) requireGroup(ctx context.Context, groupID uint64) (*groupEntities.Group, error) {
	group, err := s.groups.FindByID(ctx, groupID)
	if errors.Is(err, appErrors.ErrNotFound) {
		return nil, identityError(http.StatusNotFound, "GROUP_NOT_FOUND", "Grupo não encontrado.")
	}
	if err != nil {
		return nil, appErrors.InternalError
	}
	return group, nil
}

func (s *GroupInviteService) newInvite(ctx context.Context, groupID uint64, creatorID uint64, replaces *uint64) (*messages.GroupInviteResponseDTO, error) {
	rawCode, err := randomOpaqueToken(16)
	if err != nil {
		return nil, appErrors.InternalError
	}
	now := s.now().UTC()
	created, err := s.invites.Create(ctx, &inviteEntities.GroupInvite{GroupID: groupID, CodeHash: tokenHash(rawCode), ExpiresAt: now.Add(GroupInviteTTL), CreatedByUserID: creatorID, ReplacesInviteID: replaces})
	if err != nil {
		return nil, appErrors.InternalError
	}
	return mappers.MapInviteToResponseDTO(created, now, rawCode), nil
}

func (s *GroupInviteService) Create(ctx context.Context, groupID uint64) (*messages.GroupInviteResponseDTO, error) {
	admin, err := s.requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if _, err = s.requireGroup(ctx, groupID); err != nil {
		return nil, err
	}
	return s.newInvite(ctx, groupID, admin.ID, nil)
}

func (s *GroupInviteService) Renew(ctx context.Context, groupID uint64, inviteID uint64) (*messages.GroupInviteResponseDTO, error) {
	admin, err := s.requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if _, err = s.requireGroup(ctx, groupID); err != nil {
		return nil, err
	}
	current, err := s.invites.FindByIDAndGroup(ctx, inviteID, groupID)
	if errors.Is(err, appErrors.ErrNotFound) {
		return nil, unavailableInviteError
	}
	if err != nil {
		return nil, appErrors.InternalError
	}
	if current.RevokedAt != nil || current.ConsumedAt != nil {
		return nil, identityError(http.StatusConflict, "INVITE_UNAVAILABLE", "Convite não pode ser renovado.")
	}
	var response *messages.GroupInviteResponseDTO
	err = s.WithTransaction(ctx, func(txCtx context.Context) error {
		revoked, revokeErr := s.invites.RevokeAvailable(txCtx, inviteID, groupID, s.now().UTC())
		if revokeErr != nil {
			return revokeErr
		}
		if !revoked {
			return unavailableInviteError
		}
		response, revokeErr = s.newInvite(txCtx, groupID, admin.ID, &inviteID)
		return revokeErr
	})
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (s *GroupInviteService) Revoke(ctx context.Context, groupID uint64, inviteID uint64) error {
	if _, err := s.requireAdmin(ctx); err != nil {
		return err
	}
	if _, err := s.requireGroup(ctx, groupID); err != nil {
		return err
	}
	current, err := s.invites.FindByIDAndGroup(ctx, inviteID, groupID)
	if errors.Is(err, appErrors.ErrNotFound) {
		return unavailableInviteError
	}
	if err != nil {
		return appErrors.InternalError
	}
	if current.RevokedAt != nil {
		return nil
	}
	if current.ConsumedAt != nil {
		return identityError(http.StatusConflict, "INVITE_UNAVAILABLE", "Convite já consumido não pode ser revogado.")
	}
	_, err = s.invites.RevokeAvailable(ctx, inviteID, groupID, s.now().UTC())
	if err != nil {
		return appErrors.InternalError
	}
	return nil
}

func (s *GroupInviteService) List(ctx context.Context, groupID uint64, filter *messages.ListGroupInvitesFilterDTO) (*messages.PaginatedResponse[messages.GroupInviteResponseDTO], error) {
	if _, err := s.requireAdmin(ctx); err != nil {
		return nil, err
	}
	if _, err := s.requireGroup(ctx, groupID); err != nil {
		return nil, err
	}
	result, err := s.invites.ListByGroup(ctx, groupID, filter.GetPage())
	if err != nil {
		return nil, appErrors.InternalError
	}
	now := s.now().UTC()
	items := make([]messages.GroupInviteResponseDTO, len(result.Data))
	for i := range result.Data {
		items[i] = *mappers.MapInviteToResponseDTO(&result.Data[i], now, "")
	}
	return &messages.PaginatedResponse[messages.GroupInviteResponseDTO]{Data: items, Pagination: result.Pagination}, nil
}

func (s *GroupInviteService) currentGroupResponse(ctx context.Context, user *userEntities.User, group *groupEntities.Group) (*messages.CurrentGroupResponseDTO, error) {
	membership, err := s.memberships.FindByUser(ctx, user.ID)
	if err != nil || membership.GroupID != group.ID {
		return nil, appErrors.InternalError
	}
	return &messages.CurrentGroupResponseDTO{Group: mappers.MapGroupToSummaryDTO(group), Membership: mappers.MapMembershipToResponseDTO(membership)}, nil
}

func (s *GroupInviteService) Consume(ctx context.Context, request *messages.ConsumeGroupInviteRequestDTO) (*messages.CurrentGroupResponseDTO, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	code := strings.TrimSpace(request.Code)
	if code == "" {
		return nil, unavailableInviteError
	}
	invite, err := s.invites.FindByHash(ctx, tokenHash(code))
	if err != nil {
		return nil, unavailableInviteError
	}
	now := s.now().UTC()
	if invite.RevokedAt != nil || !invite.ExpiresAt.After(now) {
		return nil, unavailableInviteError
	}
	group, err := s.requireGroup(ctx, invite.GroupID)
	if err != nil {
		return nil, unavailableInviteError
	}
	if invite.ConsumedAt != nil {
		if invite.ConsumedByUserID == nil || *invite.ConsumedByUserID != userID {
			return nil, unavailableInviteError
		}
		user, findErr := s.users.FindByID(ctx, userID)
		if findErr != nil || user.GroupID == nil || *user.GroupID != invite.GroupID {
			return nil, unavailableInviteError
		}
		return s.currentGroupResponse(ctx, user, group)
	}
	var updated *userEntities.User
	err = s.WithTransaction(ctx, func(txCtx context.Context) error {
		user, findErr := s.users.FindByIDForUpdate(txCtx, userID)
		if findErr != nil {
			return findErr
		}
		consumed, consumeErr := s.invites.ConsumeAvailable(txCtx, invite.ID, userID, now)
		if consumeErr != nil {
			return consumeErr
		}
		if !consumed {
			latest, latestErr := s.invites.FindByHash(txCtx, tokenHash(code))
			if latestErr == nil && latest.ConsumedByUserID != nil && *latest.ConsumedByUserID == userID {
				updated = user
				return nil
			}
			return unavailableInviteError
		}
		joinedAt := now
		current, membershipErr := s.memberships.FindByUser(txCtx, userID)
		if membershipErr == nil && current.GroupID == invite.GroupID {
			joinedAt = current.JoinedAt
		}
		if membershipErr != nil && !errors.Is(membershipErr, appErrors.ErrNotFound) {
			return membershipErr
		}
		if _, upsertErr := s.memberships.UpsertForUser(txCtx, &membershipEntities.GroupMembership{UserID: userID, GroupID: invite.GroupID, JoinedAt: joinedAt}); upsertErr != nil {
			return upsertErr
		}
		groupID := invite.GroupID
		user.GroupID = &groupID
		user.OnboardingComplete = user.DocumentHash != "" && common.ValidatePhone(user.MobilePhone, true)
		updated, findErr = s.users.Update(txCtx, user)
		return findErr
	})
	if errors.Is(err, appErrors.ErrNotFound) {
		return nil, identityError(http.StatusUnauthorized, "UNAUTHENTICATED", "Autenticação necessária.")
	}
	if err != nil {
		return nil, err
	}
	return s.currentGroupResponse(ctx, updated, group)
}
