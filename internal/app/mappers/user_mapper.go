package mappers

import (
	"strings"

	"github.com/scienceandcode/nucleus-api/internal/app/messages"
	"github.com/scienceandcode/nucleus-api/internal/domain/user/entities"
)

func MapRegisterRequestDTOToEntity(request *messages.RegisterRequestDTO) *entities.User {
	return &entities.User{
		Email:            strings.ToLower(request.Email),
		Name:             request.Name,
		Password:         request.Password,
		MobilePhone:      request.MobilePhone,
		EmailConfirmedAt: nil,
	}
}

func MapUserToResponseDTO(user *entities.User) *messages.CreateUserResponseDTO {
	if user == nil {
		return nil
	}

	return &messages.CreateUserResponseDTO{
		ID: messages.Uint64StringFromUint64(user.ID),
	}
}
