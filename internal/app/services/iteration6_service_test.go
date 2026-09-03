package services

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	activityEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/activity/entities"
	gameEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/game/entities"
	userEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var iteration6Now = time.Date(2026, 10, 18, 15, 0, 0, 123_000_000, time.UTC)

func setupIteration6Test(t *testing.T) *GameService {
	t.Helper()
	TestSuite.DefaultSetup(t)
	for _, model := range []interface{ TableName() string }{
		&models.ManagerOperation{}, &models.PointEntry{}, &models.ActivityRunParticipant{}, &models.Participation{}, &models.ActivityRunQRCode{}, &models.ActivityRun{},
		&models.ParticipantOperation{}, &models.UserFavorite{}, &models.OperationAudit{}, &models.ActivityManagerAssignment{}, &models.GroupMembership{}, &models.Activity{}, &models.Space{}, &models.User{}, &models.Group{},
	} {
		TestSuite.TruncateTable(t, model)
	}
	service := NewGameService(TestSuite.BaseService, TestSuite.GameRepository, TestSuite.ActivityRepository, TestSuite.UserRepository, TestSuite.OperationAuditRepository).(*GameService)
	service.now = func() time.Time { return iteration6Now }
	service.secret = func() string { return "iteration-6-qr-secret" }
	return service
}

func seedIteration6User(t *testing.T, name string, role userEntities.UserRole, onboarding bool, points int) (*userEntities.User, context.Context) {
	t.Helper()
	user, err := TestSuite.UserRepository.Create(TestSuite.Ctx, &userEntities.User{Email: fmt.Sprintf("%s-%s@example.com", name, uuid.NewString()), Name: name, MobilePhone: "41999999999", Role: role, OnboardingComplete: onboarding, Points: points})
	require.NoError(t, err)
	return user, TestSuite.ContextWithUser(user.ID)
}

func seedIteration6Game(t *testing.T, name string, status activityEntities.Status, startsAt *time.Time) string {
	t.Helper()
	id := uuid.NewString()
	require.NoError(t, TestSuite.DbConn.Create(&models.Activity{ID: id, Slug: "game-" + id, Name: name, Kind: string(activityEntities.KindCompetitive), Status: string(status), StartsAt: startsAt, CheckInPoints: 0, MomentPoints: 0, CooldownSeconds: 0, AllowsMoment: false, CreatedAt: iteration6Now, UpdatedAt: iteration6Now}).Error)
	return id
}

func assignIteration6Manager(t *testing.T, activityID string, userID uint64) {
	t.Helper()
	require.NoError(t, TestSuite.DbConn.Create(&models.ActivityManagerAssignment{ActivityID: activityID, UserID: userID, CreatedAt: iteration6Now}).Error)
}

func createIteration6Run(t *testing.T, service *GameService, managerCtx context.Context, gameID string) *messages.ManagerRunResponseDTO {
	t.Helper()
	run, status, err := service.CreateRun(managerCtx, uuid.NewString(), &messages.CreateRunRequestDTO{GameID: gameID})
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, status)
	return run
}

func rotateIteration6QR(t *testing.T, service *GameService, managerCtx context.Context, runID string) *messages.QRResponseDTO {
	t.Helper()
	qr, status, err := service.RotateQR(managerCtx, runID, uuid.NewString())
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, status)
	assert.NotEmpty(t, qr.QRToken)
	return qr
}

func validateIteration6QR(t *testing.T, service *GameService, participantCtx context.Context, token string) *messages.ParticipationEnvelopeDTO {
	t.Helper()
	response, status, err := service.ValidateQR(participantCtx, &messages.QRValidateRequestDTO{QRToken: token, IdempotencyKey: uuid.NewString()})
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, status)
	return response
}

func TestIteration6_GameCatalogVisibilityOrderingPaginationAndDetail(t *testing.T) {
	// given
	service := setupIteration6Test(t)
	past := iteration6Now.Add(-time.Hour)
	nullID := seedIteration6Game(t, "Zulu", activityEntities.StatusActive, nil)
	firstID := seedIteration6Game(t, "Alpha", activityEntities.StatusActive, &past)
	seedIteration6Game(t, "Draft", activityEntities.StatusDraft, &past)
	archivedID := seedIteration6Game(t, "Archived", activityEntities.StatusArchived, &past)
	for index := 0; index < 10; index++ {
		seedIteration6Game(t, fmt.Sprintf("Game %02d", index), activityEntities.StatusPaused, &past)
	}

	// when
	first, firstErr := service.ListGames(TestSuite.Ctx, &messages.ListGamesFilterDTO{})
	secondFilter := &messages.ListGamesFilterDTO{}
	secondFilter.SetPage(1)
	second, secondErr := service.ListGames(TestSuite.Ctx, secondFilter)
	detail, detailErr := service.GetGame(TestSuite.Ctx, firstID)
	_, archivedErr := service.GetGame(TestSuite.Ctx, archivedID)
	_, malformedErr := service.GetGame(TestSuite.Ctx, "not-a-uuid")

	// then
	require.NoError(t, firstErr)
	require.NoError(t, secondErr)
	require.NoError(t, detailErr)
	assert.Len(t, first.Data, 10)
	assert.True(t, first.Pagination.HasNextPage)
	assert.Len(t, second.Data, 2)
	assert.Equal(t, firstID, first.Data[0].ID)
	assert.Equal(t, nullID, second.Data[1].ID)
	assert.Equal(t, "Alpha", detail.Name)
	apiServiceError(t, archivedErr, http.StatusNotFound, "NOT_FOUND")
	apiServiceError(t, malformedErr, http.StatusNotFound, "NOT_FOUND")
}

