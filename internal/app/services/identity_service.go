package services

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	appInterfaces "github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/app/mappers"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	groupEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/group/entities"
	groupInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/group/interfaces"
	identityEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/identity/entities"
	identityInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/identity/interfaces"
	refreshEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/refreshsession/entities"
	refreshInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/refreshsession/interfaces"
	userEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	userInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/user/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/common"
)

const (
	AccessTokenTTL  = 15 * time.Minute
	RefreshTokenTTL = 30 * 24 * time.Hour
)

type IdentityService struct {
	*BaseService
	users           userInterfaces.UserRepositoryInterface
	groups          groupInterfaces.GroupRepositoryInterface
	identities      identityInterfaces.GoogleIdentityRepositoryInterface
	refreshSessions refreshInterfaces.RefreshSessionRepositoryInterface
	jwt             appInterfaces.JwtServiceInterface
	google          appInterfaces.GoogleIDTokenVerifierInterface
	now             func() time.Time
}

func NewIdentityService(
	base *BaseService,
	users userInterfaces.UserRepositoryInterface,
	groups groupInterfaces.GroupRepositoryInterface,
	identities identityInterfaces.GoogleIdentityRepositoryInterface,
	refreshSessions refreshInterfaces.RefreshSessionRepositoryInterface,
	jwt appInterfaces.JwtServiceInterface,
	google appInterfaces.GoogleIDTokenVerifierInterface,
) appInterfaces.IdentityServiceInterface {
	return &IdentityService{BaseService: base, users: users, groups: groups, identities: identities, refreshSessions: refreshSessions, jwt: jwt, google: google, now: time.Now}
}

func identityError(status int, code, message string) error {
	return appErrors.NewAPIServiceError(status, code, message, nil)
}

