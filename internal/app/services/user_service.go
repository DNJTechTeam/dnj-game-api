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
	userInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/user/interfaces"
)

type UserService struct {
	*BaseService
	userRepository  userInterfaces.UserRepositoryInterface
	groupRepository groupInterfaces.GroupRepositoryInterface
}

func NewUserService(
	baseService *BaseService,
	userRepository userInterfaces.UserRepositoryInterface,
	groupRepository groupInterfaces.GroupRepositoryInterface,
) interfaces.UserServiceInterface {
	return &UserService{
		BaseService:     baseService,
		userRepository:  userRepository,
		groupRepository: groupRepository,
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

	user.GroupID = &group.ID
	updated, err := s.userRepository.Update(ctx, user)
	if err != nil {
		return nil, appErrors.InternalError
	}

	return mappers.MapUserToResponseDTO(updated, group), nil
}
