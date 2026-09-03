package services

import (
	"context"
	"encoding/base64"
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

const maxAvatarDataURLLength = 4_500_000

func normalizeAvatarURL(raw string) (*string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, nil
	}
	if strings.HasPrefix(value, "https://") {
		if len(value) > 2048 {
			return nil, identityError(http.StatusBadRequest, "INVALID_REQUEST", "avatarUrl inválido.")
		}
		return &value, nil
	}
	if len(value) > maxAvatarDataURLLength {
		return nil, identityError(http.StatusRequestEntityTooLarge, "IMAGE_TOO_LARGE", "A foto de perfil deve ter no máximo 3 MB.")
	}
	for _, contentType := range []string{"jpeg", "png", "webp"} {
		prefix := "data:image/" + contentType + ";base64,"
		if strings.HasPrefix(value, prefix) {
			decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, prefix))
			if err != nil || len(decoded) > 3*1024*1024 {
				return nil, identityError(http.StatusBadRequest, "INVALID_REQUEST", "avatarUrl inválido.")
			}
			return &value, nil
		}
	}
	return nil, identityError(http.StatusBadRequest, "INVALID_REQUEST", "avatarUrl deve ser uma URL HTTPS ou uma imagem válida.")
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
	if request.Name == nil && request.MobilePhone == nil && request.AvatarURL == nil {
		return nil, identityError(http.StatusBadRequest, "INVALID_REQUEST", "Informe name, mobilePhone ou avatarUrl.")
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
	if request.AvatarURL != nil {
		avatarURL, avatarErr := normalizeAvatarURL(*request.AvatarURL)
		if avatarErr != nil {
			return nil, avatarErr
		}
		user.AvatarURL = avatarURL
	}
	user.OnboardingComplete = user.DocumentHash != "" && common.ValidatePhone(user.MobilePhone, true) && user.GroupID != nil
	updated, err := s.users.Update(ctx, user)
	if err != nil {
		return nil, appErrors.InternalError
	}
	return s.response(ctx, updated)
}
