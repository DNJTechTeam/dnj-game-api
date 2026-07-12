package services

import (
	"context"
	"strconv"
	"time"

	appErrors "github.com/scienceandcode/nucleus-api/internal/app/errors"
	"github.com/scienceandcode/nucleus-api/internal/app/interfaces"
	"github.com/scienceandcode/nucleus-api/internal/app/mappers"
	"github.com/scienceandcode/nucleus-api/internal/app/messages"
	"github.com/scienceandcode/nucleus-api/internal/app/validators"
	"github.com/scienceandcode/nucleus-api/internal/domain/user/entities"
	rInterfaces "github.com/scienceandcode/nucleus-api/internal/domain/user/interfaces"
	"github.com/scienceandcode/nucleus-api/internal/infrastructure/common"

	"golang.org/x/crypto/bcrypt"
)

type UserAuthService struct {
	*BaseService
	userRepository           rInterfaces.UserRepositoryInterface
	jwtService               interfaces.JwtServiceInterface
	emailService             interfaces.EmailServiceInterface
	emailConfirmationService interfaces.EmailConfirmationServiceInterface
}

func NewUserAuthService(
	baseService *BaseService,
	userRepository rInterfaces.UserRepositoryInterface,
	jwtService interfaces.JwtServiceInterface,
	emailService interfaces.EmailServiceInterface,
	emailConfirmationService interfaces.EmailConfirmationServiceInterface,
) interfaces.UserAuthServiceInterface {
	return &UserAuthService{
		BaseService:              baseService,
		userRepository:           userRepository,
		jwtService:               jwtService,
		emailService:             emailService,
		emailConfirmationService: emailConfirmationService,
	}
}

