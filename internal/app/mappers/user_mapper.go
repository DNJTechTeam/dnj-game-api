package mappers

import (
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	groupEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/group/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
)

func MapUserToResponseDTO(user *entities.User, group *groupEntities.Group) *messages.UserResponseDTO {
	if user == nil {
		return nil
	}

	return &messages.UserResponseDTO{
		ID:          messages.Uint64StringFromUint64(user.ID),
		Email:       user.Email,
		Name:        user.Name,
		MobilePhone: user.MobilePhone,
		Document:    user.Document,
		Role:        string(user.Role),
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
		Group:       MapGroupToSummaryDTO(group),
	}
}

func MapUserToVerificationCodeResponseDTO(user *entities.User, group *groupEntities.Group, identityToken string) *messages.VerificationCodeResponseDTO {
	if user == nil {
		return nil
	}

	return &messages.VerificationCodeResponseDTO{
		UserResponseDTO: *MapUserToResponseDTO(user, group),
		IdentityToken:   identityToken,
	}
}