func TestIteration6_LiveQRCodeCreditsPointsWithoutJoiningRun(t *testing.T) {
	service := setupIteration6Test(t)
	manager, managerCtx := seedIteration6User(t, "Special manager", userEntities.RoleEventManager, true, 0)
	participant, participantCtx := seedIteration6User(t, "Scored participant", userEntities.RoleDefault, true, 0)
	activityID := uuid.NewString()
	require.NoError(t, TestSuite.DbConn.Create(&models.Activity{ID: activityID, Slug: "special-" + activityID, Name: "Desafio espacial", Kind: string(activityEntities.KindLive), Status: string(activityEntities.StatusActive), CheckInPoints: 25, CreatedAt: iteration6Now, UpdatedAt: iteration6Now}).Error)
	assignIteration6Manager(t, activityID, manager.ID)
	run := createIteration6Run(t, service, managerCtx, activityID)
	qr := rotateIteration6QR(t, service, managerCtx, run.ID)

	key := uuid.NewString()
	validated, status, err := service.ValidateQR(participantCtx, &messages.QRValidateRequestDTO{QRToken: qr.QRToken, IdempotencyKey: key})
	retry, retryStatus, retryErr := service.ValidateQR(participantCtx, &messages.QRValidateRequestDTO{QRToken: qr.QRToken, IdempotencyKey: key})
	repeated, repeatedStatus, repeatedErr := service.ValidateQR(participantCtx, &messages.QRValidateRequestDTO{QRToken: qr.QRToken, IdempotencyKey: uuid.NewString()})
	currentRun, currentRunErr := service.CurrentRun(participantCtx, "")
	var refreshed models.User
	require.NoError(t, TestSuite.DbConn.Where("id = ?", participant.ID).Take(&refreshed).Error)

	require.NoError(t, err)
	require.NoError(t, retryErr)
	require.NoError(t, repeatedErr)
	require.NoError(t, currentRunErr)
	assert.Equal(t, http.StatusCreated, status)
	assert.Equal(t, http.StatusCreated, retryStatus)
	assert.Equal(t, http.StatusOK, repeatedStatus)
	assert.Equal(t, "scored", validated.Action)
	assert.Equal(t, "scored", retry.Action)
	assert.Equal(t, "joined", repeated.Action)
	assert.Equal(t, string(activityEntities.KindLive), validated.ActivityKind)
	assert.Equal(t, string(activityEntities.KindLive), retry.ActivityKind)
	assert.Equal(t, 25, validated.PointsAwarded)
	assert.Zero(t, repeated.PointsAwarded)
	assert.Equal(t, 25, *validated.Participation.NewTotalPoints)
	assert.Equal(t, 25, *repeated.Participation.NewTotalPoints)
	assert.Nil(t, currentRun)
	assert.Equal(t, 25, refreshed.Points)
}

func TestIteration6_QRSupportsCheckpointAndChallengeButRejectsSchedule(t *testing.T) {
	service := setupIteration6Test(t)
	manager, managerCtx := seedIteration6User(t, "Activity manager", userEntities.RoleEventManager, true, 0)
	participant, participantCtx := seedIteration6User(t, "Activity participant", userEntities.RoleDefault, true, 0)

	checkpointID := uuid.NewString()
	challengeID := uuid.NewString()
	scheduleID := uuid.NewString()
	for _, activity := range []struct {
		id            string
		kind          activityEntities.Kind
		checkInPoints int
	}{
		{id: checkpointID, kind: activityEntities.KindCheckpoint, checkInPoints: 15},
		{id: challengeID, kind: activityEntities.KindChallenge, checkInPoints: 0},
		{id: scheduleID, kind: activityEntities.KindSchedule, checkInPoints: 0},
	} {
		require.NoError(t, TestSuite.DbConn.Create(&models.Activity{ID: activity.id, Slug: activity.id, Name: string(activity.kind), Kind: string(activity.kind), Status: string(activityEntities.StatusActive), CheckInPoints: activity.checkInPoints, MomentPoints: 20, AllowsMoment: activity.kind == activityEntities.KindChallenge, CreatedAt: iteration6Now, UpdatedAt: iteration6Now}).Error)
		assignIteration6Manager(t, activity.id, manager.ID)
	}

	checkpointRun := createIteration6Run(t, service, managerCtx, checkpointID)
	checkpointQR := rotateIteration6QR(t, service, managerCtx, checkpointRun.ID)
	checkpointResult := validateIteration6QR(t, service, participantCtx, checkpointQR.QRToken)
	checkpointRepeat, checkpointRepeatStatus, checkpointRepeatErr := service.ValidateQR(participantCtx, &messages.QRValidateRequestDTO{QRToken: checkpointQR.QRToken, IdempotencyKey: uuid.NewString()})
	assert.Equal(t, "scored", checkpointResult.Action)
	assert.Equal(t, string(activityEntities.KindCheckpoint), checkpointResult.ActivityKind)
	assert.Equal(t, 15, checkpointResult.PointsAwarded)
	require.NoError(t, checkpointRepeatErr)
	assert.Equal(t, http.StatusOK, checkpointRepeatStatus)
	assert.Equal(t, "joined", checkpointRepeat.Action)
	assert.Zero(t, checkpointRepeat.PointsAwarded)
	assert.Equal(t, 15, *checkpointRepeat.Participation.NewTotalPoints)

	challengeRun := createIteration6Run(t, service, managerCtx, challengeID)
	challengeQR := rotateIteration6QR(t, service, managerCtx, challengeRun.ID)
	challengeResult := validateIteration6QR(t, service, participantCtx, challengeQR.QRToken)
	assert.Equal(t, "joined", challengeResult.Action)
	assert.Equal(t, string(activityEntities.KindChallenge), challengeResult.ActivityKind)
	assert.Zero(t, challengeResult.PointsAwarded)
	currentRun, currentRunErr := service.CurrentRun(participantCtx, "")
	require.NoError(t, currentRunErr)
	assert.Nil(t, currentRun)

	_, _, scheduleErr := service.CreateRun(managerCtx, uuid.NewString(), &messages.CreateRunRequestDTO{GameID: scheduleID})
	apiServiceError(t, scheduleErr, http.StatusNotFound, "NOT_FOUND")

	var refreshed models.User
	require.NoError(t, TestSuite.DbConn.Where("id = ?", participant.ID).Take(&refreshed).Error)
	assert.Equal(t, 15, refreshed.Points)
}

