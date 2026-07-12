package validators

import (
	"context"
	"strings"

	appErrors "github.com/scienceandcode/nucleus-api/internal/app/errors"
	uInterfaces "github.com/scienceandcode/nucleus-api/internal/domain/user/interfaces"
	"github.com/scienceandcode/nucleus-api/internal/infrastructure/common"
)

type UserValidator struct {
	userRepository uInterfaces.UserRepositoryInterface
}

type ValidatableUserFields struct {
	ID          uint64
	Email       string
	Name        string
	Password    string
	MobilePhone string
}

func NewUserValidator(userRepository uInterfaces.UserRepositoryInterface) *UserValidator {
	return &UserValidator{userRepository}
}

func (v *UserValidator) ValidateFields(ctx context.Context, request *ValidatableUserFields) []*appErrors.FieldError {
	var fieldErrors []*appErrors.FieldError

	emailErr := v.validateEmail(ctx, request)
	if emailErr != nil {
		fieldErrors = append(fieldErrors, emailErr)
	}

	minNameLength := 3
	if len(request.Name) < minNameLength {
		fieldErrors = append(fieldErrors, appErrors.NewFieldError("name", "O nome é obrigatório e deve conter pelo menos 3 caracteres."))
	}

	if !common.ValidatePhone(request.MobilePhone, true) {
		fieldErrors = append(fieldErrors, appErrors.NewFieldError("mobilePhone", "O telefone informado não é válido."))
	}

	if request.ID == 0 {
		if !common.ValidatePasswordComplexity(request.Password) {
			fieldErrors = append(fieldErrors, appErrors.NewFieldError(
				"password",
				"A senha deve conter pelo menos 8 caracteres, um maiúsculo, um minúsculo, um número e um caractere especial.",
			))
		}
	}

	return fieldErrors
}

func (v *UserValidator) validateEmail(ctx context.Context, request *ValidatableUserFields) *appErrors.FieldError {
	email := strings.ToLower(request.Email)
	if !common.ValidateEmail(email) {
		return appErrors.NewFieldError("email", "O email informado não é válido.")
	}

	user, err := v.userRepository.FindByEmail(ctx, email)
	if err != nil {
		return appErrors.NewFieldError("email", "Ocorreu um erro ao verificar o email informado.")
	}

	if user != nil && user.ID != request.ID {
		return appErrors.NewFieldError("email", "O email informado já está sendo utilizado.")
	}

	return nil
}
