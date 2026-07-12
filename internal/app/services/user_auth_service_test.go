package services

import (
	"context"
	"strconv"
	"testing"

	appErrors "github.com/scienceandcode/nucleus-api/internal/app/errors"
	"github.com/scienceandcode/nucleus-api/internal/app/interfaces"
	"github.com/scienceandcode/nucleus-api/internal/app/messages"
	"github.com/scienceandcode/nucleus-api/internal/domain/user/entities"
	"github.com/scienceandcode/nucleus-api/internal/infrastructure/common"
	"github.com/scienceandcode/nucleus-api/internal/infrastructure/db/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

const (
	testUserEmail    = "test@example.com"
	testUserPassword = "Password@#1493"
)

// newUserAuthService wires the service with the real (testcontainers-backed)
// repositories and services.
func newUserAuthService() interfaces.UserAuthServiceInterface {
	return NewUserAuthService(
		TestSuite.BaseService,
		TestSuite.UserRepository,
		NewJwtService(TestSuite.BaseService),
		newTestEmailService(),
		NewEmailConfirmationService(TestSuite.BaseService),
	)
}

func validRegisterRequest() *messages.RegisterRequestDTO {
	return &messages.RegisterRequestDTO{
		Email:       testUserEmail,
		Name:        "Test User",
		Password:    testUserPassword,
		MobilePhone: "41992395568",
	}
}

func TestRegister(t *testing.T) {
	TestSuite.DefaultSetup(t)

	testCases := []struct {
		name          string
		request       *messages.RegisterRequestDTO
		seeder        func(t *testing.T)
		expectedError error
	}{
		{
			name:    "All fields are valid",
			request: validRegisterRequest(),
		},
		{
			name:          "No data provided",
			request:       nil,
			expectedError: appErrors.NewError("Erro ao registrar-se. Verifique os campos e tente novamente.", []*appErrors.FieldError{appErrors.NewFieldError("*", "Nenhuma informação foi fornecida.")}),
		},
		{
			name: "Invalid email",
			request: func() *messages.RegisterRequestDTO {
				r := validRegisterRequest()
				r.Email = "invalid-email"
				return r
			}(),
			expectedError: appErrors.NewError("Erro ao registrar-se. Verifique os campos e tente novamente.", []*appErrors.FieldError{appErrors.NewFieldError("email", "O email informado não é válido.")}),
		},
		{
			name: "Email already in use",
			seeder: func(t *testing.T) {
				_, err := createTestUser(t)
				require.NoError(t, err)
			},
			request:       validRegisterRequest(),
			expectedError: appErrors.NewError("Erro ao registrar-se. Verifique os campos e tente novamente.", []*appErrors.FieldError{appErrors.NewFieldError("email", "O email informado já está sendo utilizado.")}),
		},
		{
			name: "Name too short",
			request: func() *messages.RegisterRequestDTO {
				r := validRegisterRequest()
				r.Name = "Ab"
				return r
			}(),
			expectedError: appErrors.NewError("Erro ao registrar-se. Verifique os campos e tente novamente.", []*appErrors.FieldError{appErrors.NewFieldError("name", "O nome é obrigatório e deve conter pelo menos 3 caracteres.")}),
		},
		{
			name: "Weak password",
			request: func() *messages.RegisterRequestDTO {
				r := validRegisterRequest()
				r.Password = "weak"
				return r
			}(),
			expectedError: appErrors.NewError("Erro ao registrar-se. Verifique os campos e tente novamente.", []*appErrors.FieldError{appErrors.NewFieldError("password", "A senha deve conter pelo menos 8 caracteres, um maiúsculo, um minúsculo, um número e um caractere especial.")}),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			TestSuite.TruncateTable(t, &models.Task{})
			TestSuite.TruncateTable(t, &models.User{})

			service := newUserAuthService()
			if testCase.seeder != nil {
				testCase.seeder(t)
			}

			response, err := service.Register(TestSuite.Ctx, testCase.request)

			if testCase.expectedError != nil {
				require.Error(t, err)
				require.Nil(t, response)
				assert.Equal(t, testCase.expectedError, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, response)
			assert.NotZero(t, response.ID.Uint64())

			dbUser, err := TestSuite.UserRepository.FindByID(TestSuite.Ctx, response.ID.Uint64())
			require.NoError(t, err)
			require.NotNil(t, dbUser)
			assert.Equal(t, testCase.request.Email, dbUser.Email)
			assert.Nil(t, dbUser.EmailConfirmedAt)
			assert.NotNil(t, dbUser.PasswordConfirmedAt)
			assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(dbUser.Password), []byte(testCase.request.Password)))
		})
	}
}