func TestIteration6_RankingsAndOverviewUseEligibleCurrentBalances(t *testing.T) {
	// given
	service := setupIteration6Test(t)
	groupA := seedGroup(t, "Alpha Group")
	seedGroup(t, "Empty Group")
	ana, anaCtx := seedIteration6User(t, "Ana", userEntities.RoleDefault, true, 50)
	bia, _ := seedIteration6User(t, "Bia", userEntities.RoleDefault, true, 50)
	seedIteration6User(t, "Incomplete", userEntities.RoleDefault, false, 999)
	seedIteration6User(t, "Manager", userEntities.RoleEventManager, true, 999)
	for _, userID := range []uint64{ana.ID, bia.ID} {
		require.NoError(t, TestSuite.DbConn.Create(&models.GroupMembership{UserID: userID, GroupID: groupA.ID, JoinedAt: iteration6Now, CreatedAt: iteration6Now, UpdatedAt: iteration6Now}).Error)
	}
	gameID := seedIteration6Game(t, "Ranking Game", activityEntities.StatusActive, nil)
	runID := uuid.NewString()
	require.NoError(t, TestSuite.DbConn.Create(&models.ActivityRun{ID: runID, ActivityID: gameID, StartedBy: bia.ID, Status: string(gameEntities.RunStatusCompleted), PointRules: []byte(`{"first":50,"second":30,"third":20,"participation":10}`), EndedAt: timePointer(iteration6Now), CreatedAt: iteration6Now, UpdatedAt: iteration6Now}).Error)
	qrID := uuid.NewString()
	participationID := uuid.NewString()
	require.NoError(t, TestSuite.DbConn.Create(&models.ActivityRunQRCode{ID: qrID, ActivityID: gameID, ActivityRunID: runID, TokenHash: "hash", ExpiresAt: iteration6Now, Status: string(gameEntities.QRCodeStatusDisabled), CreatedAt: iteration6Now, UpdatedAt: iteration6Now}).Error)
	require.NoError(t, TestSuite.DbConn.Create(&models.Participation{ID: participationID, UserID: ana.ID, ActivityID: gameID, ActivityRunID: runID, QRCodeID: qrID, CheckedInAt: iteration6Now, Status: string(gameEntities.ParticipationStatusCompleted), CreatedAt: iteration6Now}).Error)
	require.NoError(t, TestSuite.DbConn.Create(&models.PointEntry{ID: uuid.NewString(), UserID: ana.ID, ActivityID: &gameID, ActivityRunID: &runID, ParticipationID: &participationID, Origin: "activity_run_results", Reason: "activity_run_first", Delta: 50, CreatedAt: iteration6Now.Add(-time.Minute)}).Error)

	// when
	individual, individualErr := service.Rankings(TestSuite.Ctx, "individual", 0)
	groups, groupsErr := service.Rankings(TestSuite.Ctx, "groups", 0)
	overview, overviewErr := service.Overview(anaCtx)
	_, invalidErr := service.Rankings(TestSuite.Ctx, "", 0)

	// then
	require.NoError(t, individualErr)
	require.NoError(t, groupsErr)
	require.NoError(t, overviewErr)
	individualData := individual.Data.([]messages.IndividualRankingResponseDTO)
	groupData := groups.Data.([]messages.GroupRankingResponseDTO)
	assert.Len(t, individualData, 2)
	assert.Equal(t, "Ana", individualData[0].Name)
	assert.Equal(t, uint64(1), individualData[0].Position)
	assert.Equal(t, 100, groupData[0].Points)
	assert.Equal(t, "Empty Group", groupData[1].Name)
	assert.Zero(t, groupData[1].Points)
	assert.Equal(t, uint64(1), overview.Current.RankPosition)
	assert.Equal(t, 50, overview.Current.Points)
	assert.Equal(t, uint64(1), *overview.Current.GroupRankPosition)
	assert.Equal(t, "1º lugar em Ranking Game", overview.PointEntries[0].Label)
	assert.Equal(t, "trophy", overview.PointEntries[0].Icon)
	apiServiceError(t, invalidErr, http.StatusBadRequest, "INVALID_REQUEST")
}

func TestIteration6_ManagerAuthorizationCreationIdempotencyAndOpenRunConflict(t *testing.T) {
	// given
	service := setupIteration6Test(t)
	manager, managerCtx := seedIteration6User(t, "Manager", userEntities.RoleEventManager, true, 0)
	outsider, outsiderCtx := seedIteration6User(t, "Outsider", userEntities.RoleEventManager, true, 0)
	_, participantCtx := seedIteration6User(t, "Participant", userEntities.RoleDefault, true, 0)
	gameID := seedIteration6Game(t, "Authorized Game", activityEntities.StatusActive, nil)
	assignIteration6Manager(t, gameID, manager.ID)
	key := uuid.NewString()

	// when
	created, status, createErr := service.CreateRun(managerCtx, key, &messages.CreateRunRequestDTO{GameID: gameID})
	require.NoError(t, createErr)
	startedAfterCreation := iteration6Now.Add(time.Minute)
	require.NoError(t, TestSuite.DbConn.Model(&models.ActivityRun{}).Where("id = ?", created.ID).Updates(map[string]any{"status": string(gameEntities.RunStatusActive), "started_at": startedAfterCreation}).Error)
	retry, retryStatus, retryErr := service.CreateRun(managerCtx, key, &messages.CreateRunRequestDTO{GameID: gameID})
	_, _, reusedErr := service.CreateRun(managerCtx, key, &messages.CreateRunRequestDTO{GameID: uuid.NewString()})
	_, _, conflictErr := service.CreateRun(managerCtx, uuid.NewString(), &messages.CreateRunRequestDTO{GameID: gameID})
	_, _, outsideErr := service.CreateRun(outsiderCtx, uuid.NewString(), &messages.CreateRunRequestDTO{GameID: gameID})
	_, _, roleErr := service.CreateRun(participantCtx, uuid.NewString(), &messages.CreateRunRequestDTO{GameID: gameID})
	_, _, badKeyErr := service.CreateRun(managerCtx, "bad", &messages.CreateRunRequestDTO{GameID: gameID})
	_, _, badGameErr := service.CreateRun(managerCtx, uuid.NewString(), &messages.CreateRunRequestDTO{GameID: "bad"})
	crossKey := uuid.NewString()
	require.NoError(t, TestSuite.DbConn.Create(&models.ParticipantOperation{ID: uuid.NewString(), ActorUserID: manager.ID, IdempotencyKey: crossKey, Operation: "favorite.put", ActivityID: gameID, IntentHash: "cross-intent", HTTPStatus: http.StatusOK, CreatedAt: iteration6Now}).Error)
	_, _, crossStoreErr := service.CreateRun(managerCtx, crossKey, &messages.CreateRunRequestDTO{GameID: gameID})

	// then
	require.NoError(t, createErr)
	require.NoError(t, retryErr)
	assert.Equal(t, http.StatusCreated, status)
	assert.Equal(t, http.StatusCreated, retryStatus)
	assert.Equal(t, created.ID, retry.ID)
	assert.Equal(t, string(gameEntities.RunStatusDraft), retry.Status)
	assert.Nil(t, retry.StartedAt)
	assert.Nil(t, retry.EndedAt)
	assert.Empty(t, retry.Participants)
	apiServiceError(t, reusedErr, http.StatusConflict, "IDEMPOTENCY_KEY_REUSED")
	apiServiceError(t, conflictErr, http.StatusConflict, "RUN_STATE_CONFLICT")
	apiServiceError(t, outsideErr, http.StatusNotFound, "NOT_FOUND")
	apiServiceError(t, roleErr, http.StatusForbidden, "FORBIDDEN")
	apiServiceError(t, badKeyErr, http.StatusBadRequest, "INVALID_REQUEST")
	apiServiceError(t, badGameErr, http.StatusNotFound, "NOT_FOUND")
	apiServiceError(t, crossStoreErr, http.StatusConflict, "IDEMPOTENCY_KEY_REUSED")
	var audits int64
	require.NoError(t, TestSuite.DbConn.Model(&models.OperationAudit{}).Where("actor_user_id = ?", manager.ID).Count(&audits).Error)
	assert.Equal(t, int64(1), audits)
	_ = outsider
	_ = participant
}