// Register creates a new user, marks the password as confirmed (self-service
// signup picks its own password) and emails an email-confirmation link.
func (s *UserAuthService) Register(
	ctx context.Context,
	request *messages.RegisterRequestDTO,
) (*messages.CreateUserResponseDTO, error) {
	if err := s.validateRegisterRequest(ctx, request); err != nil {
		return nil, err
	}

	userEntity := mappers.MapRegisterRequestDTOToEntity(request)

	err := s.WithTransaction(ctx, func(tCtx context.Context) error {
		timeNow := time.Now()
		userEntity.PasswordConfirmedAt = &timeNow

		created, err := s.userRepository.Create(tCtx, userEntity)
		if err != nil {
			return err
		}
		userEntity = created

		confirmationToken, err := s.emailConfirmationService.GenerateConfirmationToken(ctx, userEntity.ID, userEntity.Email)
		if err != nil {
			return appErrors.NewError("Erro ao gerar código de confirmação para o email.", nil)
		}

		if err := s.emailService.SendEmailConfirmation(ctx, userEntity.Email, confirmationToken); err != nil {
			return appErrors.NewError("Erro ao enviar email de confirmação.", nil)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return mappers.MapUserToResponseDTO(userEntity), nil
}

// Login validates credentials and returns the response DTO plus the signed
// identity token. The caller (handler) is responsible for setting the cookie.
func (s *UserAuthService) Login(
	ctx context.Context,
	request *messages.LoginRequestDTO,
) (*messages.LoginResponseDTO, string, error) {
	user, err := s.validateLoginCredentials(ctx, request)
	if err != nil {
		return nil, "", err
	}

	identityToken, err := s.jwtService.GenerateIdentityToken(ctx, user)
	if err != nil {
		return nil, "", appErrors.InternalError
	}

	response := &messages.LoginResponseDTO{
		HasUpdatedPassword: user.PasswordConfirmedAt != nil,
		EmailConfirmed:     user.EmailConfirmedAt != nil,
		Email:              user.Email,
	}

	return response, identityToken, nil
}

func (s *UserAuthService) ConfirmEmail(
	ctx context.Context,
	token string,
) (*messages.ConfirmEmailResponseDTO, error) {
	userID := common.ExtractUserIdFromContext(ctx)

	claims, err := s.emailConfirmationService.ValidateConfirmationToken(ctx, token)
	if err != nil {
		return nil, appErrors.NewError("Token de confirmação inválido ou expirado.", nil)
	}

	parsedUserID, err := strconv.ParseUint(claims.UserID, 10, 64)
	if err != nil || parsedUserID != userID {
		return nil, appErrors.NewError("Token de confirmação inválido.", nil)
	}

	user, err := s.userRepository.FindByID(ctx, userID)
	if err != nil {
		return nil, appErrors.NewError("Erro ao buscar usuário.", nil)
	}

	if user.Email != claims.Email {
		return nil, appErrors.NewError("Token de confirmação inválido.", nil)
	}

	if user.EmailConfirmedAt != nil {
		return nil, appErrors.NewError("Email já foi confirmado anteriormente.", nil)
	}

	timeNow := time.Now()
	err = s.WithTransaction(ctx, func(tCtx context.Context) error {
		user.EmailConfirmedAt = &timeNow
		_, err := s.userRepository.Update(tCtx, user)
		return err
	})

	if err != nil {
		return nil, appErrors.NewError("Erro ao confirmar email.", nil)
	}

	return &messages.ConfirmEmailResponseDTO{
		Message: "Email confirmado com sucesso.",
	}, nil
}

func (s *UserAuthService) ResendConfirmationEmail(
	ctx context.Context,
) (*messages.ResendConfirmationEmailResponseDTO, error) {
	userID := common.ExtractUserIdFromContext(ctx)

	user, err := s.userRepository.FindByID(ctx, userID)
	if err != nil {
		return nil, appErrors.NewError("Erro ao buscar usuário.", nil)
	}

	if user.EmailConfirmedAt != nil {
		return nil, appErrors.NewError("Email já foi confirmado.", nil)
	}

	confirmationToken, err := s.emailConfirmationService.GenerateConfirmationToken(ctx, user.ID, user.Email)
	if err != nil {
		return nil, appErrors.NewError("Erro ao gerar token de confirmação.", nil)
	}

	if err := s.emailService.SendEmailConfirmation(context.Background(), user.Email, confirmationToken); err != nil {
		return nil, appErrors.NewError("Erro ao enviar email de confirmação. Tente novamente.", nil)
	}

	return &messages.ResendConfirmationEmailResponseDTO{
		Message: "Email de confirmação reenviado com sucesso.",
	}, nil
}

// ForgotPassword always succeeds from the caller's point of view so it cannot
// be used to probe which emails are registered.
func (s *UserAuthService) ForgotPassword(
	ctx context.Context,
	request *messages.ForgotPasswordRequestDTO,
) error {
	return s.WithTransaction(ctx, func(tCtx context.Context) error {
		user, _ := s.userRepository.FindByEmail(ctx, request.Email)
		if user == nil {
			return nil
		}

		user.Password = common.GenerateRandomPassword()
		user.PasswordConfirmedAt = nil

		if _, err := s.userRepository.Update(tCtx, user); err != nil {
			return err
		}

		if err := s.emailService.SendPasswordRecoveryEmail(context.Background(), user.Email, user.Password); err != nil {
			return appErrors.NewError("Erro ao enviar email de recuperação de senha. Tente novamente.", nil)
		}

		return nil
	})
}

// UpdatePassword sets the definitive password for a user whose password was
// reset (PasswordConfirmedAt is nil) — e.g. after a password recovery.
func (s *UserAuthService) UpdatePassword(
	ctx context.Context,
	request *messages.UpdatePasswordRequestDTO,
) (*messages.UpdatePasswordResponseDTO, error) {
	userID := common.ExtractUserIdFromContext(ctx)

	if request == nil {
		return nil, appErrors.NewError("Erro ao atualizar senha. Verifique os campos e tente novamente.", []*appErrors.FieldError{
			appErrors.NewFieldError("*", "Nenhuma informação foi fornecida."),
		})
	}

	user, err := s.userRepository.FindByID(ctx, userID)
	if err != nil {
		return nil, appErrors.NewError("Erro ao buscar usuário.", nil)
	}

	if user.PasswordConfirmedAt != nil {
		return nil, appErrors.NewError("A senha já foi atualizada anteriormente.", nil)
	}

	if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(request.Password)) == nil {
		return nil, appErrors.NewError("A senha nova não pode ser igual à senha anterior.", nil)
	}

	if !common.ValidatePasswordComplexity(request.Password) {
		return nil, appErrors.NewError("Erro ao atualizar senha. Verifique os campos e tente novamente.", []*appErrors.FieldError{
			appErrors.NewFieldError("password", "A senha deve conter pelo menos 8 caracteres, um maiúsculo, um minúsculo, um número e um caractere especial."),
		})
	}

	timeNow := time.Now()
	err = s.WithTransaction(ctx, func(tCtx context.Context) error {
		user.Password = request.Password
		user.PasswordConfirmedAt = &timeNow
		_, err := s.userRepository.Update(tCtx, user)
		return err
	})

	if err != nil {
		return nil, appErrors.NewError("Erro ao atualizar senha. Verifique os campos e tente novamente.", nil)
	}

	return &messages.UpdatePasswordResponseDTO{
		Message: "Senha atualizada com sucesso.",
	}, nil
}

func (s *UserAuthService) validateRegisterRequest(
	ctx context.Context,
	request *messages.RegisterRequestDTO,
) error {
	errorMessage := "Erro ao registrar-se. Verifique os campos e tente novamente."

	if request == nil {
		return appErrors.NewError(errorMessage, []*appErrors.FieldError{
			appErrors.NewFieldError("*", "Nenhuma informação foi fornecida."),
		})
	}

	userValidator := validators.NewUserValidator(s.userRepository)
	fieldErrors := userValidator.ValidateFields(ctx, &validators.ValidatableUserFields{
		Email:       request.Email,
		Name:        request.Name,
		Password:    request.Password,
		MobilePhone: request.MobilePhone,
	})

	if len(fieldErrors) > 0 {
		return appErrors.NewError(errorMessage, fieldErrors)
	}

	return nil
}

func (s *UserAuthService) validateLoginCredentials(ctx context.Context, request *messages.LoginRequestDTO) (*entities.User, error) {
	err := appErrors.NewError("Credenciais inválidas.", nil)

	if request == nil {
		return nil, err
	}

	if !common.ValidateEmail(request.Email) {
		return nil, err
	}

	user, _ := s.userRepository.FindByEmail(ctx, request.Email)
	if user == nil {
		return nil, err
	}

	if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(request.Password)) != nil {
		return nil, err
	}

	return user, nil
}