func randomOpaqueToken(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (s *IdentityService) userResponse(ctx context.Context, user *userEntities.User) (*messages.IdentityUserResponseDTO, error) {
	var group *groupEntities.Group
	var err error
	if user.GroupID != nil {
		group, err = s.groups.FindByID(ctx, *user.GroupID)
		if err != nil {
			return nil, appErrors.InternalError
		}
	}
	legacy := mappers.MapUserToResponseDTO(user, group)
	return &messages.IdentityUserResponseDTO{
		ID: legacy.ID, Email: legacy.Email, Name: legacy.Name, MobilePhone: legacy.MobilePhone,
		DocumentMasked: legacy.DocumentMasked, Role: legacy.Role, Group: legacy.Group,
		OnboardingComplete: user.OnboardingComplete,
	}, nil
}

func (s *IdentityService) issueSession(ctx context.Context, user *userEntities.User, familyID string) (*messages.IdentitySessionResponseDTO, error) {
	accessToken, err := s.jwt.GenerateIdentityToken(ctx, user)
	if err != nil {
		return nil, appErrors.InternalError
	}
	refreshToken, err := randomOpaqueToken(32)
	if err != nil {
		return nil, appErrors.InternalError
	}
	csrfToken, err := randomOpaqueToken(32)
	if err != nil {
		return nil, appErrors.InternalError
	}
	if familyID == "" {
		familyID, err = randomOpaqueToken(16)
		if err != nil {
			return nil, appErrors.InternalError
		}
	}
	sessionID, err := randomOpaqueToken(16)
	if err != nil {
		return nil, appErrors.InternalError
	}
	now := s.now().UTC()
	if _, err := s.refreshSessions.Create(ctx, &refreshEntities.RefreshSession{
		ID: sessionID, UserID: user.ID, FamilyID: familyID, TokenHash: tokenHash(refreshToken),
		ExpiresAt: now.Add(RefreshTokenTTL), CreatedAt: now, LastUsedAt: now,
	}); err != nil {
		return nil, appErrors.InternalError
	}
	responseUser, err := s.userResponse(ctx, user)
	if err != nil {
		return nil, err
	}
	return &messages.IdentitySessionResponseDTO{
		AccessToken: accessToken, TokenType: "Bearer", ExpiresIn: int64(AccessTokenTTL.Seconds()),
		RefreshToken: refreshToken, CSRFToken: csrfToken, OnboardingRequired: !user.OnboardingComplete, User: responseUser,
	}, nil
}

func (s *IdentityService) AuthenticateGoogle(ctx context.Context, request *messages.GoogleAuthRequestDTO) (*messages.IdentitySessionResponseDTO, error) {
	audience := strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_ID"))
	if audience == "" {
		return nil, appErrors.InternalError
	}
	payload, err := s.google.Verify(ctx, request.IDToken, audience)
	if err != nil || payload == nil || !payload.EmailVerified {
		return nil, identityError(http.StatusUnauthorized, "INVALID_GOOGLE_TOKEN", "Token Google inválido.")
	}
	email := strings.ToLower(strings.TrimSpace(payload.Email))
	var user *userEntities.User
	err = s.WithTransaction(ctx, func(txCtx context.Context) error {
		linked, err := s.identities.FindByProviderAndSubject(txCtx, identityEntities.ProviderGoogle, payload.Subject)
		if err != nil {
			return err
		}
		if linked != nil {
			if !strings.EqualFold(linked.Email, email) {
				return identityError(http.StatusConflict, "IDENTITY_CONFLICT", "A identidade Google não corresponde à conta vinculada.")
			}
			user, err = s.users.FindByID(txCtx, linked.UserID)
			if err != nil || !strings.EqualFold(user.Email, email) {
				return identityError(http.StatusConflict, "IDENTITY_CONFLICT", "A identidade Google não corresponde à conta vinculada.")
			}
			return nil
		}

		user, err = s.users.FindByEmail(txCtx, email)
		if err != nil {
			return err
		}
		if user == nil {
			name := strings.TrimSpace(payload.Name)
			if name == "" {
				name = strings.Split(email, "@")[0]
			}
			user, err = s.users.Create(txCtx, &userEntities.User{Email: email, Name: name, Role: userEntities.RoleDefault, OnboardingComplete: false})
			if err != nil {
				return err
			}
		}
		_, err = s.identities.Create(txCtx, &identityEntities.GoogleIdentity{
			UserID: user.ID, Provider: identityEntities.ProviderGoogle, Subject: payload.Subject, Email: email,
		})
		return err
	})
	if err != nil {
		if errors.Is(err, appErrors.ErrConflict) {
			return nil, identityError(http.StatusConflict, "IDENTITY_CONFLICT", "Esta conta já possui outro vínculo de identidade.")
		}
		return nil, err
	}
	return s.issueSession(ctx, user, "")
}

func (s *IdentityService) Refresh(ctx context.Context, rawToken string) (*messages.IdentitySessionResponseDTO, error) {
	if strings.TrimSpace(rawToken) == "" {
		return nil, identityError(http.StatusUnauthorized, "INVALID_REFRESH_TOKEN", "Sessão inválida.")
	}
	var user *userEntities.User
	var familyID string
	var rotated *messages.IdentitySessionResponseDTO
	var reuseDetected bool
	var expired bool
	err := s.WithTransaction(ctx, func(txCtx context.Context) error {
		session, err := s.refreshSessions.FindByTokenHashForUpdate(txCtx, tokenHash(rawToken))
		if errors.Is(err, appErrors.ErrNotFound) {
			return identityError(http.StatusUnauthorized, "INVALID_REFRESH_TOKEN", "Sessão inválida.")
		}
		if err != nil {
			return err
		}
		familyID = session.FamilyID
		now := s.now().UTC()
		if session.RevokedAt != nil {
			if err := s.refreshSessions.RevokeFamily(txCtx, familyID, true); err != nil {
				return err
			}
			reuseDetected = true
			return nil
		}
		if !session.ExpiresAt.After(now) {
			session.RevokedAt = &now
			if _, err := s.refreshSessions.Update(txCtx, session); err != nil {
				return err
			}
			expired = true
			return nil
		}
		user, err = s.users.FindByID(txCtx, session.UserID)
		if err != nil {
			return identityError(http.StatusUnauthorized, "INVALID_REFRESH_TOKEN", "Sessão inválida.")
		}
		rotated, err = s.issueSession(txCtx, user, familyID)
		if err != nil {
			return err
		}
		session.RevokedAt = &now
		session.LastUsedAt = now
		session.ReplacedByHash = tokenHash(rotated.RefreshToken)
		_, err = s.refreshSessions.Update(txCtx, session)
		return err
	})
	if err != nil {
		return nil, err
	}
	if reuseDetected {
		return nil, identityError(http.StatusUnauthorized, "REFRESH_TOKEN_REUSE", "Reuso de sessão detectado; a família foi revogada.")
	}
	if expired {
		return nil, identityError(http.StatusUnauthorized, "REFRESH_TOKEN_EXPIRED", "Sessão expirada.")
	}
	return rotated, nil
}

func (s *IdentityService) Current(ctx context.Context) (*messages.CurrentSessionResponseDTO, error) {
	userID := common.ExtractUserIdFromContext(ctx)
	if userID == 0 {
		return nil, identityError(http.StatusUnauthorized, "UNAUTHENTICATED", "Autenticação necessária.")
	}
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return nil, identityError(http.StatusUnauthorized, "UNAUTHENTICATED", "Autenticação necessária.")
	}
	responseUser, err := s.userResponse(ctx, user)
	if err != nil {
		return nil, err
	}
	return &messages.CurrentSessionResponseDTO{OnboardingRequired: !user.OnboardingComplete, User: responseUser}, nil
}

