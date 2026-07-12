package services

import (
	"testing"
	"time"

	uEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/api/auth"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateIdentityToken(t *testing.T) {
	TestSuite.DefaultSetup(t)

	testCases := []struct {
		name           string
		user           *uEntities.User
		expectedUserID string
	}{
		{
			name: "Newly created user",
			user: &uEntities.User{
				ID:    1,
				Email: "test@example.com",
				Name:  "Test User",
			},
			expectedUserID: "1",
		},
		{
			name: "Large id user",
			user: &uEntities.User{
				ID:    999999,
				Email: "largeid@example.com",
				Name:  "Large ID User",
			},
			expectedUserID: "999999",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			service := newJwtService()

			token, err := service.GenerateIdentityToken(TestSuite.Ctx, testCase.user)
			require.NoError(t, err)
			assert.NotEmpty(t, token)

			parsedToken, err := jwt.ParseWithClaims(token, &auth.IdentityClaims{}, func(token *jwt.Token) (interface{}, error) {
				return []byte("testIdentitySecret"), nil
			})
			require.NoError(t, err)
			assert.True(t, parsedToken.Valid)

			claims, ok := parsedToken.Claims.(*auth.IdentityClaims)
			require.True(t, ok)
			assert.Equal(t, testCase.expectedUserID, claims.UserID)

			require.NotNil(t, claims.ExpiresAt)
			assert.WithinDuration(t, time.Now().Add(time.Hour*24), claims.ExpiresAt.Time, 5*time.Second)
		})
	}
}

func newJwtService() *JwtService {
	return NewJwtService(TestSuite.BaseService).(*JwtService)
}
