package services

import (
	"context"
	"strings"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/app/mappers"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	groupInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/group/interfaces"
	userEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	userInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/user/interfaces"
	svcInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/subscriptionwebhookverificationcode/interfaces"
)

// errLookupFailed is the single generic error returned for both "email not
// found" and "document mismatch" during onboarding, so the endpoint can't be
// used to enumerate which emails are registered.
var errLookupFailed = appErrors.NewError("Ocorreu um erro ao tentar localizar o jovem na base.", []*appErrors.FieldError{
	appErrors.NewFieldError("email", "email não localizado"),
})

var errInvalidVerificationCode = appErrors.NewError("Código de verificação inválido.", []*appErrors.FieldError{
	appErrors.NewFieldError("verificationCode", "código de verificação inválido"),
})

// AuthService implements the passwordless onboarding flow: a subscriber
// confirms their email+document (Onboarding), receives a 6-digit code by
// email, and exchanges it for an identity token (VerifyCode) — creating the
// User the first time the code is confirmed.
type AuthService struct {
	*BaseService
	verificationCodeRepository svcInterfaces.SubscriptionWebhookVerificationCodeRepositoryInterface
	userRepository              userInterfaces.UserRepositoryInterface
	groupRepository              groupInterfaces.GroupRepositoryInterface
	jwtService                   interfaces.JwtServiceInterface
	emailService                 interfaces.EmailServiceInterface
}

func NewAuthService(
	baseService *BaseService,
	verificationCodeRepository svcInterfaces.SubscriptionWebhookVerificationCodeRepositoryInterface,
	userRepository userInterfaces.UserRepositoryInterface,
	groupRepository groupInterfaces.GroupRepositoryInterface,
	jwtService interfaces.JwtServiceInterface,
	emailService interfaces.EmailServiceInterface,
) interfaces.AuthServiceInterface {
	return &AuthService{
		BaseService:                 baseService,
		verificationCodeRepository:  verificationCodeRepository,
		userRepository:              userRepository,
		groupRepository:             groupRepository,
		jwtService:                  jwtService,
		emailService:                emailService,
	}
}

// Onboarding looks up the pending verification code created from a
// subscription webhook and, when the document matches, (re)sends the code by
// email. Calling it again after the user already verified is allowed — it
// works as a "resend code to log in again" flow, reusing the same code since
// there is no TTL/rotation yet.
func (s *AuthService) Onboarding(ctx context.Context, request *messages.OnboardingRequestDTO) error {
	email := strings.ToLower(strings.TrimSpace(request.Email))

	code, err := s.verificationCodeRepository.FindByEmail(ctx, email)
	if err != nil {
		return appErrors.InternalError
	}
	if code == nil {
		return errLookupFailed
	}
	if strings.TrimSpace(code.Document) != strings.TrimSpace(request.Document) {
		return errLookupFailed
	}

	if err := s.emailService.SendVerificationCodeEmail(ctx, code.Email, code.VerificationCode); err != nil {
		return appErrors.NewSimpleError("Erro ao enviar email de verificação. Tente novamente.")
	}

	return nil
}

func (s *AuthService) VerifyCode(ctx context.Context, request *messages.VerificationCodeRequestDTO) (*messages.VerificationCodeResponseDTO, error) {
	email := strings.ToLower(strings.TrimSpace(request.Email))

	code, err := s.verificationCodeRepository.FindByEmailAndCode(ctx, email, strings.TrimSpace(request.VerificationCode))
	if err != nil {
		return nil, appErrors.InternalError
	}
	if code == nil {
		return nil, errInvalidVerificationCode
	}

	var user *userEntities.User

	if code.UserID != nil {
		user, err = s.userRepository.FindByID(ctx, *code.UserID)
		if err != nil {
			return nil, appErrors.InternalError
		}
	} else {
		err = s.WithTransaction(ctx, func(ctx context.Context) error {
			created, err := s.userRepository.Create(ctx, &userEntities.User{
				Email:       code.Email,
				Name:        code.Name,
				MobilePhone: code.MobilePhone,
				Document:    code.Document,
				Role:        userEntities.RoleDefault,
			})
			if err != nil {
				return appErrors.InternalError
			}

			if group := strings.TrimSpace(code.Group); group != "" {
				matchedGroup, err := s.groupRepository.FindByNameExact(ctx, group)
				if err != nil {
					return appErrors.InternalError
				}
				if matchedGroup != nil {
					created.GroupID = &matchedGroup.ID
					created, err = s.userRepository.Update(ctx, created)
					if err != nil {
						return appErrors.InternalError
					}
				}
			}

			code.UserID = &created.ID
			if _, err := s.verificationCodeRepository.Update(ctx, code); err != nil {
				return appErrors.InternalError
			}

			user = created
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	token, err := s.jwtService.GenerateIdentityToken(ctx, user)
	if err != nil {
		return nil, appErrors.InternalError
	}

	var group *messages.GroupSummaryDTO
	if user.GroupID != nil {
		userGroup, err := s.groupRepository.FindByID(ctx, *user.GroupID)
		if err != nil {
			return nil, appErrors.InternalError
		}
		group = mappers.MapGroupToSummaryDTO(userGroup)
	}

	response := mappers.MapUserToVerificationCodeResponseDTO(user, nil, token)
	response.Group = group
	return response, nil
}