func TestLogin(t *testing.T) {
	TestSuite.DefaultSetup(t)

	invalidCredentials := appErrors.NewError("Credenciais inválidas.", nil)

	testCases := []struct {
		name          string
		seed          bool
		request       *messages.LoginRequestDTO
		expectedError error
	}{
		{name: "Nil request", request: nil, expectedError: invalidCredentials},
		{name: "Invalid email format", request: &messages.LoginRequestDTO{Email: "bad", Password: testUserPassword}, expectedError: invalidCredentials},
		{name: "Email not found", request: &messages.LoginRequestDTO{Email: "missing@example.com", Password: testUserPassword}, expectedError: invalidCredentials},
		{name: "Wrong password", seed: true, request: &messages.LoginRequestDTO{Email: testUserEmail, Password: "Wrong@#123"}, expectedError: invalidCredentials},
		{name: "Valid login", seed: true, request: &messages.LoginRequestDTO{Email: testUserEmail, Password: testUserPassword}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			TestSuite.TruncateTable(t, &models.Task{})
			TestSuite.TruncateTable(t, &models.User{})

			service := newUserAuthService()
			if testCase.seed {
				_, err := createTestUser(t)
				require.NoError(t, err)
			}

			response, token, err := service.Login(TestSuite.Ctx, testCase.request)

			if testCase.expectedError != nil {
				require.Error(t, err)
				require.Nil(t, response)
				assert.Equal(t, testCase.expectedError, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, response)
			assert.NotEmpty(t, token)
			assert.Equal(t, testUserEmail, response.Email)
			assert.True(t, response.HasUpdatedPassword)
		})
	}
}

func TestConfirmEmail(t *testing.T) {
	TestSuite.DefaultSetup(t)

	t.Run("Success - valid token", func(t *testing.T) {
		TestSuite.TruncateTable(t, &models.Task{})
		TestSuite.TruncateTable(t, &models.User{})

		service := newUserAuthService()
		user, err := createTestUser(t)
		require.NoError(t, err)

		token, err := NewEmailConfirmationService(TestSuite.BaseService).
			GenerateConfirmationToken(TestSuite.Ctx, user.ID, user.Email)
		require.NoError(t, err)

		ctx := context.WithValue(TestSuite.Ctx, common.UserIDContextKey, strconv.FormatUint(user.ID, 10))
		response, err := service.ConfirmEmail(ctx, token)
		require.NoError(t, err)
		require.NotNil(t, response)

		dbUser, err := TestSuite.UserRepository.FindByID(TestSuite.Ctx, user.ID)
		require.NoError(t, err)
		assert.NotNil(t, dbUser.EmailConfirmedAt)
	})

	t.Run("Invalid token", func(t *testing.T) {
		TestSuite.TruncateTable(t, &models.Task{})
		TestSuite.TruncateTable(t, &models.User{})

		service := newUserAuthService()
		user, err := createTestUser(t)
		require.NoError(t, err)

		ctx := context.WithValue(TestSuite.Ctx, common.UserIDContextKey, strconv.FormatUint(user.ID, 10))
		response, err := service.ConfirmEmail(ctx, "malformed-token")
		require.Error(t, err)
		require.Nil(t, response)
	})
}

func TestUpdatePassword(t *testing.T) {
	TestSuite.DefaultSetup(t)

	t.Run("Success after password reset", func(t *testing.T) {
		TestSuite.TruncateTable(t, &models.Task{})
		TestSuite.TruncateTable(t, &models.User{})

		service := newUserAuthService()

		// A reset user has PasswordConfirmedAt = nil.
		created, err := TestSuite.UserRepository.Create(TestSuite.Ctx, &entities.User{
			Email:       "reset@example.com",
			Name:        "Reset User",
			Password:    common.GenerateRandomPassword(),
			MobilePhone: "41992395568",
		})
		require.NoError(t, err)

		ctx := context.WithValue(TestSuite.Ctx, common.UserIDContextKey, strconv.FormatUint(created.ID, 10))
		response, err := service.UpdatePassword(ctx, &messages.UpdatePasswordRequestDTO{Password: "NewPass@#2024"})
		require.NoError(t, err)
		require.NotNil(t, response)

		dbUser, err := TestSuite.UserRepository.FindByID(TestSuite.Ctx, created.ID)
		require.NoError(t, err)
		assert.NotNil(t, dbUser.PasswordConfirmedAt)
		assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(dbUser.Password), []byte("NewPass@#2024")))
	})
}

func TestForgotPassword(t *testing.T) {
	TestSuite.DefaultSetup(t)

	TestSuite.TruncateTable(t, &models.Task{})
	TestSuite.TruncateTable(t, &models.User{})

	service := newUserAuthService()

	// Unknown email must not error (no account enumeration).
	err := service.ForgotPassword(TestSuite.Ctx, &messages.ForgotPasswordRequestDTO{Email: "unknown@example.com"})
	require.NoError(t, err)

	user, err := createTestUser(t)
	require.NoError(t, err)
	originalPassword := user.Password

	err = service.ForgotPassword(TestSuite.Ctx, &messages.ForgotPasswordRequestDTO{Email: user.Email})
	require.NoError(t, err)

	dbUser, err := TestSuite.UserRepository.FindByID(TestSuite.Ctx, user.ID)
	require.NoError(t, err)
	assert.Nil(t, dbUser.PasswordConfirmedAt)
	assert.NotEqual(t, originalPassword, dbUser.Password)
}

// createTestUser registers the default user through the service so it goes
// through the normal hashing and confirmation flow.
func createTestUser(t *testing.T) (*entities.User, error) {
	t.Helper()
	service := newUserAuthService()
	response, err := service.Register(TestSuite.Ctx, validRegisterRequest())
	if err != nil {
		return nil, err
	}
	return TestSuite.UserRepository.FindByID(TestSuite.Ctx, response.ID.Uint64())
}
