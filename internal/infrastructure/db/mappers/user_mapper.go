package mappers

import (
	"github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
)

func MapUserToEntity(user *models.User) *entities.User {
	if user == nil {
		return nil
	}

	return &entities.User{
		ID:                 user.ID,
		Email:              user.Email,
		Name:               user.Name,
		MobilePhone:        user.MobilePhone,
		Document:           user.Document,
		DocumentHash:       user.DocumentHash,
		DocumentLast4:      user.DocumentLast4,
		Role:               entities.UserRole(user.Role),
		ManagerScope:       user.ManagerScope,
		GroupID:            user.GroupID,
		Points:             user.Points,
		OnboardingComplete: user.OnboardingComplete,
		CreatedAt:          user.CreatedAt,
		UpdatedAt:          user.UpdatedAt,
	}
}

func MapEntityToUser(user *entities.User) *models.User {
	if user == nil {
		return nil
	}

	return &models.User{
		ID:                 user.ID,
		Email:              user.Email,
		Name:               user.Name,
		MobilePhone:        user.MobilePhone,
		Document:           user.Document,
		DocumentHash:       user.DocumentHash,
		DocumentLast4:      user.DocumentLast4,
		Role:               string(user.Role),
		ManagerScope:       user.ManagerScope,
		GroupID:            user.GroupID,
		Points:             user.Points,
		OnboardingComplete: user.OnboardingComplete,
		CreatedAt:          user.CreatedAt,
		UpdatedAt:          user.UpdatedAt,
	}
}
