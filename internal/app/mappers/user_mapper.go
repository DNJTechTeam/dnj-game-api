package mappers

import (
	"fmt"

	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	groupEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/group/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
)

func MapUserToResponseDTO(user *entities.User, group *groupEntities.Group) *messages.UserResponseDTO {
	if user == nil {
		return nil
	}

	return &messages.UserResponseDTO{
		ID:                 messages.Uint64StringFromUint64(user.ID),
		Email:              user.Email,
		Name:               user.Name,
		AvatarURL:          user.AvatarURL,
		MobilePhone:        user.MobilePhone,
		Document:           user.Document,
		DocumentMasked:     maskDocumentLast4(user.DocumentLast4),
		Role:               string(user.Role),
		OnboardingComplete: user.OnboardingComplete,
		CreatedAt:          user.CreatedAt,
		UpdatedAt:          user.UpdatedAt,
		Group:              MapGroupToSummaryDTO(group),
	}
}

func maskDocumentLast4(last4 string) string {
	if len(last4) != 4 {
		return ""
	}
	return fmt.Sprintf("***.***.*%s-%s", last4[:2], last4[2:])
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
