package services

import (
	"context"
	"errors"
	"strings"

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
)

type UserService struct {
	*BaseService
	userRepository       userInterfaces.UserRepositoryInterface
	groupRepository      groupInterfaces.GroupRepositoryInterface
	membershipRepository membershipInterfaces.GroupMembershipRepositoryInterface
}

func NewUserService(
	baseService *BaseService,
	userRepository userInterfaces.UserRepositoryInterface,
	groupRepository groupInterfaces.GroupRepositoryInterface,
	membershipRepository membershipInterfaces.GroupMembershipRepositoryInterface,
) interfaces.UserServiceInterface {
	return &UserService{
		BaseService:          baseService,
		userRepository:       userRepository,
		groupRepository:      groupRepository,
		membershipRepository: membershipRepository,
	}
}

func (s *UserService) UpdateGroup(ctx context.Context, userID uint64, request *messages.UpdateUserGroupRequestDTO) (*messages.UserResponseDTO, error) {
	user, err := s.userRepository.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, appErrors.ErrNotFound) {
			return nil, appErrors.ErrNotFound
		}
		return nil, appErrors.InternalError
	}

	var group *groupEntities.Group

	switch {
	case request.GroupID > 0:
		if !s.groupRepository.ExistsByID(ctx, request.GroupID) {
			return nil, appErrors.NewError("Erro ao atualizar grupo.", []*appErrors.FieldError{
				appErrors.NewFieldError("groupId", "grupo não encontrado"),
			})
		}
		group, err = s.groupRepository.FindByID(ctx, request.GroupID)
		if err != nil {
			return nil, appErrors.InternalError
		}
	case strings.TrimSpace(request.GroupName) != "":
		groupName := strings.TrimSpace(request.GroupName)
		group, err = s.groupRepository.FindByNameExact(ctx, groupName)
		if err != nil {
			return nil, appErrors.InternalError
		}
		if group == nil {
			if err := s.WithTransaction(ctx, func(ctx context.Context) error {
				created, err := s.groupRepository.Create(ctx, &groupEntities.Group{Name: groupName})
				if err != nil {
					return appErrors.InternalError
				}
				group = created
				return nil
			}); err != nil {
				return nil, err
			}
		}
	default:
		return nil, appErrors.NewError("Erro ao atualizar grupo.", []*appErrors.FieldError{
			appErrors.NewFieldError("groupId", "informe groupId ou groupName"),
		})
	}

	var updated *userEntities.User
	err = s.WithTransaction(ctx, func(txCtx context.Context) error {
		user.GroupID = &group.ID
		var updateErr error
		updated, updateErr = s.userRepository.Update(txCtx, user)
		if updateErr != nil {
			return updateErr
		}
		_, updateErr = s.membershipRepository.UpsertForUser(txCtx, &membershipEntities.GroupMembership{UserID: user.ID, GroupID: group.ID, JoinedAt: updated.UpdatedAt.UTC()})
		return updateErr
	})
	if err != nil {
		return nil, appErrors.InternalError
	}

	return mappers.MapUserToResponseDTO(updated, group), nil
}
