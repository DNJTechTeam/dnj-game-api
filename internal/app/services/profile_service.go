package services

import (
	"context"
	"errors"
	"net/http"
	"strings"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/app/mappers"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	groupEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/group/entities"
	groupInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/group/interfaces"
	userEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	userInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/user/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/common"
)

type ProfileService struct {
	*BaseService
	users  userInterfaces.UserRepositoryInterface
	groups groupInterfaces.GroupRepositoryInterface
}

func NewProfileService(base *BaseService, users userInterfaces.UserRepositoryInterface, groups groupInterfaces.GroupRepositoryInterface) interfaces.ProfileServiceInterface {
	return &ProfileService{BaseService: base, users: users, groups: groups}
}

func (s *ProfileService) response(ctx context.Context, user *userEntities.User) (*messages.CurrentProfileResponseDTO, error) {
	var group *groupEntities.Group
	var err error
	if user.GroupID != nil {
		group, err = s.groups.FindByID(ctx, *user.GroupID)
		if err != nil {
			return nil, appErrors.InternalError
		}
	}
	rank, err := s.users.RankPosition(ctx, user.ID, user.Points)
	if err != nil {
		return nil, appErrors.InternalError
	}
	return mappers.MapUserToCurrentProfileDTO(user, group, rank), nil
}

func (s *ProfileService) Current(ctx context.Context) (*messages.CurrentProfileResponseDTO, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return nil, identityError(http.StatusUnauthorized, "UNAUTHENTICATED", "Autenticação necessária.")
	}
	return s.response(ctx, user)
}

func (s *ProfileService) Update(ctx context.Context, request *messages.UpdateCurrentProfileRequestDTO) (*messages.CurrentProfileResponseDTO, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	if request.Name == nil && request.MobilePhone == nil {
		return nil, identityError(http.StatusBadRequest, "INVALID_REQUEST", "Informe name ou mobilePhone.")
	}
	user, err := s.users.FindByID(ctx, userID)
	if errors.Is(err, appErrors.ErrNotFound) {
		return nil, identityError(http.StatusUnauthorized, "UNAUTHENTICATED", "Autenticação necessária.")
	}
	if err != nil {
		return nil, appErrors.InternalError
	}
	if request.Name != nil {
		name := strings.TrimSpace(*request.Name)
		if len([]rune(name)) < 2 || len([]rune(name)) > 120 {
			return nil, identityError(http.StatusBadRequest, "INVALID_REQUEST", "name deve ter entre 2 e 120 caracteres.")
		}
		user.Name = name
	}
	if request.MobilePhone != nil {
		phone := common.SanitizePhone(*request.MobilePhone)
		if !common.ValidatePhone(phone, true) {
			return nil, identityError(http.StatusBadRequest, "INVALID_REQUEST", "mobilePhone deve ser um telefone móvel válido.")
		}
		user.MobilePhone = phone
	}
	user.OnboardingComplete = user.DocumentHash != "" && common.ValidatePhone(user.MobilePhone, true) && user.GroupID != nil
	updated, err := s.users.Update(ctx, user)
	if err != nil {
		return nil, appErrors.InternalError
	}
	return s.response(ctx, updated)
}
