package mappers

import (
	"github.com/scienceandcode/nucleus-api/internal/domain/user/entities"
	"github.com/scienceandcode/nucleus-api/internal/infrastructure/db/models"
)

func MapUserToEntity(user *models.User) *entities.User {
	if user == nil {
		return nil
	}

	return &entities.User{
		ID:                  user.ID,
		Email:               user.Email,
		Name:                user.Name,
		MobilePhone:         user.MobilePhone,
		EmailConfirmedAt:    user.EmailConfirmedAt,
		PasswordConfirmedAt: user.PasswordConfirmedAt,
		Password:            user.Password,
		CreatedAt:           user.CreatedAt,
		UpdatedAt:           user.UpdatedAt,
	}
}

func MapEntityToUser(user *entities.User) *models.User {
	if user == nil {
		return nil
	}

	return &models.User{
		ID:                  user.ID,
		Email:               user.Email,
		Name:                user.Name,
		MobilePhone:         user.MobilePhone,
		EmailConfirmedAt:    user.EmailConfirmedAt,
		PasswordConfirmedAt: user.PasswordConfirmedAt,
		Password:            user.Password,
		CreatedAt:           user.CreatedAt,
		UpdatedAt:           user.UpdatedAt,
	}
}