func TestIteration6_ManagerGameCreationUpdateAuthorizationAndIdempotency(t *testing.T) {
	// given
	service := setupIteration6Test(t)
	manager, managerCtx := seedIteration6User(t, "Game Manager", userEntities.RoleEventManager, true, 0)
	_, outsiderCtx := seedIteration6User(t, "Other Manager", userEntities.RoleEventManager, true, 0)
	wrongScope, wrongScopeCtx := seedIteration6User(t, "Space Manager", userEntities.RoleEventManager, true, 0)
	specialManager, specialManagerCtx := seedIteration6User(t, "Special Manager", userEntities.RoleEventManager, true, 0)
	require.NoError(t, TestSuite.DbConn.Model(&models.User{}).Where("id = ?", wrongScope.ID).Update("manager_scope", "space").Error)
	require.NoError(t, TestSuite.DbConn.Model(&models.User{}).Where("id = ?", specialManager.ID).Update("manager_scope", "special_events").Error)

	unsupportedID := uuid.NewString()
	require.NoError(t, TestSuite.DbConn.Create(&models.Activity{ID: unsupportedID, Slug: "schedule-" + unsupportedID, Name: "Programação", Kind: string(activityEntities.KindSchedule), Status: string(activityEntities.StatusActive), CreatedAt: iteration6Now, UpdatedAt: iteration6Now}).Error)
	assignIteration6Manager(t, unsupportedID, manager.ID)
	createKey := uuid.NewString()
	updateKey := uuid.NewString()

	// when
	created, status, createErr := service.CreateManagerGame(managerCtx, createKey, &messages.CreateManagerGameRequestDTO{Name: "  Corrida do Saco  "})
	require.NoError(t, createErr)
	require.NotNil(t, created)
	updated, updateErr := service.UpdateManagerGame(managerCtx, created.ID, updateKey, &messages.UpdateManagerGameRequestDTO{Name: "Corrida Atualizada"})
	require.NoError(t, updateErr)
	require.NotNil(t, updated)
	_, _, reusedCreateErr := service.CreateManagerGame(managerCtx, createKey, &messages.CreateManagerGameRequestDTO{Name: "Deve sofrer rollback"})
	_, reusedUpdateErr := service.UpdateManagerGame(managerCtx, created.ID, updateKey, &messages.UpdateManagerGameRequestDTO{Name: "Também deve sofrer rollback"})
	_, outsiderErr := service.UpdateManagerGame(outsiderCtx, created.ID, uuid.NewString(), &messages.UpdateManagerGameRequestDTO{Name: "Invasão"})
	_, unsupportedErr := service.UpdateManagerGame(managerCtx, unsupportedID, uuid.NewString(), &messages.UpdateManagerGameRequestDTO{Name: "Programação alterada"})
	_, _, wrongScopeErr := service.CreateManagerGame(wrongScopeCtx, uuid.NewString(), &messages.CreateManagerGameRequestDTO{Name: "Sem permissão"})
	specialCreated, specialStatus, specialErr := service.CreateManagerGame(specialManagerCtx, uuid.NewString(), &messages.CreateManagerGameRequestDTO{Name: "Caça ao tesouro"})
	require.NoError(t, specialErr)
	require.NotNil(t, specialCreated)
	_, _, nilCreateErr := service.CreateManagerGame(managerCtx, uuid.NewString(), nil)
	_, _, blankCreateErr := service.CreateManagerGame(managerCtx, uuid.NewString(), &messages.CreateManagerGameRequestDTO{Name: "  "})
	_, _, malformedCreateKeyErr := service.CreateManagerGame(managerCtx, "not-a-uuid", &messages.CreateManagerGameRequestDTO{Name: "Nome válido"})
	_, nilUpdateErr := service.UpdateManagerGame(managerCtx, created.ID, uuid.NewString(), nil)
	_, malformedGameErr := service.UpdateManagerGame(managerCtx, "not-a-uuid", uuid.NewString(), &messages.UpdateManagerGameRequestDTO{Name: "Nome válido"})
	_, malformedUpdateKeyErr := service.UpdateManagerGame(managerCtx, created.ID, "not-a-uuid", &messages.UpdateManagerGameRequestDTO{Name: "Nome válido"})

	var persisted, specialPersisted models.Activity
	require.NoError(t, TestSuite.DbConn.Where("id = ?", created.ID).Take(&persisted).Error)
	require.NoError(t, TestSuite.DbConn.Where("id = ?", specialCreated.ID).Take(&specialPersisted).Error)
	var rolledBackCreates int64
	require.NoError(t, TestSuite.DbConn.Model(&models.Activity{}).Where("name = ?", "Deve sofrer rollback").Count(&rolledBackCreates).Error)
	var audits int64
	require.NoError(t, TestSuite.DbConn.Model(&models.OperationAudit{}).Where("actor_user_id = ? AND action IN ?", manager.ID, []string{"manager.game.create", "manager.game.update"}).Count(&audits).Error)

	// then
	assert.Equal(t, http.StatusCreated, status)
	assert.Equal(t, "Corrida do Saco", created.Name)
	assert.Equal(t, 50, created.Points.First)
	assert.Equal(t, created.ID, updated.ID)
	assert.Equal(t, "Corrida Atualizada", updated.Name)
	assert.Equal(t, "Corrida Atualizada", persisted.Name)
	assert.Equal(t, string(activityEntities.KindCompetitive), persisted.Kind)
	assert.Zero(t, rolledBackCreates)
	assert.Equal(t, int64(2), audits)
	apiServiceError(t, reusedCreateErr, http.StatusConflict, "IDEMPOTENCY_KEY_REUSED")
	apiServiceError(t, reusedUpdateErr, http.StatusConflict, "IDEMPOTENCY_KEY_REUSED")
	apiServiceError(t, outsiderErr, http.StatusNotFound, "NOT_FOUND")
	apiServiceError(t, unsupportedErr, http.StatusNotFound, "NOT_FOUND")
	apiServiceError(t, wrongScopeErr, http.StatusForbidden, "FORBIDDEN")
	assert.Equal(t, http.StatusCreated, specialStatus)
	assert.Equal(t, string(activityEntities.KindLive), specialPersisted.Kind)
	apiServiceError(t, nilCreateErr, http.StatusBadRequest, "INVALID_REQUEST")
	apiServiceError(t, blankCreateErr, http.StatusBadRequest, "INVALID_REQUEST")
	apiServiceError(t, malformedCreateKeyErr, http.StatusBadRequest, "INVALID_REQUEST")
	apiServiceError(t, nilUpdateErr, http.StatusBadRequest, "INVALID_REQUEST")
	apiServiceError(t, malformedGameErr, http.StatusNotFound, "NOT_FOUND")
	apiServiceError(t, malformedUpdateKeyErr, http.StatusBadRequest, "INVALID_REQUEST")
}

