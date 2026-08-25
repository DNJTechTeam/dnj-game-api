package services

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	activityEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/activity/entities"
	gameEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/game/entities"
	gameInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/game/interfaces"
	userEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	userInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/user/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func iteration6ServiceWithMocks(t *testing.T, games gameInterfaces.GameRepositoryInterface, users userInterfaces.UserRepositoryInterface) *GameService {
	t.Helper()
	audits := mocks.NewMockOperationAuditRepositoryInterface(t)
	service := NewGameService(TestSuite.BaseService, games, users, audits).(*GameService)
	service.now = func() time.Time { return iteration6Now }
	service.secret = func() string { return "iteration-6-error-secret" }
	return service
}

func iteration6DefaultUser() *userEntities.User {
	return &userEntities.User{ID: 42, Name: "Participant", Role: userEntities.RoleDefault, OnboardingComplete: true}
}

func iteration6ManagerUser() *userEntities.User {
	return &userEntities.User{ID: 84, Name: "Manager", Role: userEntities.RoleEventManager, OnboardingComplete: true}
}

func TestIteration6_ServiceMapsRepositoryFailures(t *testing.T) {
	setupIteration6Test(t)
	dbFailure := errors.New("database unavailable")

	t.Run("catalog and rankings", func(t *testing.T) {
		games := mocks.NewMockGameRepositoryInterface(t)
		users := mocks.NewMockUserRepositoryInterface(t)
		service := iteration6ServiceWithMocks(t, games, users)
		games.On("ListPublicGames", mock.Anything, iteration6Now, uint64(0)).Return(nil, dbFailure).Once()
		games.On("FindPublicGame", mock.Anything, mock.Anything, iteration6Now).Return(nil, dbFailure).Once()
		games.On("ListIndividualRankings", mock.Anything, uint64(0)).Return(nil, dbFailure).Once()
		games.On("ListGroupRankings", mock.Anything, uint64(0)).Return(nil, dbFailure).Once()

		_, listErr := service.ListGames(TestSuite.Ctx, &messages.ListGamesFilterDTO{})
		_, getErr := service.GetGame(TestSuite.Ctx, "11111111-1111-4111-8111-111111111111")
		_, individualErr := service.Rankings(TestSuite.Ctx, "individual", 0)
		_, groupErr := service.Rankings(TestSuite.Ctx, "groups", 0)
		for _, err := range []error{listErr, getErr, individualErr, groupErr} {
			assert.ErrorIs(t, err, appErrors.InternalError)
		}
	})

	for _, testCase := range []struct {
		name  string
		setup func(*mocks.MockGameRepositoryInterface)
	}{
		{"individual ranking", func(games *mocks.MockGameRepositoryInterface) {
			games.On("TopIndividualRankings", mock.Anything, 30).Return(nil, dbFailure)
		}},
		{"group ranking", func(games *mocks.MockGameRepositoryInterface) {
			games.On("TopIndividualRankings", mock.Anything, 30).Return(nil, nil)
			games.On("TopGroupRankings", mock.Anything, 10).Return(nil, dbFailure)
		}},
		{"point entries", func(games *mocks.MockGameRepositoryInterface) {
			games.On("TopIndividualRankings", mock.Anything, 30).Return(nil, nil)
			games.On("TopGroupRankings", mock.Anything, 10).Return(nil, nil)
			games.On("ListPointEntries", mock.Anything, uint64(42), 50).Return(nil, dbFailure)
		}},
		{"current rank", func(games *mocks.MockGameRepositoryInterface) {
			games.On("TopIndividualRankings", mock.Anything, 30).Return(nil, nil)
			games.On("TopGroupRankings", mock.Anything, 10).Return(nil, nil)
			games.On("ListPointEntries", mock.Anything, uint64(42), 50).Return(nil, nil)
			games.On("FindCurrentRanking", mock.Anything, uint64(42)).Return(nil, nil, dbFailure)
		}},
	} {
		t.Run("overview "+testCase.name, func(t *testing.T) {
			games := mocks.NewMockGameRepositoryInterface(t)
			users := mocks.NewMockUserRepositoryInterface(t)
			users.On("FindByID", mock.Anything, uint64(42)).Return(iteration6DefaultUser(), nil)
			testCase.setup(games)
			_, err := iteration6ServiceWithMocks(t, games, users).Overview(TestSuite.ContextWithUser(42))
			assert.ErrorIs(t, err, appErrors.InternalError)
		})
	}

	for _, testCase := range []struct {
		name  string
		setup func(*mocks.MockGameRepositoryInterface)
	}{
		{"catalog", func(games *mocks.MockGameRepositoryInterface) {
			games.On("ListManageableGames", mock.Anything, uint64(84), false, iteration6Now).Return(nil, dbFailure)
		}},
		{"open run", func(games *mocks.MockGameRepositoryInterface) {
			games.On("ListManageableGames", mock.Anything, uint64(84), false, iteration6Now).Return(nil, nil)
			games.On("FindOpenRunForManager", mock.Anything, uint64(84), false).Return(nil, dbFailure)
		}},
	} {
		t.Run("manager overview "+testCase.name, func(t *testing.T) {
			games := mocks.NewMockGameRepositoryInterface(t)
			users := mocks.NewMockUserRepositoryInterface(t)
			users.On("FindByID", mock.Anything, uint64(84)).Return(iteration6ManagerUser(), nil)
			testCase.setup(games)
			_, err := iteration6ServiceWithMocks(t, games, users).ManagerOverview(TestSuite.ContextWithUser(84))
			assert.ErrorIs(t, err, appErrors.InternalError)
		})
	}

	t.Run("manager run missing and repository failure", func(t *testing.T) {
		games := mocks.NewMockGameRepositoryInterface(t)
		users := mocks.NewMockUserRepositoryInterface(t)
		users.On("FindByID", mock.Anything, uint64(84)).Return(iteration6ManagerUser(), nil).Twice()
		games.On("FindRunForManager", mock.Anything, mock.Anything, uint64(84), false, false).Return(nil, appErrors.ErrNotFound).Once()
		games.On("FindRunForManager", mock.Anything, mock.Anything, uint64(84), false, false).Return(nil, dbFailure).Once()
		service := iteration6ServiceWithMocks(t, games, users)
		_, missingErr := service.ManagerRun(TestSuite.ContextWithUser(84), "11111111-1111-4111-8111-111111111111")
		_, failureErr := service.ManagerRun(TestSuite.ContextWithUser(84), "22222222-2222-4222-8222-222222222222")
		apiServiceError(t, missingErr, 404, "NOT_FOUND")
		assert.ErrorIs(t, failureErr, appErrors.InternalError)
	})
}

