package services

import (
	"context"
	"strconv"
	"time"

	"github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
	uEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/api/auth"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/common"

	"github.com/golang-jwt/jwt/v5"
)

type JwtService struct {
	BaseService *BaseService
}

func NewJwtService(baseService *BaseService) interfaces.JwtServiceInterface {
	return &JwtService{baseService}
}

// GenerateIdentityToken issues an HS256 JWT carrying the user's identity.
// The token is valid for 24 hours and is signed with JWT_IDENTITY_SECRET.
func (s *JwtService) GenerateIdentityToken(ctx context.Context, user *uEntities.User) (string, error) {
	claims := auth.IdentityClaims{
		UserID:             strconv.FormatUint(user.ID, 10),
		HasUpdatedPassword: user.PasswordConfirmedAt != nil,
		EmailConfirmed:     user.EmailConfirmedAt != nil,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 24)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(common.GetEnv("JWT_IDENTITY_SECRET")))
}