func TestIteration6_QRRotationValidationRetryAndExpiry(t *testing.T) {
	// given
	service := setupIteration6Test(t)
	manager, managerCtx := seedIteration6User(t, "Manager", userEntities.RoleEventManager, true, 0)
	participant, participantCtx := seedIteration6User(t, "Participant", userEntities.RoleDefault, true, 0)
	gameID := seedIteration6Game(t, "QR Game", activityEntities.StatusActive, nil)
	require.NoError(t, TestSuite.DbConn.Model(&models.Activity{}).Where("id = ?", gameID).Update("allows_moment", true).Error)
	assignIteration6Manager(t, gameID, manager.ID)
	run := createIteration6Run(t, service, managerCtx, gameID)
	firstQR := rotateIteration6QR(t, service, managerCtx, run.ID)
	secondQR := rotateIteration6QR(t, service, managerCtx, run.ID)
	key := uuid.NewString()

	// when
	_, _, rotatedErr := service.ValidateQR(participantCtx, &messages.QRValidateRequestDTO{QRToken: firstQR.QRToken, IdempotencyKey: uuid.NewString()})
	created, createdStatus, createErr := service.ValidateQR(participantCtx, &messages.QRValidateRequestDTO{QRToken: secondQR.QRToken, IdempotencyKey: key})
	require.NoError(t, TestSuite.DbConn.Create(&models.PointEntry{ID: uuid.NewString(), UserID: participant.ID, Origin: "legacy_balance", Reason: "legacy_balance", Delta: 99, CreatedAt: iteration6Now}).Error)
	require.NoError(t, TestSuite.DbConn.Model(&models.User{}).Where("id = ?", participant.ID).Update("points", 99).Error)
	retry, retryStatus, retryErr := service.ValidateQR(participantCtx, &messages.QRValidateRequestDTO{QRToken: secondQR.QRToken, IdempotencyKey: key})
	repeated, repeatedStatus, repeatedErr := service.ValidateQR(participantCtx, &messages.QRValidateRequestDTO{QRToken: secondQR.QRToken, IdempotencyKey: uuid.NewString()})
	_, _, reusedErr := service.ValidateQR(participantCtx, &messages.QRValidateRequestDTO{QRToken: "different", IdempotencyKey: key})
	crossKey := uuid.NewString()
	require.NoError(t, TestSuite.DbConn.Create(&models.ManagerOperation{ID: uuid.NewString(), ActorUserID: participant.ID, IdempotencyKey: crossKey, Operation: "manager.activity-run.start", ActivityID: gameID, ActivityRunID: &run.ID, IntentHash: "cross-intent", HTTPStatus: http.StatusOK, CreatedAt: iteration6Now}).Error)
	_, _, crossStoreErr := service.ValidateQR(participantCtx, &messages.QRValidateRequestDTO{QRToken: secondQR.QRToken, IdempotencyKey: crossKey})
	service.now = func() time.Time { return secondQR.ExpiresAt }
	_, _, expiredErr := service.ValidateQR(TestSuite.ContextWithUser(seedExtraParticipant(t).ID), &messages.QRValidateRequestDTO{QRToken: secondQR.QRToken, IdempotencyKey: uuid.NewString()})

	// then
	apiServiceError(t, rotatedErr, http.StatusConflict, "QR_UNAVAILABLE")
	require.NoError(t, createErr)
	require.NoError(t, retryErr)
	require.NoError(t, repeatedErr)
	assert.Equal(t, http.StatusCreated, createdStatus)
	assert.Equal(t, http.StatusCreated, retryStatus)
	assert.Equal(t, http.StatusOK, repeatedStatus)
	assert.Equal(t, created.Participation.ID, retry.Participation.ID)
	assert.Equal(t, created.Participation.ID, repeated.Participation.ID)
	assert.Zero(t, created.Participation.CheckInPoints)
	assert.True(t, created.Participation.CanShareMoment)
	assert.Equal(t, 0, *created.Participation.NewTotalPoints)
	assert.Equal(t, 0, *retry.Participation.NewTotalPoints)
	assert.Equal(t, 99, *repeated.Participation.NewTotalPoints)
	apiServiceError(t, reusedErr, http.StatusConflict, "IDEMPOTENCY_KEY_REUSED")
	apiServiceError(t, crossStoreErr, http.StatusConflict, "IDEMPOTENCY_KEY_REUSED")
	apiServiceError(t, expiredErr, http.StatusGone, "QR_EXPIRED")
	var entries int64
	require.NoError(t, TestSuite.DbConn.Model(&models.PointEntry{}).Where("user_id = ? AND origin = ?", participant.ID, "activity_run_results").Count(&entries).Error)
	assert.Zero(t, entries)
}

func seedExtraParticipant(t *testing.T) *userEntities.User {
	t.Helper()
	user, _ := seedIteration6User(t, "Extra", userEntities.RoleDefault, true, 0)
	return user
}

func TestIteration6_RunTransitionsPreserveStartedAtAndCancelWithoutPoints(t *testing.T) {
	// given
	service := setupIteration6Test(t)
	manager, managerCtx := seedIteration6User(t, "Manager", userEntities.RoleEventManager, true, 0)
	gameID := seedIteration6Game(t, "State Game", activityEntities.StatusActive, nil)
	assignIteration6Manager(t, gameID, manager.ID)
	run := createIteration6Run(t, service, managerCtx, gameID)

	// when
	_, invalidPauseErr := service.PauseRun(managerCtx, run.ID, uuid.NewString())
	started, startErr := service.StartRun(managerCtx, run.ID, uuid.NewString())
	_, invalidStartErr := service.StartRun(managerCtx, run.ID, uuid.NewString())
	paused, pauseErr := service.PauseRun(managerCtx, run.ID, uuid.NewString())
	service.now = func() time.Time { return iteration6Now.Add(5 * time.Minute) }
	resumed, resumeErr := service.ResumeRun(managerCtx, run.ID, uuid.NewString())
	cancelled, cancelErr := service.CancelRun(managerCtx, run.ID, uuid.NewString())
	_, terminalErr := service.ResumeRun(managerCtx, run.ID, uuid.NewString())

	// then
	apiServiceError(t, invalidPauseErr, http.StatusConflict, "RUN_STATE_CONFLICT")
	require.NoError(t, startErr)
	require.NoError(t, pauseErr)
	require.NoError(t, resumeErr)
	require.NoError(t, cancelErr)
	apiServiceError(t, invalidStartErr, http.StatusConflict, "RUN_STATE_CONFLICT")
	apiServiceError(t, terminalErr, http.StatusConflict, "RUN_STATE_CONFLICT")
	assert.Equal(t, "active", started.Status)
	assert.Equal(t, "paused", paused.Status)
	assert.Equal(t, started.StartedAt, resumed.StartedAt)
	assert.Equal(t, "cancelled", cancelled.Status)
	assert.NotNil(t, cancelled.EndedAt)
	var entries int64
	require.NoError(t, TestSuite.DbConn.Model(&models.PointEntry{}).Count(&entries).Error)
	assert.Zero(t, entries)
}

