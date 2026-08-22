package services

import (
	"context"
	"net/mail"
	"strings"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/app/mappers"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	groupInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/group/interfaces"
	svcInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/subscriptionwebhookverificationcode/interfaces"
	userEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	userInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/user/interfaces"
)

// errLookupFailed is the single generic error returned when the informed
// document isn't found, so the endpoint can't be used to enumerate which
// documents are registered.
var errLookupFailed = appErrors.NewError("Ocorreu um erro ao tentar localizar o jovem na base.", []*appErrors.FieldError{
	appErrors.NewFieldError("document", "documento não localizado"),
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
	userRepository             userInterfaces.UserRepositoryInterface
	groupRepository            groupInterfaces.GroupRepositoryInterface
	jwtService                 interfaces.JwtServiceInterface
	emailService               interfaces.EmailServiceInterface
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
		BaseService:                baseService,
		verificationCodeRepository: verificationCodeRepository,
		userRepository:             userRepository,
		groupRepository:            groupRepository,
		jwtService:                 jwtService,
		emailService:               emailService,
	}
}

// Onboarding looks up the pending verification code created from a
// subscription webhook by document (CPF) and (re)sends the code by email.
// Calling it again after the user already verified is allowed — it works as
// a "resend code to log in again" flow, reusing the same code since there is
// no TTL/rotation yet.
//
// Some records arrive from the order webhook with no email on file (a
// companion registered under the buyer's shared email — see
// OrderPayloadTranslator). When that happens, Onboarding returns
// EMAIL_REQUIRED without sending anything; the caller is expected to call
// this same endpoint again with the email filled in, which backfills the
// record and sends the code.
func (s *AuthService) Onboarding(ctx context.Context, request *messages.OnboardingRequestDTO) (*messages.OnboardingResponseDTO, error) {
	document := strings.TrimSpace(request.Document)
	if document == "" {
		return nil, appErrors.NewError("Documento é obrigatório.", []*appErrors.FieldError{
			appErrors.NewFieldError("document", "documento não informado"),
		})
	}

	code, err := s.verificationCodeRepository.FindByDocument(ctx, document)
	if err != nil {
		return nil, appErrors.InternalError
	}
	if code == nil {
		return nil, errLookupFailed
	}

	email := strings.TrimSpace(code.Email)
	if email == "" {
		requestEmail := strings.ToLower(strings.TrimSpace(request.Email))
		if requestEmail == "" {
			return &messages.OnboardingResponseDTO{Status: messages.OnboardingStatusEmailRequired}, nil
		}
		if _, err := mail.ParseAddress(requestEmail); err != nil {
			return nil, appErrors.NewError("E-mail inválido.", []*appErrors.FieldError{
				appErrors.NewFieldError("email", "e-mail em formato inválido"),
			})
		}

		code.Email = requestEmail
		code, err = s.verificationCodeRepository.Update(ctx, code)
		if err != nil {
			return nil, appErrors.InternalError
		}
		email = requestEmail
	}

	if err := s.emailService.SendVerificationCodeEmail(ctx, email, code.VerificationCode); err != nil {
		return nil, appErrors.NewError("Erro ao enviar email de verificação. Tente novamente.", nil)
	}

	return &messages.OnboardingResponseDTO{
		Status: messages.OnboardingStatusCodeSent,
		Email:  obfuscateEmail(email),
	}, nil
}

// obfuscateEmail masks the local-part of an email for display, keeping the
// first and last character (or just the first, when the local-part has 2
// characters or fewer) and the full domain.
func obfuscateEmail(email string) string {
	at := strings.LastIndex(email, "@")
	if at <= 0 {
		return email
	}

	local := email[:at]
	domain := email[at:]

	if len(local) <= 2 {
		return string(local[0]) + "***" + domain
	}

	return string(local[0]) + "***" + string(local[len(local)-1]) + domain
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
