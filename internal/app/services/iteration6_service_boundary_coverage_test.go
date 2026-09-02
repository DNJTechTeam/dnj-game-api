package services

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	activityEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/activity/entities"
	favoriteEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/favorite/entities"
	gameEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/game/entities"
	userEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestIteration6_ServiceBoundaryCoverage(t *testing.T) {
	setupIteration6Test(t)
	dbFailure := errors.New("database unavailable")

	t.Run("pure helper validation branches", func(t *testing.T) {
		_, err := managerGameName("")
		apiServiceError(t, err, 400, "INVALID_REQUEST")
		_, err = managerGameName(strings.Repeat("x", 121))
		apiServiceError(t, err, 400, "INVALID_REQUEST")
		scope := "unsupported"
		apiServiceError(t, requireInteractiveManagerScope(&userEntities.User{ManagerScope: &scope}, false), 403, "FORBIDDEN")
	})

	t.Run("participant authorization branches", func(t *testing.T) {
		cases := []struct {
			name string
			user *userEntities.User
			err  error
			code string
		}{
			{name: "missing user", code: "UNAUTHENTICATED"},
			{name: "repository not found", err: appErrors.ErrNotFound, code: "UNAUTHENTICATED"},
			{name: "repository failure", err: dbFailure},
			{name: "onboarding", user: &userEntities.User{OnboardingComplete: false}, code: "ONBOARDING_REQUIRED"},
			{name: "manager role", user: &userEntities.User{OnboardingComplete: true, Role: userEntities.RoleEventManager}, code: "FORBIDDEN"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				users := mocks.NewMockUserRepositoryInterface(t)
				users.On("FindByID", mock.Anything, uint64(42)).Return(tc.user, tc.err).Once()
				service := &GameService{users: users}
				_, err := service.participant(TestSuite.ContextWithUser(42), false)
				if tc.code != "" {
					apiServiceError(t, err, map[string]int{"UNAUTHENTICATED": 401, "ONBOARDING_REQUIRED": 409, "FORBIDDEN": 403}[tc.code], tc.code)
				} else {
					assert.ErrorIs(t, err, appErrors.InternalError)
				}
			})
		}
	})

	t.Run("manager authorization branches", func(t *testing.T) {
		cases := []struct {
			name string
			user *userEntities.User
			err  error
			code string
		}{
			{name: "missing user", code: "UNAUTHENTICATED"},
			{name: "repository not found", err: appErrors.ErrNotFound, code: "UNAUTHENTICATED"},
			{name: "repository failure", err: dbFailure},
			{name: "default role", user: &userEntities.User{OnboardingComplete: true, Role: userEntities.RoleDefault}, code: "FORBIDDEN"},
			{name: "onboarding", user: &userEntities.User{OnboardingComplete: false, Role: userEntities.RoleEventManager}, code: "ONBOARDING_REQUIRED"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				users := mocks.NewMockUserRepositoryInterface(t)
				users.On("FindByID", mock.Anything, uint64(84)).Return(tc.user, tc.err).Once()
				service := &GameService{users: users}
				_, _, err := service.manager(TestSuite.ContextWithUser(84), false)
				if tc.code != "" {
					apiServiceError(t, err, map[string]int{"UNAUTHENTICATED": 401, "ONBOARDING_REQUIRED": 409, "FORBIDDEN": 403}[tc.code], tc.code)
				} else {
					assert.ErrorIs(t, err, appErrors.InternalError)
				}
			})
		}
	})

	t.Run("audit and idempotency failure branches", func(t *testing.T) {
		games := mocks.NewMockGameRepositoryInterface(t)
		audits := mocks.NewMockOperationAuditRepositoryInterface(t)
		service := &GameService{audits: audits}
		audits.On("Create", mock.Anything, mock.Anything).Return(nil, appErrors.ErrConflict).Once()
		apiServiceError(t, service.auditManagerOperation(context.Background(), 84, "key", "operation", "run", map[string]any{"ok": true}, iteration6Now), 409, "IDEMPOTENCY_KEY_REUSED")
		audits.On("Create", mock.Anything, mock.Anything).Return(nil, dbFailure).Once()
		assert.ErrorIs(t, service.auditManagerOperation(context.Background(), 84, "key-2", "operation", "run", map[string]any{"ok": true}, iteration6Now), appErrors.InternalError)

		service.games = games
		games.On("FindManagerOperation", mock.Anything, uint64(84), "prior-error").Return(nil, dbFailure).Once()
		_, err := service.findPriorManagerOperation(context.Background(), 84, "prior-error", "operation", "hash")
		assert.ErrorIs(t, err, appErrors.InternalError)
		games.On("FindManagerOperation", mock.Anything, uint64(84), "cross-conflict").Return(nil, appErrors.ErrNotFound).Once()
		games.On("FindParticipantOperation", mock.Anything, uint64(84), "cross-conflict").Return(&favoriteEntities.ParticipantOperation{}, nil).Once()
		_, err = service.findPriorManagerOperation(context.Background(), 84, "cross-conflict", "operation", "hash")
		apiServiceError(t, err, 409, "IDEMPOTENCY_KEY_REUSED")
		games.On("FindManagerOperation", mock.Anything, uint64(84), "audit-error").Return(nil, appErrors.ErrNotFound).Once()
		games.On("FindParticipantOperation", mock.Anything, uint64(84), "audit-error").Return(nil, appErrors.ErrNotFound).Once()
		audits.On("FindByActorAndIdempotencyKey", mock.Anything, uint64(84), "audit-error").Return(nil, dbFailure).Once()
		_, err = service.findPriorManagerOperation(context.Background(), 84, "audit-error", "operation", "hash")
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("current participation and manager run read failures", func(t *testing.T) {
		games := mocks.NewMockGameRepositoryInterface(t)
		users := mocks.NewMockUserRepositoryInterface(t)
		users.On("FindByID", mock.Anything, uint64(42)).Return(iteration6DefaultUser(), nil).Once()
		games.On("FindCurrentParticipation", mock.Anything, uint64(42)).Return(nil, dbFailure).Once()
		service := &GameService{games: games, users: users}
		_, err := service.CurrentParticipation(TestSuite.ContextWithUser(42))
		assert.ErrorIs(t, err, appErrors.InternalError)

		_, _, _, _, err = service.readManagerRun(context.Background(), "bad", false)
		apiServiceError(t, err, 404, "NOT_FOUND")
		manager := iteration6ManagerUser()
		scope := "unsupported"
		manager.ManagerScope = &scope
		users.On("FindByID", mock.Anything, uint64(84)).Return(manager, nil).Once()
		_, _, _, _, err = service.readManagerRun(TestSuite.ContextWithUser(84), "11111111-1111-4111-8111-111111111111", false)
		apiServiceError(t, err, 403, "FORBIDDEN")

		users.On("FindByID", mock.Anything, uint64(84)).Return(iteration6ManagerUser(), nil).Once()
		games.On("FindRunForManager", mock.Anything, mock.Anything, uint64(84), false, false).Return(&gameEntities.ActivityRun{ID: "run-id"}, nil).Once()
		games.On("ListRunParticipants", mock.Anything, "run-id").Return(nil, dbFailure).Once()
		_, _, _, _, err = service.readManagerRun(TestSuite.ContextWithUser(84), "11111111-1111-4111-8111-111111111111", false)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("manager overview schedule and participants failures", func(t *testing.T) {
		users := mocks.NewMockUserRepositoryInterface(t)
		games := mocks.NewMockGameRepositoryInterface(t)
		activities := mocks.NewMockActivityRepositoryInterface(t)
		users.On("FindByID", mock.Anything, uint64(84)).Return(iteration6ManagerUser(), nil).Times(3)
		games.On("ListManageableGames", mock.Anything, uint64(84), false, iteration6Now).Return(nil, nil).Once()
		activities.On("ListManagerSchedule", mock.Anything, uint64(84), false).Return(nil, dbFailure).Once()
		service := &GameService{games: games, users: users, activities: activities, now: func() time.Time { return iteration6Now }}
		_, err := service.ManagerOverview(TestSuite.ContextWithUser(84))
		assert.ErrorIs(t, err, appErrors.InternalError)

		games.On("ListManageableGames", mock.Anything, uint64(84), false, iteration6Now).Return(nil, nil).Once()
		activities.On("ListManagerSchedule", mock.Anything, uint64(84), false).Return(nil, nil).Once()
		games.On("FindOpenRunForManager", mock.Anything, uint64(84), false).Return(&gameEntities.ActivityRun{ID: "run-id", Status: gameEntities.RunStatusActive}, nil).Once()
		games.On("ListRunParticipants", mock.Anything, "run-id").Return(nil, dbFailure).Once()
		_, err = service.ManagerOverview(TestSuite.ContextWithUser(84))
		assert.ErrorIs(t, err, appErrors.InternalError)

		games.On("ListManageableGames", mock.Anything, uint64(84), false, iteration6Now).Return(nil, nil).Once()
		activities.On("ListManagerSchedule", mock.Anything, uint64(84), false).Return(nil, nil).Once()
		games.On("FindOpenRunForManager", mock.Anything, uint64(84), false).Return(nil, appErrors.ErrNotFound).Once()
		_, err = service.ManagerOverview(TestSuite.ContextWithUser(84))
		assert.NoError(t, err)
	})

	t.Run("write request validation branches", func(t *testing.T) {
		service := NewGameService(TestSuite.BaseService, TestSuite.GameRepository, TestSuite.ActivityRepository, TestSuite.UserRepository, TestSuite.OperationAuditRepository).(*GameService)
		service.now = func() time.Time { return iteration6Now }
		service.secret = func() string { return "secret" }
		managerCtx := TestSuite.ContextWithUser(84)
		validKey := "11111111-1111-4111-8111-111111111111"

		_, _, err := service.CreateManagerGame(managerCtx, validKey, nil)
		apiServiceError(t, err, 400, "INVALID_REQUEST")
		_, _, err = service.CreateManagerGame(managerCtx, validKey, &messages.CreateManagerGameRequestDTO{Name: ""})
		apiServiceError(t, err, 400, "INVALID_REQUEST")
		_, _, err = service.CreateManagerGame(managerCtx, "bad", &messages.CreateManagerGameRequestDTO{Name: "Name"})
		apiServiceError(t, err, 400, "INVALID_REQUEST")

		_, err = service.UpdateManagerGame(managerCtx, "bad", validKey, nil)
		apiServiceError(t, err, 400, "INVALID_REQUEST")
		_, err = service.UpdateManagerGame(managerCtx, "bad", validKey, &messages.UpdateManagerGameRequestDTO{Name: ""})
		apiServiceError(t, err, 400, "INVALID_REQUEST")
		_, err = service.UpdateManagerGame(managerCtx, "bad", validKey, &messages.UpdateManagerGameRequestDTO{Name: "Name"})
		apiServiceError(t, err, 404, "NOT_FOUND")
		_, err = service.UpdateManagerGame(managerCtx, "11111111-1111-4111-8111-111111111111", "bad", &messages.UpdateManagerGameRequestDTO{Name: "Name"})
		apiServiceError(t, err, 400, "INVALID_REQUEST")

		_, _, err = service.ValidateQR(context.Background(), nil)
		apiServiceError(t, err, 400, "INVALID_REQUEST")
		_, _, err = service.ValidateQR(context.Background(), &messages.QRValidateRequestDTO{QRToken: " ", IdempotencyKey: validKey})
		apiServiceError(t, err, 400, "INVALID_REQUEST")
		_, _, err = service.ValidateQR(context.Background(), &messages.QRValidateRequestDTO{QRToken: "token", IdempotencyKey: "bad"})
		apiServiceError(t, err, 400, "INVALID_REQUEST")

		_, err = service.FinalizeRun(context.Background(), "bad", validKey, nil)
		apiServiceError(t, err, 404, "NOT_FOUND")
		_, err = service.FinalizeRun(context.Background(), "11111111-1111-4111-8111-111111111111", "bad", nil)
		apiServiceError(t, err, 400, "INVALID_REQUEST")
		_, err = service.FinalizeRun(context.Background(), "11111111-1111-4111-8111-111111111111", validKey, nil)
		apiServiceError(t, err, 400, "INVALID_REQUEST")
	})

	t.Run("validate QR failure branches", func(t *testing.T) {
		newCase := func(t *testing.T) (*GameService, *mocks.MockGameRepositoryInterface, *mocks.MockActivityRepositoryInterface) {
			t.Helper()
			games := mocks.NewMockGameRepositoryInterface(t)
			users := mocks.NewMockUserRepositoryInterface(t)
			activities := mocks.NewMockActivityRepositoryInterface(t)
			audits := mocks.NewMockOperationAuditRepositoryInterface(t)
			users.On("FindByIDForUpdate", mock.Anything, uint64(42)).Return(iteration6DefaultUser(), nil).Once()
			games.On("FindParticipantOperation", mock.Anything, uint64(42), mock.Anything).Return(nil, appErrors.ErrNotFound).Once()
			games.On("FindManagerOperation", mock.Anything, uint64(42), mock.Anything).Return(nil, appErrors.ErrNotFound).Once()
			audits.On("FindByActorAndIdempotencyKey", mock.Anything, uint64(42), mock.Anything).Return(nil, appErrors.ErrNotFound).Once()
			return &GameService{BaseService: TestSuite.BaseService, games: games, users: users, activities: activities, audits: audits, now: func() time.Time { return iteration6Now }, secret: func() string { return "secret" }}, games, activities
		}
		key := "22222222-2222-4222-8222-222222222222"

		t.Run("participant operation failure", func(t *testing.T) {
			games := mocks.NewMockGameRepositoryInterface(t)
			users := mocks.NewMockUserRepositoryInterface(t)
			audits := mocks.NewMockOperationAuditRepositoryInterface(t)
			users.On("FindByIDForUpdate", mock.Anything, uint64(42)).Return(iteration6DefaultUser(), nil).Once()
			games.On("FindParticipantOperation", mock.Anything, uint64(42), mock.Anything).Return(nil, dbFailure).Once()
			service := &GameService{BaseService: TestSuite.BaseService, games: games, users: users, audits: audits, now: func() time.Time { return iteration6Now }, secret: func() string { return "secret" }}
			_, _, err := service.ValidateQR(TestSuite.ContextWithUser(42), &messages.QRValidateRequestDTO{QRToken: "token", IdempotencyKey: key})
			assert.ErrorIs(t, err, appErrors.InternalError)
		})

		t.Run("QR unavailable", func(t *testing.T) {
			service, games, _ := newCase(t)
			games.On("FindQRByTokenHashForUpdate", mock.Anything, mock.Anything, iteration6Now).Return(nil, appErrors.ErrNotFound).Once()
			_, _, err := service.ValidateQR(TestSuite.ContextWithUser(42), &messages.QRValidateRequestDTO{QRToken: "token", IdempotencyKey: key})
			apiServiceError(t, err, 409, "QR_UNAVAILABLE")
		})

		t.Run("activity lookup failure", func(t *testing.T) {
			service, games, activities := newCase(t)
			games.On("FindQRByTokenHashForUpdate", mock.Anything, mock.Anything, iteration6Now).Return(&gameEntities.QRCode{ActivityID: "activity", ActivityRunID: "run", ExpiresAt: iteration6Now.Add(time.Hour), Status: gameEntities.QRCodeStatusActive}, nil).Once()
			activities.On("FindByID", mock.Anything, "activity").Return(nil, dbFailure).Once()
			_, _, err := service.ValidateQR(TestSuite.ContextWithUser(42), &messages.QRValidateRequestDTO{QRToken: "token", IdempotencyKey: key})
			assert.ErrorIs(t, err, appErrors.InternalError)
		})

		t.Run("existing participation failure", func(t *testing.T) {
			service, games, activities := newCase(t)
			games.On("FindQRByTokenHashForUpdate", mock.Anything, mock.Anything, iteration6Now).Return(&gameEntities.QRCode{ActivityID: "activity", ActivityRunID: "run", ExpiresAt: iteration6Now.Add(time.Hour), Status: gameEntities.QRCodeStatusActive}, nil).Once()
			activities.On("FindByID", mock.Anything, "activity").Return(&activityEntities.Activity{ID: "activity", Kind: activityEntities.KindCompetitive}, nil).Once()
			games.On("FindParticipationByRunAndUser", mock.Anything, "run", uint64(42)).Return(nil, dbFailure).Once()
			_, _, err := service.ValidateQR(TestSuite.ContextWithUser(42), &messages.QRValidateRequestDTO{QRToken: "token", IdempotencyKey: key})
			assert.ErrorIs(t, err, appErrors.InternalError)
		})

		t.Run("participation create failure", func(t *testing.T) {
			service, games, activities := newCase(t)
			games.On("FindQRByTokenHashForUpdate", mock.Anything, mock.Anything, iteration6Now).Return(&gameEntities.QRCode{ActivityID: "activity", ActivityRunID: "run", ExpiresAt: iteration6Now.Add(time.Hour), Status: gameEntities.QRCodeStatusActive}, nil).Once()
			activities.On("FindByID", mock.Anything, "activity").Return(&activityEntities.Activity{ID: "activity", Kind: activityEntities.KindCompetitive}, nil).Once()
			games.On("FindParticipationByRunAndUser", mock.Anything, "run", uint64(42)).Return(nil, appErrors.ErrNotFound).Once()
			games.On("CreateParticipation", mock.Anything, mock.Anything, mock.Anything).Return(dbFailure).Once()
			_, _, err := service.ValidateQR(TestSuite.ContextWithUser(42), &messages.QRValidateRequestDTO{QRToken: "token", IdempotencyKey: key})
			assert.ErrorIs(t, err, appErrors.InternalError)
		})

		t.Run("participation reread failure", func(t *testing.T) {
			service, games, activities := newCase(t)
			games.On("FindQRByTokenHashForUpdate", mock.Anything, mock.Anything, iteration6Now).Return(&gameEntities.QRCode{ActivityID: "activity", ActivityRunID: "run", ExpiresAt: iteration6Now.Add(time.Hour), Status: gameEntities.QRCodeStatusActive}, nil).Once()
			activities.On("FindByID", mock.Anything, "activity").Return(&activityEntities.Activity{ID: "activity", Kind: activityEntities.KindCompetitive}, nil).Once()
			games.On("FindParticipationByRunAndUser", mock.Anything, "run", uint64(42)).Return(nil, appErrors.ErrNotFound).Once()
			games.On("CreateParticipation", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
			games.On("FindParticipationByID", mock.Anything, mock.Anything).Return(nil, dbFailure).Once()
			_, _, err := service.ValidateQR(TestSuite.ContextWithUser(42), &messages.QRValidateRequestDTO{QRToken: "token", IdempotencyKey: key})
			assert.ErrorIs(t, err, appErrors.InternalError)
		})

		t.Run("live award failure", func(t *testing.T) {
			service, games, activities := newCase(t)
			games.On("FindQRByTokenHashForUpdate", mock.Anything, mock.Anything, iteration6Now).Return(&gameEntities.QRCode{ActivityID: "activity", ActivityRunID: "run", ExpiresAt: iteration6Now.Add(time.Hour), Status: gameEntities.QRCodeStatusActive}, nil).Once()
			activities.On("FindByID", mock.Anything, "activity").Return(&activityEntities.Activity{ID: "activity", Kind: activityEntities.KindLive, CheckInPoints: 5}, nil).Once()
			games.On("FindParticipationByRunAndUser", mock.Anything, "run", uint64(42)).Return(nil, appErrors.ErrNotFound).Once()
			games.On("CreateParticipation", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
			games.On("ApplyAward", mock.Anything, mock.Anything, gameEntities.ResultParticipation, 5, mock.Anything).Return(dbFailure).Once()
			_, _, err := service.ValidateQR(TestSuite.ContextWithUser(42), &messages.QRValidateRequestDTO{QRToken: "token", IdempotencyKey: key})
			assert.ErrorIs(t, err, appErrors.InternalError)
		})

		for _, createErr := range []error{appErrors.ErrConflict, dbFailure} {
			t.Run("participant operation persistence", func(t *testing.T) {
				service, games, activities := newCase(t)
				games.On("FindQRByTokenHashForUpdate", mock.Anything, mock.Anything, iteration6Now).Return(&gameEntities.QRCode{ActivityID: "activity", ActivityRunID: "run", ExpiresAt: iteration6Now.Add(time.Hour), Status: gameEntities.QRCodeStatusActive}, nil).Once()
				activities.On("FindByID", mock.Anything, "activity").Return(&activityEntities.Activity{ID: "activity", Kind: activityEntities.KindCompetitive}, nil).Once()
				games.On("FindParticipationByRunAndUser", mock.Anything, "run", uint64(42)).Return(nil, appErrors.ErrNotFound).Once()
				games.On("CreateParticipation", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
				games.On("FindParticipationByID", mock.Anything, mock.Anything).Return(&gameEntities.Participation{ID: "participation", UserID: 42, ActivityID: "activity", ActivityRunID: "run"}, nil).Once()
				games.On("CreateParticipantOperation", mock.Anything, mock.Anything).Return(createErr).Once()
				_, _, err := service.ValidateQR(TestSuite.ContextWithUser(42), &messages.QRValidateRequestDTO{QRToken: "token", IdempotencyKey: key})
				if errors.Is(createErr, appErrors.ErrConflict) {
					apiServiceError(t, err, 409, "IDEMPOTENCY_KEY_REUSED")
				} else {
					assert.ErrorIs(t, err, appErrors.InternalError)
				}
			})
		}
	})
}