func TestIteration6_FinalizeResultsAwardsExactlyOnceAndRollsBackInvalidSets(t *testing.T) {
	// given
	service := setupIteration6Test(t)
	manager, managerCtx := seedIteration6User(t, "Manager", userEntities.RoleEventManager, true, 0)
	firstUser, firstCtx := seedIteration6User(t, "First", userEntities.RoleDefault, true, 0)
	secondUser, secondCtx := seedIteration6User(t, "Second", userEntities.RoleDefault, true, 0)
	gameID := seedIteration6Game(t, "Results Game", activityEntities.StatusActive, nil)
	assignIteration6Manager(t, gameID, manager.ID)
	run := createIteration6Run(t, service, managerCtx, gameID)
	qr := rotateIteration6QR(t, service, managerCtx, run.ID)
	firstParticipation := validateIteration6QR(t, service, firstCtx, qr.QRToken)
	secondParticipation := validateIteration6QR(t, service, secondCtx, qr.QRToken)
	_, err := service.StartRun(managerCtx, run.ID, uuid.NewString())
	require.NoError(t, err)
	participants, err := TestSuite.GameRepository.ListRunParticipants(TestSuite.Ctx, run.ID)
	require.NoError(t, err)
	sort.Slice(participants, func(i, j int) bool { return participants[i].UserID < participants[j].UserID })
	invalid := &messages.FinalizeRunResultsRequestDTO{Results: []messages.RunResultRequestDTO{{ParticipantID: participants[0].ID, Result: "first"}}}
	valid := &messages.FinalizeRunResultsRequestDTO{Results: []messages.RunResultRequestDTO{{ParticipantID: participants[0].ID, Result: "first"}, {ParticipantID: participants[1].ID, Result: "participation"}}}
	key := uuid.NewString()

	// when
	_, invalidErr := service.FinalizeRun(managerCtx, run.ID, uuid.NewString(), invalid)
	completed, completeErr := service.FinalizeRun(managerCtx, run.ID, key, valid)
	retry, retryErr := service.FinalizeRun(managerCtx, run.ID, key, valid)
	terminal, terminalErr := service.FinalizeRun(managerCtx, run.ID, uuid.NewString(), valid)
	firstAfter, _ := TestSuite.UserRepository.FindByID(TestSuite.Ctx, firstUser.ID)
	secondAfter, _ := TestSuite.UserRepository.FindByID(TestSuite.Ctx, secondUser.ID)
	defaultCurrent, defaultCurrentErr := service.CurrentRun(firstCtx, "")
	explicitCurrent, explicitCurrentErr := service.CurrentRun(firstCtx, run.ID)
	currentParticipation, currentParticipationErr := service.CurrentParticipation(firstCtx)

	// then
	apiServiceError(t, invalidErr, http.StatusBadRequest, "INVALID_REQUEST")
	require.NoError(t, completeErr)
	require.NoError(t, retryErr)
	require.NoError(t, terminalErr)
	require.NoError(t, defaultCurrentErr)
	require.NoError(t, explicitCurrentErr)
	require.NoError(t, currentParticipationErr)
	assert.Equal(t, "completed", completed.Status)
	assert.Equal(t, completed.Status, retry.Status)
	assert.Equal(t, completed.Status, terminal.Status)
	assert.Equal(t, 50, firstAfter.Points)
	assert.Equal(t, 10, secondAfter.Points)
	assert.Equal(t, firstParticipation.Participation.ID, participants[0].ParticipationID)
	assert.Equal(t, secondParticipation.Participation.ID, participants[1].ParticipationID)
	assert.Nil(t, defaultCurrent)
	require.NotNil(t, explicitCurrent)
	assert.Equal(t, "first", *explicitCurrent.Run.Result)
	assert.Equal(t, 50, *explicitCurrent.Run.Points)
	assert.Nil(t, currentParticipation)
	var entries int64
	var audits int64
	require.NoError(t, TestSuite.DbConn.Model(&models.PointEntry{}).Where("activity_run_id = ?", run.ID).Count(&entries).Error)
	require.NoError(t, TestSuite.DbConn.Model(&models.OperationAudit{}).Where("action = ?", "manager.activity-run.results.finalize").Count(&audits).Error)
	assert.Equal(t, int64(2), entries)
	assert.Equal(t, int64(1), audits)
	var awardNotifications int64
	require.NoError(t, TestSuite.DbConn.Model(&models.Notification{}).
		Where("user_id = ? AND category = ?", firstUser.ID, "points").Count(&awardNotifications).Error)
	assert.Zero(t, awardNotifications)
	mismatches, mismatchErr := TestSuite.GameRepository.ListPointBalanceMismatches(TestSuite.Ctx)
	require.NoError(t, mismatchErr)
	assert.Empty(t, mismatches)
	updateErr := TestSuite.DbConn.Exec("UPDATE point_entries SET delta = delta + 1 WHERE activity_run_id = ?", run.ID).Error
	require.Error(t, updateErr)
	deleteErr := TestSuite.DbConn.Exec("DELETE FROM point_entries WHERE activity_run_id = ?", run.ID).Error
	require.Error(t, deleteErr)
	require.NoError(t, TestSuite.DbConn.Model(&models.User{}).Where("id = ?", firstUser.ID).UpdateColumn("points", 51).Error)
	mismatches, mismatchErr = TestSuite.GameRepository.ListPointBalanceMismatches(TestSuite.Ctx)
	require.NoError(t, mismatchErr)
	require.Len(t, mismatches, 1)
	assert.Equal(t, firstUser.ID, mismatches[0].UserID)
	assert.Equal(t, int64(50), mismatches[0].LedgerPoints)
	assert.Equal(t, int64(51), mismatches[0].MaterializedPoints)
}

func TestIteration6_ConcurrentCreationAndScansRemainSingleEffect(t *testing.T) {
	// given
	service := setupIteration6Test(t)
	manager, managerCtx := seedIteration6User(t, "Manager", userEntities.RoleEventManager, true, 0)
	participant, participantCtx := seedIteration6User(t, "Participant", userEntities.RoleDefault, true, 0)
	gameID := seedIteration6Game(t, "Concurrent Game", activityEntities.StatusActive, nil)
	assignIteration6Manager(t, gameID, manager.ID)
	key := uuid.NewString()
	var wg sync.WaitGroup
	createdIDs := make(chan string, 2)
	errorsCh := make(chan error, 2)

	// when
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			run, _, err := service.CreateRun(managerCtx, key, &messages.CreateRunRequestDTO{GameID: gameID})
			if err == nil {
				createdIDs <- run.ID
			}
			errorsCh <- err
		}()
	}
	wg.Wait()
	close(createdIDs)
	close(errorsCh)
	ids := make([]string, 0, 2)
	for id := range createdIDs {
		ids = append(ids, id)
	}
	for err := range errorsCh {
		require.NoError(t, err)
	}
	require.Len(t, ids, 2)
	qr := rotateIteration6QR(t, service, managerCtx, ids[0])
	participationIDs := make(chan string, 2)
	scanErrors := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, _, err := service.ValidateQR(participantCtx, &messages.QRValidateRequestDTO{QRToken: qr.QRToken, IdempotencyKey: uuid.NewString()})
			if err == nil {
				participationIDs <- result.Participation.ID
			}
			scanErrors <- err
		}()
	}
	wg.Wait()
	close(participationIDs)
	close(scanErrors)
	scanned := make([]string, 0, 2)
	for id := range participationIDs {
		scanned = append(scanned, id)
	}
	for err := range scanErrors {
		require.NoError(t, err)
	}

	// then
	assert.Equal(t, ids[0], ids[1])
	require.Len(t, scanned, 2)
	assert.Equal(t, scanned[0], scanned[1])
	var participations int64
	var points int64
	require.NoError(t, TestSuite.DbConn.Model(&models.Participation{}).Where("user_id = ?", participant.ID).Count(&participations).Error)
	require.NoError(t, TestSuite.DbConn.Model(&models.PointEntry{}).Where("user_id = ?", participant.ID).Count(&points).Error)
	assert.Equal(t, int64(1), participations)
	assert.Zero(t, points)
}