func normalizeDocument(document string) string { return common.SanitizePhone(document) }

func validCPF(document string) bool {
	if len(document) != 11 || strings.Count(document, document[:1]) == 11 {
		return false
	}
	for digit := 9; digit < 11; digit++ {
		sum := 0
		for index := 0; index < digit; index++ {
			sum += int(document[index]-'0') * (digit + 1 - index)
		}
		check := (sum * 10) % 11
		if check == 10 {
			check = 0
		}
		if check != int(document[digit]-'0') {
			return false
		}
	}
	return true
}

func (s *IdentityService) CompleteOnboarding(ctx context.Context, request *messages.CompleteOnboardingRequestDTO) (*messages.CurrentSessionResponseDTO, error) {
	userID := common.ExtractUserIdFromContext(ctx)
	document := normalizeDocument(request.Document)
	phone := common.SanitizePhone(request.MobilePhone)
	groupID := request.GroupID.Uint64()
	if userID == 0 {
		return nil, identityError(http.StatusUnauthorized, "UNAUTHENTICATED", "Autenticação necessária.")
	}
	if !validCPF(document) || !common.ValidatePhone(phone, true) || groupID == 0 {
		return nil, identityError(http.StatusBadRequest, "INVALID_ONBOARDING", "CPF, telefone e grupo válidos são obrigatórios.")
	}
	secret := os.Getenv("DOCUMENT_HMAC_SECRET")
	if secret == "" {
		return nil, appErrors.InternalError
	}
	if !s.groups.ExistsByID(ctx, groupID) {
		return nil, identityError(http.StatusNotFound, "GROUP_NOT_FOUND", "Grupo não encontrado.")
	}
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return nil, identityError(http.StatusUnauthorized, "UNAUTHENTICATED", "Autenticação necessária.")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(document))
	documentHash := hex.EncodeToString(mac.Sum(nil))
	existing, err := s.users.FindByDocumentHash(ctx, documentHash)
	if err != nil {
		return nil, appErrors.InternalError
	}
	if existing != nil && existing.ID != user.ID {
		return nil, identityError(http.StatusConflict, "DOCUMENT_ALREADY_LINKED", "CPF já vinculado a outra conta.")
	}
	user.Document = ""
	user.DocumentHash = documentHash
	user.DocumentLast4 = document[len(document)-4:]
	user.MobilePhone = phone
	user.GroupID = &groupID
	user.OnboardingComplete = true
	if _, err := s.users.Update(ctx, user); errors.Is(err, appErrors.ErrConflict) {
		return nil, identityError(http.StatusConflict, "DOCUMENT_ALREADY_LINKED", "CPF já vinculado a outra conta.")
	} else if err != nil {
		return nil, appErrors.InternalError
	}
	return s.Current(ctx)
}

func (s *IdentityService) Logout(ctx context.Context, rawToken string) error {
	if strings.TrimSpace(rawToken) == "" {
		return nil
	}
	return s.WithTransaction(ctx, func(txCtx context.Context) error {
		session, err := s.refreshSessions.FindByTokenHashForUpdate(txCtx, tokenHash(rawToken))
		if errors.Is(err, appErrors.ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		return s.refreshSessions.RevokeFamily(txCtx, session.FamilyID, false)
	})
}
