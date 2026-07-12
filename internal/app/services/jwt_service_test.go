package services

import (
	"testing"
	"time"

	uEntities "github.com/scienceandcode/nucleus-api/internal/domain/user/entities"
	"github.com/scienceandcode/nucleus-api/internal/infrastructure/api/auth"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateIdentityToken(t *testing.T) {
	TestSuite.DefaultSetup(t)

	testCases := []struct {
		name                       string
		user                       *uEntities.User
		expectedUserID             string
		expectedEmailConfirmed     bool
		expectedHasUpdatedPassword bool
	}{
		{
			name: "Email not confirmed, password not updated",
			user: &uEntities.User{
				ID:    1,
				Email: "test@example.com",
				Name:  "Test User",
			},
			expectedUserID:             "1",
			expectedEmailConfirmed:     false,
			expectedHasUpdatedPassword: false,
		},
		{
			name: "Email confirmed, password updated",
			user: &uEntities.User{
				ID:                  999999,
				Email:               "largeid@example.com",
				Name:                "Large ID User",
				EmailConfirmedAt:    timePtr(time.Now()),
				PasswordConfirmedAt: timePtr(time.Now()),
			},
			expectedUserID:             "999999",
			expectedEmailConfirmed:     true,
			expectedHasUpdatedPassword: true,
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
			assert.Equal(t, testCase.expectedEmailConfirmed, claims.EmailConfirmed)
			assert.Equal(t, testCase.expectedHasUpdatedPassword, claims.HasUpdatedPassword)

			require.NotNil(t, claims.ExpiresAt)
			assert.WithinDuration(t, time.Now().Add(time.Hour*24), claims.ExpiresAt.Time, 5*time.Second)
		})
	}
}

func newJwtService() *JwtService {
	return NewJwtService(TestSuite.BaseService).(*JwtService)
}

func timePtr(t time.Time) *time.Time {
	return &t
}