func TestIteration6_CurrentReadsAndManagerDashboardUsePersistedRun(t *testing.T) {
	service := setupIteration6Test(t)
	manager, managerCtx := seedIteration6User(t, "Manager", userEntities.RoleEventManager, true, 0)
	_, participantCtx := seedIteration6User(t, "Participant", userEntities.RoleDefault, true, 0)
	_, alienCtx := seedIteration6User(t, "Alien", userEntities.RoleDefault, true, 0)
	gameID := seedIteration6Game(t, "Dashboard Game", activityEntities.StatusActive, nil)
	assignIteration6Manager(t, gameID, manager.ID)

	empty, err := service.ManagerOverview(managerCtx)
	require.NoError(t, err)
	require.Len(t, empty.Actions.Games, 1)
	assert.Nil(t, empty.Actions.Run)

	run := createIteration6Run(t, service, managerCtx, gameID)
	qr := rotateIteration6QR(t, service, managerCtx, run.ID)
	joined := validateIteration6QR(t, service, participantCtx, qr.QRToken)

	dashboard, err := service.ManagerOverview(managerCtx)
	require.NoError(t, err)
	require.NotNil(t, dashboard.Actions.Run)
	assert.Equal(t, "checkin", dashboard.Actions.Run.Status)
	assert.Equal(t, "Dashboard Game", dashboard.Actions.Run.GameName)
	require.Len(t, dashboard.Actions.Run.Participants, 1)

	managerRun, err := service.ManagerRun(managerCtx, run.ID)
	require.NoError(t, err)
	assert.Equal(t, "Dashboard Game", managerRun.Game.Name)
	require.Len(t, managerRun.Participants, 1)
	assert.Equal(t, "Participant", managerRun.Participants[0].Name)

	currentParticipation, err := service.CurrentParticipation(participantCtx)
	require.NoError(t, err)
	require.NotNil(t, currentParticipation)
	assert.Equal(t, joined.Participation.ID, currentParticipation.Participation.ID)

	current, err := service.CurrentRun(participantCtx, "")
	require.NoError(t, err)
	require.NotNil(t, current)
	assert.Equal(t, run.ID, current.Run.ID)
	explicit, err := service.CurrentRun(participantCtx, run.ID)
	require.NoError(t, err)
	assert.Equal(t, run.ID, explicit.Run.ID)

	missing, err := service.CurrentRun(alienCtx, run.ID)
	require.NoError(t, err)
	assert.Nil(t, missing)
	malformed, err := service.CurrentRun(participantCtx, "not-a-uuid")
	require.NoError(t, err)
	assert.Nil(t, malformed)
	_, err = service.ManagerRun(managerCtx, "not-a-uuid")
	apiServiceError(t, err, http.StatusNotFound, "NOT_FOUND")

	_, err = service.StartRun(managerCtx, run.ID, uuid.NewString())
	require.NoError(t, err)
	active, err := service.ManagerOverview(managerCtx)
	require.NoError(t, err)
	assert.Equal(t, "running", active.Actions.Run.Status)

	require.NoError(t, TestSuite.DbConn.Model(&models.Activity{}).Where("id = ?", gameID).Update("status", string(activityEntities.StatusArchived)).Error)
	history, err := service.ManagerRun(managerCtx, run.ID)
	require.NoError(t, err)
	assert.Equal(t, "Dashboard Game", history.Game.Name)
	archivedOverview, err := service.ManagerOverview(managerCtx)
	require.NoError(t, err)
	assert.Empty(t, archivedOverview.Actions.Games)
	require.NotNil(t, archivedOverview.Actions.Run)
}

func TestIteration6_AuthenticationAndRoleChangesAreRevalidated(t *testing.T) {
	service := setupIteration6Test(t)
	removed, removedCtx := seedIteration6User(t, "Removed", userEntities.RoleDefault, true, 0)
	_, incompleteCtx := seedIteration6User(t, "Incomplete", userEntities.RoleDefault, false, 0)
	changed, changedCtx := seedIteration6User(t, "Changed", userEntities.RoleDefault, true, 0)
	manager, managerCtx := seedIteration6User(t, "Manager", userEntities.RoleEventManager, true, 0)
	_, incompleteManagerCtx := seedIteration6User(t, "Incomplete Manager", userEntities.RoleEventManager, false, 0)
	_, adminCtx := seedIteration6User(t, "Admin", userEntities.RoleAdmin, true, 0)
	gameID := seedIteration6Game(t, "Global Game", activityEntities.StatusActive, nil)

	require.NoError(t, TestSuite.DbConn.Delete(&models.User{}, removed.ID).Error)
	require.NoError(t, TestSuite.DbConn.Model(&models.User{}).Where("id = ?", changed.ID).Update("role", string(userEntities.RoleEventManager)).Error)
	require.NoError(t, TestSuite.DbConn.Model(&models.User{}).Where("id = ?", manager.ID).Update("role", string(userEntities.RoleDefault)).Error)

	_, removedErr := service.Overview(removedCtx)
	_, incompleteErr := service.CurrentParticipation(incompleteCtx)
	_, changedErr := service.CurrentRun(changedCtx, "")
	_, managerErr := service.ManagerOverview(managerCtx)
	_, incompleteManagerErr := service.ManagerOverview(incompleteManagerCtx)
	adminOverview, adminOverviewErr := service.ManagerOverview(adminCtx)
	adminRun, _, adminRunErr := service.CreateRun(adminCtx, uuid.NewString(), &messages.CreateRunRequestDTO{GameID: gameID})

	apiServiceError(t, removedErr, http.StatusUnauthorized, "UNAUTHENTICATED")
	apiServiceError(t, incompleteErr, http.StatusConflict, "ONBOARDING_REQUIRED")
	apiServiceError(t, changedErr, http.StatusForbidden, "FORBIDDEN")
	apiServiceError(t, managerErr, http.StatusForbidden, "FORBIDDEN")
	apiServiceError(t, incompleteManagerErr, http.StatusConflict, "ONBOARDING_REQUIRED")
	require.NoError(t, adminOverviewErr)
	require.NoError(t, adminRunErr)
	require.Len(t, adminOverview.Actions.Games, 1)
	assert.Equal(t, gameID, adminRun.Game.ID)
}