func TestIteration6_WriteServicesRollbackAdapterFailures(t *testing.T) {
	setupIteration6Test(t)
	dbFailure := errors.New("database unavailable")
	runID := "11111111-1111-4111-8111-111111111111"
	activityID := "22222222-2222-4222-8222-222222222222"
	participantID := "33333333-3333-4333-8333-333333333333"
	participationID := "44444444-4444-4444-8444-444444444444"
	ctx := TestSuite.ContextWithUser(84)

	newWriteService := func(t *testing.T) (*GameService, *mocks.MockGameRepositoryInterface, *mocks.MockUserRepositoryInterface, *mocks.MockOperationAuditRepositoryInterface) {
		games := mocks.NewMockGameRepositoryInterface(t)
		users := mocks.NewMockUserRepositoryInterface(t)
		audits := mocks.NewMockOperationAuditRepositoryInterface(t)
		service := &GameService{BaseService: TestSuite.BaseService, games: games, users: users, audits: audits, now: func() time.Time { return iteration6Now }, secret: func() string { return "secret" }}
		users.On("FindByIDForUpdate", mock.Anything, uint64(84)).Return(iteration6ManagerUser(), nil).Once()
		return service, games, users, audits
	}

	expectNoPrior := func(games *mocks.MockGameRepositoryInterface, audits *mocks.MockOperationAuditRepositoryInterface, key string) {
		games.On("FindManagerOperation", mock.Anything, uint64(84), key).Return(nil, appErrors.ErrNotFound).Once()
		games.On("FindParticipantOperation", mock.Anything, uint64(84), key).Return(nil, appErrors.ErrNotFound).Once()
		audits.On("FindByActorAndIdempotencyKey", mock.Anything, uint64(84), key).Return(nil, appErrors.ErrNotFound).Once()
	}

	t.Run("QR rotation", func(t *testing.T) {
		service, games, _, audits := newWriteService(t)
		key := "55555555-5555-4555-8555-555555555555"
		run := &gameEntities.ActivityRun{ID: runID, ActivityID: activityID, Status: gameEntities.RunStatusDraft}
		games.On("FindRunForManager", mock.Anything, runID, uint64(84), false, true).Return(run, nil).Once()
		expectNoPrior(games, audits, key)
		games.On("DisableActiveQR", mock.Anything, runID, iteration6Now).Return(dbFailure).Once()

		_, _, err := service.RotateQR(ctx, runID, key)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("state transition", func(t *testing.T) {
		service, games, _, audits := newWriteService(t)
		key := "66666666-6666-4666-8666-666666666666"
		run := &gameEntities.ActivityRun{ID: runID, ActivityID: activityID, Status: gameEntities.RunStatusActive}
		games.On("FindRunForManager", mock.Anything, runID, uint64(84), false, true).Return(run, nil).Once()
		expectNoPrior(games, audits, key)
		games.On("TransitionRun", mock.Anything, runID, gameEntities.RunStatusActive, gameEntities.RunStatusPaused, mock.Anything, mock.Anything, iteration6Now).Return(dbFailure).Once()

		_, err := service.PauseRun(ctx, runID, key)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("result user lock", func(t *testing.T) {
		service, games, _, audits := newWriteService(t)
		key := "77777777-7777-4777-8777-777777777777"
		run := &gameEntities.ActivityRun{ID: runID, ActivityID: activityID, Status: gameEntities.RunStatusActive, PointRules: gameEntities.DefaultPointRules()}
		participant := gameEntities.RunParticipant{ID: participantID, ActivityRunID: runID, UserID: 42, ParticipationID: participationID}
		games.On("FindRunForManager", mock.Anything, runID, uint64(84), false, true).Return(run, nil).Once()
		expectNoPrior(games, audits, key)
		games.On("TransitionRun", mock.Anything, runID, gameEntities.RunStatusActive, gameEntities.RunStatusResults, mock.Anything, mock.Anything, iteration6Now).Return(nil).Once()
		games.On("ListRunParticipants", mock.Anything, runID).Return([]gameEntities.RunParticipant{participant}, nil).Once()
		games.On("LockUsers", mock.Anything, []uint64{42}).Return(dbFailure).Once()

		_, err := service.FinalizeRun(ctx, runID, key, &messages.FinalizeRunResultsRequestDTO{Results: []messages.RunResultRequestDTO{{ParticipantID: participantID, Result: "first"}}})
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("transition conflict", func(t *testing.T) {
		service, games, _, audits := newWriteService(t)
		key := "88888888-8888-4888-8888-888888888888"
		run := &gameEntities.ActivityRun{ID: runID, ActivityID: activityID, Status: gameEntities.RunStatusActive}
		games.On("FindRunForManager", mock.Anything, runID, uint64(84), false, true).Return(run, nil).Once()
		expectNoPrior(games, audits, key)
		games.On("TransitionRun", mock.Anything, runID, gameEntities.RunStatusActive, gameEntities.RunStatusPaused, mock.Anything, mock.Anything, iteration6Now).Return(appErrors.ErrConflict).Once()

		_, err := service.PauseRun(ctx, runID, key)
		apiServiceError(t, err, 409, "RUN_STATE_CONFLICT")
	})

	t.Run("result transition conflict", func(t *testing.T) {
		service, games, _, audits := newWriteService(t)
		key := "99999999-9999-4999-8999-999999999999"
		run := &gameEntities.ActivityRun{ID: runID, ActivityID: activityID, Status: gameEntities.RunStatusPaused, PointRules: gameEntities.DefaultPointRules()}
		games.On("FindRunForManager", mock.Anything, runID, uint64(84), false, true).Return(run, nil).Once()
		expectNoPrior(games, audits, key)
		games.On("TransitionRun", mock.Anything, runID, gameEntities.RunStatusPaused, gameEntities.RunStatusResults, mock.Anything, mock.Anything, iteration6Now).Return(appErrors.ErrConflict).Once()

		_, err := service.FinalizeRun(ctx, runID, key, &messages.FinalizeRunResultsRequestDTO{Results: []messages.RunResultRequestDTO{}})
		apiServiceError(t, err, 409, "RUN_STATE_CONFLICT")
	})

	t.Run("result identity mismatch", func(t *testing.T) {
		service, games, _, audits := newWriteService(t)
		key := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
		run := &gameEntities.ActivityRun{ID: runID, ActivityID: activityID, Status: gameEntities.RunStatusResults, PointRules: gameEntities.DefaultPointRules()}
		participant := gameEntities.RunParticipant{ID: participantID, ActivityRunID: runID, UserID: 42, ParticipationID: participationID}
		games.On("FindRunForManager", mock.Anything, runID, uint64(84), false, true).Return(run, nil).Once()
		expectNoPrior(games, audits, key)
		games.On("ListRunParticipants", mock.Anything, runID).Return([]gameEntities.RunParticipant{participant}, nil).Once()

		_, err := service.FinalizeRun(ctx, runID, key, &messages.FinalizeRunResultsRequestDTO{Results: []messages.RunResultRequestDTO{{ParticipantID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", Result: "first"}}})
		apiServiceError(t, err, 400, "INVALID_REQUEST")
	})

	t.Run("QR lookup failure", func(t *testing.T) {
		games := mocks.NewMockGameRepositoryInterface(t)
		users := mocks.NewMockUserRepositoryInterface(t)
		audits := mocks.NewMockOperationAuditRepositoryInterface(t)
		service := &GameService{BaseService: TestSuite.BaseService, games: games, users: users, audits: audits, now: func() time.Time { return iteration6Now }, secret: func() string { return "secret" }}
		users.On("FindByIDForUpdate", mock.Anything, uint64(42)).Return(iteration6DefaultUser(), nil).Once()
		games.On("FindParticipantOperation", mock.Anything, uint64(42), mock.Anything).Return(nil, appErrors.ErrNotFound).Once()
		games.On("FindManagerOperation", mock.Anything, uint64(42), mock.Anything).Return(nil, appErrors.ErrNotFound).Once()
		audits.On("FindByActorAndIdempotencyKey", mock.Anything, uint64(42), mock.Anything).Return(nil, appErrors.ErrNotFound).Once()
		games.On("FindQRByTokenHashForUpdate", mock.Anything, mock.Anything, iteration6Now).Return(nil, dbFailure).Once()

		_, _, err := service.ValidateQR(TestSuite.ContextWithUser(42), &messages.QRValidateRequestDTO{QRToken: "token", IdempotencyKey: "cccccccc-cccc-4ccc-8ccc-cccccccccccc"})
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("QR cross-store idempotency lookup failure", func(t *testing.T) {
		games := mocks.NewMockGameRepositoryInterface(t)
		users := mocks.NewMockUserRepositoryInterface(t)
		audits := mocks.NewMockOperationAuditRepositoryInterface(t)
		service := &GameService{BaseService: TestSuite.BaseService, games: games, users: users, audits: audits, now: func() time.Time { return iteration6Now }, secret: func() string { return "secret" }}
		users.On("FindByIDForUpdate", mock.Anything, uint64(42)).Return(iteration6DefaultUser(), nil).Once()
		games.On("FindParticipantOperation", mock.Anything, uint64(42), mock.Anything).Return(nil, appErrors.ErrNotFound).Once()
		games.On("FindManagerOperation", mock.Anything, uint64(42), mock.Anything).Return(nil, dbFailure).Once()

		_, _, err := service.ValidateQR(TestSuite.ContextWithUser(42), &messages.QRValidateRequestDTO{QRToken: "token", IdempotencyKey: "14141414-1414-4414-8414-141414141414"})
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("manager cross-store idempotency lookup failure", func(t *testing.T) {
		service, games, _, _ := newWriteService(t)
		key := "15151515-1515-4515-8515-151515151515"
		games.On("FindManagerOperation", mock.Anything, uint64(84), key).Return(nil, appErrors.ErrNotFound).Once()
		games.On("FindParticipantOperation", mock.Anything, uint64(84), key).Return(nil, dbFailure).Once()

		_, _, err := service.CreateRun(ctx, key, &messages.CreateRunRequestDTO{GameID: activityID})
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	createRequest := &messages.CreateRunRequestDTO{GameID: activityID}
	activity := &activityEntities.Activity{ID: activityID, Kind: activityEntities.KindCompetitive, Status: activityEntities.StatusActive}

	t.Run("run activity lookup", func(t *testing.T) {
		service, games, _, audits := newWriteService(t)
		key := "dddddddd-dddd-4ddd-8ddd-dddddddddddd"
		expectNoPrior(games, audits, key)
		games.On("FindManageableActivityForUpdate", mock.Anything, activityID, uint64(84), false, iteration6Now).Return(nil, dbFailure).Once()

		_, _, err := service.CreateRun(ctx, key, createRequest)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("run open lookup", func(t *testing.T) {
		service, games, _, audits := newWriteService(t)
		key := "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee"
		expectNoPrior(games, audits, key)
		games.On("FindManageableActivityForUpdate", mock.Anything, activityID, uint64(84), false, iteration6Now).Return(activity, nil).Once()
		games.On("FindOpenRunByActivityForUpdate", mock.Anything, activityID).Return(nil, dbFailure).Once()

		_, _, err := service.CreateRun(ctx, key, createRequest)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("run insert conflict", func(t *testing.T) {
		service, games, _, audits := newWriteService(t)
		key := "ffffffff-ffff-4fff-8fff-ffffffffffff"
		expectNoPrior(games, audits, key)
		games.On("FindManageableActivityForUpdate", mock.Anything, activityID, uint64(84), false, iteration6Now).Return(activity, nil).Once()
		games.On("FindOpenRunByActivityForUpdate", mock.Anything, activityID).Return(nil, appErrors.ErrNotFound).Once()
		games.On("CreateRun", mock.Anything, mock.Anything).Return(nil, appErrors.ErrConflict).Once()

		_, _, err := service.CreateRun(ctx, key, createRequest)
		apiServiceError(t, err, 409, "RUN_STATE_CONFLICT")
	})

	t.Run("run idempotency persistence", func(t *testing.T) {
		service, games, _, audits := newWriteService(t)
		key := "12121212-1212-4212-8212-121212121212"
		expectNoPrior(games, audits, key)
		games.On("FindManageableActivityForUpdate", mock.Anything, activityID, uint64(84), false, iteration6Now).Return(activity, nil).Once()
		games.On("FindOpenRunByActivityForUpdate", mock.Anything, activityID).Return(nil, appErrors.ErrNotFound).Once()
		games.On("CreateRun", mock.Anything, mock.Anything).Return(&gameEntities.ActivityRun{ID: runID, ActivityID: activityID, Status: gameEntities.RunStatusDraft}, nil).Once()
		games.On("CreateManagerOperation", mock.Anything, mock.Anything).Return(dbFailure).Once()

		_, _, err := service.CreateRun(ctx, key, createRequest)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("manager repository", func(t *testing.T) {
		games := mocks.NewMockGameRepositoryInterface(t)
		users := mocks.NewMockUserRepositoryInterface(t)
		users.On("FindByIDForUpdate", mock.Anything, uint64(84)).Return(nil, dbFailure).Once()
		service := iteration6ServiceWithMocks(t, games, users)

		_, _, err := service.CreateRun(ctx, "13131313-1313-4313-8313-131313131313", createRequest)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})
}

func TestIteration6_QRRequiresConfiguredSecret(t *testing.T) {
	service := &GameService{secret: func() string { return " " }}
	_, _, validateErr := service.ValidateQR(context.Background(), &messages.QRValidateRequestDTO{QRToken: "opaque", IdempotencyKey: "11111111-1111-4111-8111-111111111111"})
	_, _, rotateErr := service.RotateQR(context.Background(), "22222222-2222-4222-8222-222222222222", "33333333-3333-4333-8333-333333333333")
	assert.ErrorIs(t, validateErr, appErrors.InternalError)
	assert.ErrorIs(t, rotateErr, appErrors.InternalError)
}

func TestIteration6_IdentityRepositoryFailuresAndPureFallbacks(t *testing.T) {
	setupIteration6Test(t)
	dbFailure := errors.New("database unavailable")

	t.Run("participant repository failure", func(t *testing.T) {
		games := mocks.NewMockGameRepositoryInterface(t)
		users := mocks.NewMockUserRepositoryInterface(t)
		users.On("FindByID", mock.Anything, uint64(42)).Return(nil, dbFailure).Once()
		_, err := iteration6ServiceWithMocks(t, games, users).CurrentParticipation(TestSuite.ContextWithUser(42))
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("current run repository failure", func(t *testing.T) {
		games := mocks.NewMockGameRepositoryInterface(t)
		users := mocks.NewMockUserRepositoryInterface(t)
		users.On("FindByID", mock.Anything, uint64(42)).Return(iteration6DefaultUser(), nil).Once()
		games.On("FindRunForParticipant", mock.Anything, uint64(42), (*string)(nil)).Return(nil, nil, dbFailure).Once()
		_, err := iteration6ServiceWithMocks(t, games, users).CurrentRun(TestSuite.ContextWithUser(42), "")
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("missing participant object", func(t *testing.T) {
		games := mocks.NewMockGameRepositoryInterface(t)
		users := mocks.NewMockUserRepositoryInterface(t)
		users.On("FindByID", mock.Anything, uint64(42)).Return(nil, nil).Once()
		_, err := iteration6ServiceWithMocks(t, games, users).CurrentParticipation(TestSuite.ContextWithUser(42))
		apiServiceError(t, err, 401, "UNAUTHENTICATED")
	})

	t.Run("missing manager object", func(t *testing.T) {
		games := mocks.NewMockGameRepositoryInterface(t)
		users := mocks.NewMockUserRepositoryInterface(t)
		users.On("FindByID", mock.Anything, uint64(84)).Return(nil, nil).Once()
		_, err := iteration6ServiceWithMocks(t, games, users).ManagerOverview(TestSuite.ContextWithUser(84))
		apiServiceError(t, err, 401, "UNAUTHENTICATED")
	})

	t.Setenv("DOCUMENT_HMAC_SECRET", "configured-secret")
	games := mocks.NewMockGameRepositoryInterface(t)
	users := mocks.NewMockUserRepositoryInterface(t)
	audits := mocks.NewMockOperationAuditRepositoryInterface(t)
	service := NewGameService(TestSuite.BaseService, games, users, audits).(*GameService)
	assert.Equal(t, os.Getenv("DOCUMENT_HMAC_SECRET"), service.secret())
	assert.Equal(t, "fallback", valueOr(nil, "fallback"))
	assert.Equal(t, "paused", dashboardStatus(gameEntities.RunStatusPaused))
}