func TestIteration6_StrictValidationAndIdempotentTerminalResults(t *testing.T) {
	service := setupIteration6Test(t)
	manager, managerCtx := seedIteration6User(t, "Manager", userEntities.RoleEventManager, true, 0)
	_, participantCtx := seedIteration6User(t, "Participant", userEntities.RoleDefault, true, 0)
	gameID := seedIteration6Game(t, "Validation Game", activityEntities.StatusActive, nil)
	otherGameID := seedIteration6Game(t, "Other Game", activityEntities.StatusActive, nil)
	assignIteration6Manager(t, gameID, manager.ID)
	assignIteration6Manager(t, otherGameID, manager.ID)

	_, listErr := service.ListGames(TestSuite.Ctx, nil)
	_, scopeErr := service.Rankings(TestSuite.Ctx, "", 0)
	_, _, nilQRErr := service.ValidateQR(participantCtx, nil)
	_, _, blankQRErr := service.ValidateQR(participantCtx, &messages.QRValidateRequestDTO{QRToken: " "})
	_, _, keyQRErr := service.ValidateQR(participantCtx, &messages.QRValidateRequestDTO{QRToken: "token", IdempotencyKey: "bad-key"})
	_, _, unknownQRErr := service.ValidateQR(participantCtx, &messages.QRValidateRequestDTO{QRToken: "unknown", IdempotencyKey: uuid.NewString()})
	_, _, nilRunErr := service.CreateRun(managerCtx, uuid.NewString(), nil)
	_, _, malformedGameErr := service.CreateRun(managerCtx, uuid.NewString(), &messages.CreateRunRequestDTO{GameID: "bad"})
	_, _, malformedRunKeyErr := service.CreateRun(managerCtx, "bad-key", &messages.CreateRunRequestDTO{GameID: gameID})

	createKey := uuid.NewString()
	run, status, err := service.CreateRun(managerCtx, createKey, &messages.CreateRunRequestDTO{GameID: gameID})
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, status)
	_, _, reusedCreateErr := service.CreateRun(managerCtx, createKey, &messages.CreateRunRequestDTO{GameID: otherGameID})
	_, _, malformedRotateErr := service.RotateQR(managerCtx, "bad", uuid.NewString())
	_, _, malformedRotateKeyErr := service.RotateQR(managerCtx, run.ID, "bad-key")

	qrKey := uuid.NewString()
	qr, _, err := service.RotateQR(managerCtx, run.ID, qrKey)
	require.NoError(t, err)
	retryQR, retryStatus, err := service.RotateQR(managerCtx, run.ID, qrKey)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, retryStatus)
	assert.Equal(t, qr.QRToken, retryQR.QRToken)
	validateIteration6QR(t, service, participantCtx, qr.QRToken)

	startKey := uuid.NewString()
	started, err := service.StartRun(managerCtx, run.ID, startKey)
	require.NoError(t, err)
	_, err = service.PauseRun(managerCtx, run.ID, uuid.NewString())
	require.NoError(t, err)
	retriedStart, err := service.StartRun(managerCtx, run.ID, startKey)
	require.NoError(t, err)
	assert.Equal(t, started.Status, retriedStart.Status)
	_, reusedTransitionErr := service.ResumeRun(managerCtx, run.ID, startKey)
	_, malformedTransitionErr := service.StartRun(managerCtx, "bad", uuid.NewString())
	_, malformedTransitionKeyErr := service.StartRun(managerCtx, run.ID, "bad-key")
	_, _, activeQRErr := service.RotateQR(managerCtx, run.ID, uuid.NewString())

	participantID := uuid.NewString()
	_, nilResultsErr := canonicalResults(nil)
	_, malformedParticipantErr := canonicalResults(&messages.FinalizeRunResultsRequestDTO{Results: []messages.RunResultRequestDTO{{ParticipantID: "bad", Result: "first"}}})
	_, duplicateParticipantErr := canonicalResults(&messages.FinalizeRunResultsRequestDTO{Results: []messages.RunResultRequestDTO{{ParticipantID: participantID, Result: "first"}, {ParticipantID: participantID, Result: "second"}}})
	_, invalidResultErr := canonicalResults(&messages.FinalizeRunResultsRequestDTO{Results: []messages.RunResultRequestDTO{{ParticipantID: participantID, Result: "winner"}}})
	_, duplicatePodiumErr := canonicalResults(&messages.FinalizeRunResultsRequestDTO{Results: []messages.RunResultRequestDTO{{ParticipantID: uuid.NewString(), Result: "first"}, {ParticipantID: uuid.NewString(), Result: "first"}}})
	_, malformedFinalizeErr := service.FinalizeRun(managerCtx, "bad", uuid.NewString(), &messages.FinalizeRunResultsRequestDTO{Results: []messages.RunResultRequestDTO{}})
	_, malformedFinalizeKeyErr := service.FinalizeRun(managerCtx, run.ID, "bad-key", &messages.FinalizeRunResultsRequestDTO{Results: []messages.RunResultRequestDTO{}})

	cases := []struct {
		err    error
		status int
		code   string
	}{
		{listErr, 400, "INVALID_REQUEST"}, {scopeErr, 400, "INVALID_REQUEST"},
		{nilQRErr, 400, "INVALID_REQUEST"}, {blankQRErr, 400, "INVALID_REQUEST"}, {keyQRErr, 400, "INVALID_REQUEST"}, {unknownQRErr, 409, "QR_UNAVAILABLE"},
		{nilRunErr, 400, "INVALID_REQUEST"}, {malformedGameErr, 404, "NOT_FOUND"}, {malformedRunKeyErr, 400, "INVALID_REQUEST"}, {reusedCreateErr, 409, "IDEMPOTENCY_KEY_REUSED"},
		{malformedRotateErr, 404, "NOT_FOUND"}, {malformedRotateKeyErr, 400, "INVALID_REQUEST"}, {activeQRErr, 409, "RUN_STATE_CONFLICT"},
		{reusedTransitionErr, 409, "IDEMPOTENCY_KEY_REUSED"}, {malformedTransitionErr, 404, "NOT_FOUND"}, {malformedTransitionKeyErr, 400, "INVALID_REQUEST"},
		{nilResultsErr, 400, "INVALID_REQUEST"}, {malformedParticipantErr, 400, "INVALID_REQUEST"}, {duplicateParticipantErr, 400, "INVALID_REQUEST"}, {invalidResultErr, 400, "INVALID_REQUEST"}, {duplicatePodiumErr, 400, "INVALID_REQUEST"},
		{malformedFinalizeErr, 404, "NOT_FOUND"}, {malformedFinalizeKeyErr, 400, "INVALID_REQUEST"},
	}
	for _, item := range cases {
		apiServiceError(t, item.err, item.status, item.code)
	}
}
