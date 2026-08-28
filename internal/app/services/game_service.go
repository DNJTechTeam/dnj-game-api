package services

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	appInterfaces "github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
	appMappers "github.com/dnjtechteam/dnj-game-api/internal/app/mappers"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	activityEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/activity/entities"
	activityInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/activity/interfaces"
	favoriteEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/favorite/entities"
	gameEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/game/entities"
	gameInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/game/interfaces"
	auditEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/operationaudit/entities"
	auditInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/operationaudit/interfaces"
	userEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	userInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/user/interfaces"
	"github.com/google/uuid"
)

const qrLifetime = 45 * time.Minute

type GameService struct {
	*BaseService
	games      gameInterfaces.GameRepositoryInterface
	activities activityInterfaces.ActivityRepositoryInterface
	users      userInterfaces.UserRepositoryInterface
	audits     auditInterfaces.OperationAuditRepositoryInterface
	now        func() time.Time
	secret     func() string
}

func NewGameService(base *BaseService, games gameInterfaces.GameRepositoryInterface, activities activityInterfaces.ActivityRepositoryInterface, users userInterfaces.UserRepositoryInterface, audits auditInterfaces.OperationAuditRepositoryInterface) appInterfaces.GameServiceInterface {
	return &GameService{BaseService: base, games: games, activities: activities, users: users, audits: audits, now: time.Now, secret: func() string { return os.Getenv("DOCUMENT_HMAC_SECRET") }}
}

func gameError(status int, code, message string) error {
	return appErrors.NewAPIServiceError(status, code, message, nil)
}

func intentHash(operation string, value any) string {
	encoded, _ := json.Marshal(value)
	digest := sha256.Sum256(append([]byte(operation+"\x00"), encoded...))
	return hex.EncodeToString(digest[:])
}

func (s *GameService) qrToken(qrID string) string {
	mac := hmac.New(sha256.New, []byte(s.secret()))
	_, _ = mac.Write([]byte("dnj-v2-activity-run-qr\x00" + qrID))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (s *GameService) qrHash(token string) string {
	mac := hmac.New(sha256.New, []byte(s.secret()))
	_, _ = mac.Write([]byte("dnj-v2-activity-run-qr-hash\x00" + token))
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *GameService) requireQRSecret() error {
	if strings.TrimSpace(s.secret()) == "" {
		return appErrors.InternalError
	}
	return nil
}

func (s *GameService) ListGames(ctx context.Context, filter *messages.ListGamesFilterDTO) (*messages.PaginatedResponse[messages.GameResponseDTO], error) {
	if filter == nil {
		return nil, gameError(http.StatusBadRequest, "INVALID_REQUEST", "Filtros inválidos.")
	}
	generatedAt := s.now().UTC()
	result, err := s.games.ListPublicGames(ctx, generatedAt, filter.GetPage())
	if err != nil {
		return nil, appErrors.InternalError
	}
	data := make([]messages.GameResponseDTO, len(result.Data))
	for index := range result.Data {
		data[index] = appMappers.MapGameToResponseDTO(&result.Data[index], deriveActivityState(&result.Data[index].Activity, generatedAt))
	}
	return &messages.PaginatedResponse[messages.GameResponseDTO]{Data: data, Pagination: result.Pagination}, nil
}

func (s *GameService) GetGame(ctx context.Context, rawID string) (*messages.GameResponseDTO, error) {
	id, err := uuid.Parse(rawID)
	if err != nil {
		return nil, gameError(http.StatusNotFound, "NOT_FOUND", "Jogo não encontrado.")
	}
	generatedAt := s.now().UTC()
	game, err := s.games.FindPublicGame(ctx, id.String(), generatedAt)
	if errors.Is(err, appErrors.ErrNotFound) {
		return nil, gameError(http.StatusNotFound, "NOT_FOUND", "Jogo não encontrado.")
	}
	if err != nil {
		return nil, appErrors.InternalError
	}
	response := appMappers.MapGameToResponseDTO(game, deriveActivityState(&game.Activity, generatedAt))
	return &response, nil
}

func (s *GameService) Rankings(ctx context.Context, scope string, page uint64) (*messages.RankingResponseDTO, error) {
	generatedAt := s.now().UTC()
	switch messages.RankingScope(scope) {
	case messages.RankingScopeIndividual:
		result, err := s.games.ListIndividualRankings(ctx, page)
		if err != nil {
			return nil, appErrors.InternalError
		}
		data := make([]messages.IndividualRankingResponseDTO, len(result.Data))
		for i := range result.Data {
			data[i] = appMappers.MapIndividualRankingToResponseDTO(result.Data[i])
		}
		return &messages.RankingResponseDTO{Data: data, Pagination: result.Pagination, GeneratedAt: generatedAt}, nil
	case messages.RankingScopeGroups:
		result, err := s.games.ListGroupRankings(ctx, page)
		if err != nil {
			return nil, appErrors.InternalError
		}
		data := make([]messages.GroupRankingResponseDTO, len(result.Data))
		for i := range result.Data {
			data[i] = appMappers.MapGroupRankingToResponseDTO(result.Data[i])
		}
		return &messages.RankingResponseDTO{Data: data, Pagination: result.Pagination, GeneratedAt: generatedAt}, nil
	default:
		return nil, gameError(http.StatusBadRequest, "INVALID_REQUEST", "scope deve ser individual ou groups.")
	}
}

func (s *GameService) participant(ctx context.Context, lock bool) (*userEntities.User, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	var user *userEntities.User
	if lock {
		user, err = s.users.FindByIDForUpdate(ctx, userID)
	} else {
		user, err = s.users.FindByID(ctx, userID)
	}
	if errors.Is(err, appErrors.ErrNotFound) || (err == nil && user == nil) {
		return nil, gameError(http.StatusUnauthorized, "UNAUTHENTICATED", "Autenticação necessária.")
	}
	if err != nil {
		return nil, appErrors.InternalError
	}
	if !user.OnboardingComplete {
		return nil, gameError(http.StatusConflict, "ONBOARDING_REQUIRED", "Conclua o onboarding antes de continuar.")
	}
	if user.Role != userEntities.RoleDefault {
		return nil, gameError(http.StatusForbidden, "FORBIDDEN", "Participação não permitida para este usuário.")
	}
	return user, nil
}

func (s *GameService) Overview(ctx context.Context) (*messages.GameOverviewResponseDTO, error) {
	user, err := s.participant(ctx, false)
	if err != nil {
		return nil, err
	}
	individual, err := s.games.TopIndividualRankings(ctx, 30)
	if err != nil {
		return nil, appErrors.InternalError
	}
	groups, err := s.games.TopGroupRankings(ctx, 10)
	if err != nil {
		return nil, appErrors.InternalError
	}
	entries, err := s.games.ListPointEntries(ctx, user.ID, 50)
	if err != nil {
		return nil, appErrors.InternalError
	}
	current, currentGroup, err := s.games.FindCurrentRanking(ctx, user.ID)
	if err != nil {
		return nil, appErrors.InternalError
	}
	response := &messages.GameOverviewResponseDTO{Individual: make([]messages.IndividualRankingResponseDTO, len(individual)), Groups: make([]messages.GroupRankingResponseDTO, len(groups)), PointEntries: make([]messages.PointEntryResponseDTO, len(entries)), Current: messages.GameCurrentResponseDTO{RankPosition: current.Position, Points: current.Points}}
	for i := range individual {
		response.Individual[i] = appMappers.MapIndividualRankingToResponseDTO(individual[i])
	}
	for i := range groups {
		response.Groups[i] = appMappers.MapGroupRankingToResponseDTO(groups[i])
	}
	for i := range entries {
		response.PointEntries[i] = appMappers.MapPointEntryToResponseDTO(entries[i])
	}
	if currentGroup != nil {
		groupID := messages.Uint64StringFromUint64(currentGroup.GroupID)
		position := currentGroup.Position
		response.Current.GroupID = &groupID
		response.Current.GroupRankPosition = &position
	}
	return response, nil
}

func (s *GameService) CurrentRun(ctx context.Context, rawRunID string) (*messages.ParticipantRunEnvelopeDTO, error) {
	user, err := s.participant(ctx, false)
	if err != nil {
		return nil, err
	}
	var runID *string
	if rawRunID != "" {
		parsed, parseErr := uuid.Parse(rawRunID)
		if parseErr != nil {
			return nil, nil
		}
		normalized := parsed.String()
		runID = &normalized
	}
	run, participant, err := s.games.FindRunForParticipant(ctx, user.ID, runID)
	if errors.Is(err, appErrors.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, appErrors.InternalError
	}
	if run.Activity != nil && run.Activity.Kind == activityEntities.KindLive {
		return nil, nil
	}
	response := messages.ParticipantRunResponseDTO{ID: run.ID, Status: string(run.Status), GameName: run.Activity.Name, StartedAt: utcPointer(run.StartedAt), EndedAt: utcPointer(run.EndedAt)}
	if participant.Result != nil {
		result := string(*participant.Result)
		points := participant.PointsAwarded
		response.Result = &result
		response.Points = &points
	}
	return &messages.ParticipantRunEnvelopeDTO{Run: response}, nil
}

func (s *GameService) CurrentParticipation(ctx context.Context) (*messages.ParticipationEnvelopeDTO, error) {
	user, err := s.participant(ctx, false)
	if err != nil {
		return nil, err
	}
	participation, err := s.games.FindCurrentParticipation(ctx, user.ID)
	if errors.Is(err, appErrors.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, appErrors.InternalError
	}
	return &messages.ParticipationEnvelopeDTO{Participation: appMappers.MapParticipationToResponseDTO(participation, nil)}, nil
}

func (s *GameService) ValidateQR(ctx context.Context, request *messages.QRValidateRequestDTO) (*messages.ParticipationEnvelopeDTO, int, error) {
	if request == nil || strings.TrimSpace(request.QRToken) == "" || len(strings.TrimSpace(request.QRToken)) > 512 {
		return nil, 0, gameError(http.StatusBadRequest, "INVALID_REQUEST", "Envie um QR válido.")
	}
	key, err := uuid.Parse(request.IdempotencyKey)
	if err != nil {
		return nil, 0, gameError(http.StatusBadRequest, "INVALID_REQUEST", "idempotencyKey deve ser um UUID válido.")
	}
	if err := s.requireQRSecret(); err != nil {
		return nil, 0, err
	}
	token := strings.TrimSpace(request.QRToken)
	tokenHash := s.qrHash(token)
	operation := "participant.activity-run.join"
	requestHash := intentHash(operation, struct {
		QRHash string `json:"qrHash"`
	}{QRHash: tokenHash})
	var response *messages.ParticipationEnvelopeDTO
	status := http.StatusCreated
	err = s.WithTransaction(ctx, func(txCtx context.Context) error {
		user, authErr := s.participant(txCtx, true)
		if authErr != nil {
			return authErr
		}
		prior, findErr := s.games.FindParticipantOperation(txCtx, user.ID, key.String())
		if findErr == nil {
			if prior.Operation != operation || prior.IntentHash != requestHash || prior.ResultRef == nil || prior.ResultPoints == nil {
				return gameError(http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "idempotencyKey já foi usada em outra intenção.")
			}
			participation, participationErr := s.games.FindParticipationByID(txCtx, *prior.ResultRef)
			if participationErr != nil {
				return appErrors.InternalError
			}
			activity, activityErr := s.activities.FindByID(txCtx, participation.ActivityID)
			if activityErr != nil {
				return appErrors.InternalError
			}
			total := *prior.ResultPoints
			action, pointsAwarded := "joined", 0
			if activity.Kind == activityEntities.KindLive {
				action, pointsAwarded = "scored", activity.CheckInPoints
			}
			response = &messages.ParticipationEnvelopeDTO{Participation: appMappers.MapParticipationToResponseDTO(participation, &total), Action: action, PointsAwarded: pointsAwarded}
			status = prior.HTTPStatus
			return nil
		}
		if !errors.Is(findErr, appErrors.ErrNotFound) {
			return appErrors.InternalError
		}
		if _, managerErr := s.games.FindManagerOperation(txCtx, user.ID, key.String()); managerErr == nil {
			return gameError(http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "idempotencyKey já foi usada em outra intenção.")
		} else if !errors.Is(managerErr, appErrors.ErrNotFound) {
			return appErrors.InternalError
		}
		if _, auditErr := s.audits.FindByActorAndIdempotencyKey(txCtx, user.ID, key.String()); auditErr == nil {
			return gameError(http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "idempotencyKey já foi usada em outra intenção.")
		} else if !errors.Is(auditErr, appErrors.ErrNotFound) {
			return appErrors.InternalError
		}
		now := s.now().UTC()
		qr, qrErr := s.games.FindQRByTokenHashForUpdate(txCtx, tokenHash, now)
		if errors.Is(qrErr, appErrors.ErrNotFound) {
			return gameError(http.StatusConflict, "QR_UNAVAILABLE", "Este QR não está disponível.")
		}
		if qrErr != nil {
			return appErrors.InternalError
		}
		if !now.Before(qr.ExpiresAt.UTC()) {
			return gameError(http.StatusGone, "QR_EXPIRED", "O prazo deste QR terminou.")
		}
		if qr.Status != gameEntities.QRCodeStatusActive {
			return gameError(http.StatusConflict, "QR_UNAVAILABLE", "Este QR não está disponível.")
		}
		activity, activityErr := s.activities.FindByID(txCtx, qr.ActivityID)
		if activityErr != nil {
			return appErrors.InternalError
		}
		scoreOnly := activity.Kind == activityEntities.KindLive
		participation, existingErr := s.games.FindParticipationByRunAndUser(txCtx, qr.ActivityRunID, user.ID)
		if existingErr == nil {
			status = http.StatusOK
		} else if errors.Is(existingErr, appErrors.ErrNotFound) {
			participationID := uuid.NewString()
			points := 0
			if scoreOnly {
				points = activity.CheckInPoints
			}
			participation = &gameEntities.Participation{ID: participationID, UserID: user.ID, ActivityID: qr.ActivityID, ActivityRunID: qr.ActivityRunID, QRCodeID: qr.ID, CheckedInAt: now, Status: gameEntities.ParticipationStatusActive, CanShareMoment: qr.AllowsMoment, CheckInPoints: 0, CreatedAt: now}
			participant := &gameEntities.RunParticipant{ID: uuid.NewString(), ActivityRunID: qr.ActivityRunID, UserID: user.ID, ParticipationID: participationID, CheckedInAt: now, CreatedAt: now}
			if createErr := s.games.CreateParticipation(txCtx, participation, participant); createErr != nil {
				return appErrors.InternalError
			}
			if scoreOnly {
				runRef, participationRef := qr.ActivityRunID, participationID
				entry := &gameEntities.PointEntry{ID: uuid.NewString(), UserID: user.ID, ActivityID: qr.ActivityID, ActivityRunID: &runRef, ParticipationID: &participationRef, Origin: "activity_run_results", Reason: appMappers.PointReason(gameEntities.ResultParticipation), Delta: points, CreatedAt: now}
				if awardErr := s.games.ApplyAward(txCtx, participant.ID, gameEntities.ResultParticipation, points, entry); awardErr != nil {
					return appErrors.InternalError
				}
			}
			participation, existingErr = s.games.FindParticipationByID(txCtx, participationID)
			if existingErr != nil {
				return appErrors.InternalError
			}
		} else {
			return appErrors.InternalError
		}
		resultRef := participation.ID
		total := user.Points
		action := "joined"
		if scoreOnly {
			total += activity.CheckInPoints
			action = "scored"
		}
		if createErr := s.games.CreateParticipantOperation(txCtx, &favoriteEntities.ParticipantOperation{ID: uuid.NewString(), ActorUserID: user.ID, IdempotencyKey: key.String(), Operation: operation, ActivityID: qr.ActivityID, IntentHash: requestHash, HTTPStatus: status, ResultRef: &resultRef, ResultPoints: &total, CreatedAt: now}); createErr != nil {
			if errors.Is(createErr, appErrors.ErrConflict) {
				return gameError(http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "idempotencyKey já foi usada em outra intenção.")
			}
			return appErrors.InternalError
		}
		pointsAwarded := 0
		if scoreOnly {
			pointsAwarded = activity.CheckInPoints
		}
		response = &messages.ParticipationEnvelopeDTO{Participation: appMappers.MapParticipationToResponseDTO(participation, &total), Action: action, PointsAwarded: pointsAwarded}
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	return response, status, nil
}

func (s *GameService) manager(ctx context.Context, lock bool) (*userEntities.User, bool, error) {
	actorID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, false, err
	}
	var actor *userEntities.User
	if lock {
		actor, err = s.users.FindByIDForUpdate(ctx, actorID)
	} else {
		actor, err = s.users.FindByID(ctx, actorID)
	}
	if errors.Is(err, appErrors.ErrNotFound) || (err == nil && actor == nil) {
		return nil, false, gameError(http.StatusUnauthorized, "UNAUTHENTICATED", "Autenticação necessária.")
	}
	if err != nil {
		return nil, false, appErrors.InternalError
	}
	if actor.Role != userEntities.RoleAdmin && actor.Role != userEntities.RoleEventManager {
		return nil, false, gameError(http.StatusForbidden, "FORBIDDEN", "Operação não permitida para este papel.")
	}
	if !actor.OnboardingComplete {
		return nil, false, gameError(http.StatusConflict, "ONBOARDING_REQUIRED", "Conclua o onboarding antes de continuar.")
	}
	return actor, actor.Role == userEntities.RoleAdmin, nil
}

func utcPointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	utc := value.UTC()
	return &utc
}

func managerRunDTO(run *gameEntities.ActivityRun, participants []gameEntities.RunParticipant) *messages.ManagerRunResponseDTO {
	response := &messages.ManagerRunResponseDTO{ID: run.ID, Game: messages.RunGameResponseDTO{ID: run.ActivityID}, Status: string(run.Status), StartedAt: utcPointer(run.StartedAt), EndedAt: utcPointer(run.EndedAt), Participants: make([]messages.RunParticipantResponseDTO, len(participants))}
	if run.Activity != nil {
		response.Game.Name = run.Activity.Name
	}
	for i := range participants {
		response.Participants[i] = appMappers.MapRunParticipantToResponseDTO(participants[i])
	}
	return response
}

func (s *GameService) readManagerRun(ctx context.Context, rawRunID string, lock bool) (*userEntities.User, bool, *gameEntities.ActivityRun, []gameEntities.RunParticipant, error) {
	id, err := uuid.Parse(rawRunID)
	if err != nil {
		return nil, false, nil, nil, gameError(http.StatusNotFound, "NOT_FOUND", "Partida não encontrada.")
	}
	actor, global, err := s.manager(ctx, lock)
	if err != nil {
		return nil, false, nil, nil, err
	}
	if scopeErr := requireInteractiveManagerScope(actor, global); scopeErr != nil {
		return nil, false, nil, nil, scopeErr
	}
	run, err := s.games.FindRunForManager(ctx, id.String(), actor.ID, global, lock)
	if errors.Is(err, appErrors.ErrNotFound) {
		return nil, false, nil, nil, gameError(http.StatusNotFound, "NOT_FOUND", "Partida não encontrada.")
	}
	if err != nil {
		return nil, false, nil, nil, appErrors.InternalError
	}
	participants, err := s.games.ListRunParticipants(ctx, run.ID)
	if err != nil {
		return nil, false, nil, nil, appErrors.InternalError
	}
	return actor, global, run, participants, nil
}

func (s *GameService) ManagerRun(ctx context.Context, runID string) (*messages.ManagerRunResponseDTO, error) {
	_, _, run, participants, err := s.readManagerRun(ctx, runID, false)
	if err != nil {
		return nil, err
	}
	return managerRunDTO(run, participants), nil
}

func dashboardStatus(status gameEntities.RunStatus) string {
	if status == gameEntities.RunStatusDraft {
		return "checkin"
	}
	if status == gameEntities.RunStatusActive {
		return "running"
	}
	return string(status)
}

func (s *GameService) ManagerOverview(ctx context.Context) (*messages.ManagerGameOverviewResponseDTO, error) {
	actor, global, err := s.manager(ctx, false)
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	games, err := s.games.ListManageableGames(ctx, actor.ID, global, now)
	if err != nil {
		return nil, appErrors.InternalError
	}
	scope := "actions"
	if actor.ManagerScope != nil && *actor.ManagerScope != "" {
		scope = *actor.ManagerScope
	}
	response := &messages.ManagerGameOverviewResponseDTO{Scope: scope, Actions: messages.ManagerGameOverviewActionsDTO{Games: make([]messages.ManagerGameResponseDTO, len(games))}}
	schedule, err := s.activities.ListManagerSchedule(ctx, actor.ID, global)
	if err != nil {
		return nil, appErrors.InternalError
	}
	space := &messages.ManagerSpaceOverviewDTO{Upcoming: make([]messages.ManagerSpaceItemResponseDTO, 0, len(schedule))}
	for index := range schedule {
		item := schedule[index]
		spaceItem := messages.ManagerSpaceItemResponseDTO{ID: item.Activity.ID, Title: item.Activity.Name, StartsAt: utcPointer(item.Activity.StartsAt), StartedAt: utcPointer(item.Activity.ActualStartedAt), Status: string(item.Activity.Status), FlexMinutes: item.Activity.FlexMinutes}
		if item.Space != nil {
			spaceItem.SpaceName = item.Space.Name
		}
		if space.Current == nil && item.Activity.Status != activityEntities.StatusCompleted {
			copy := spaceItem
			space.Current = &copy
		} else {
			space.Upcoming = append(space.Upcoming, spaceItem)
		}
	}
	response.Space = space
	rules := gameEntities.DefaultPointRules()
	for i := range games {
		response.Actions.Games[i].ID = games[i].Activity.ID
		response.Actions.Games[i].Name = games[i].Activity.Name
		response.Actions.Games[i].Points.First = rules.First
		response.Actions.Games[i].Points.Second = rules.Second
		response.Actions.Games[i].Points.Third = rules.Third
		response.Actions.Games[i].Points.Participation = rules.Participation
	}
	run, err := s.games.FindOpenRunForManager(ctx, actor.ID, global)
	if errors.Is(err, appErrors.ErrNotFound) {
		return response, nil
	}
	if err != nil {
		return nil, appErrors.InternalError
	}
	participants, err := s.games.ListRunParticipants(ctx, run.ID)
	if err != nil {
		return nil, appErrors.InternalError
	}
	dashboard := &messages.ManagerDashboardRunResponseDTO{ID: run.ID, GameID: run.ActivityID, Status: dashboardStatus(run.Status), StartedAt: utcPointer(run.StartedAt), EndedAt: utcPointer(run.EndedAt), Participants: make([]messages.RunParticipantResponseDTO, len(participants))}
	if run.Activity != nil {
		dashboard.GameName = run.Activity.Name
	}
	for i := range participants {
		dashboard.Participants[i] = appMappers.MapRunParticipantToResponseDTO(participants[i])
	}
	response.Actions.Run = dashboard
	return response, nil
}

func (s *GameService) findPriorManagerOperation(ctx context.Context, actorID uint64, key, operation, hash string) (*gameEntities.ManagerOperation, error) {
	prior, err := s.games.FindManagerOperation(ctx, actorID, key)
	if err == nil {
		if prior.Operation != operation || prior.IntentHash != hash {
			return nil, gameError(http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "Idempotency-Key já foi usada em outra operação.")
		}
		return prior, nil
	}
	if !errors.Is(err, appErrors.ErrNotFound) {
		return nil, appErrors.InternalError
	}
	if _, participantErr := s.games.FindParticipantOperation(ctx, actorID, key); participantErr == nil {
		return nil, gameError(http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "Idempotency-Key já foi usada em outra operação.")
	} else if !errors.Is(participantErr, appErrors.ErrNotFound) {
		return nil, appErrors.InternalError
	}
	if _, auditErr := s.audits.FindByActorAndIdempotencyKey(ctx, actorID, key); auditErr == nil {
		return nil, gameError(http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "Idempotency-Key já foi usada em outra operação.")
	} else if !errors.Is(auditErr, appErrors.ErrNotFound) {
		return nil, appErrors.InternalError
	}
	return nil, nil
}

func parseManagerKey(raw string) (string, error) {
	key, err := uuid.Parse(raw)
	if err != nil {
		return "", gameError(http.StatusBadRequest, "INVALID_REQUEST", "Idempotency-Key deve ser um UUID válido.")
	}
	return key.String(), nil
}

func requireInteractiveManagerScope(actor *userEntities.User, global bool) error {
	if global {
		return nil
	}
	// Existing operational accounts predate manager_scope; retain their former
	// Radicalidade behavior until an administrator assigns an explicit scope.
	if actor.ManagerScope != nil && *actor.ManagerScope != "actions" && *actor.ManagerScope != "special_events" {
		return gameError(http.StatusForbidden, "FORBIDDEN", "Esta operação exige o escopo de Radicalidade ou Eventos especiais.")
	}
	return nil
}

func managerGameDTO(activity *activityEntities.Activity) *messages.ManagerGameResponseDTO {
	rules := gameEntities.DefaultPointRules()
	response := &messages.ManagerGameResponseDTO{ID: activity.ID, Name: activity.Name}
	response.Points.First = rules.First
	response.Points.Second = rules.Second
	response.Points.Third = rules.Third
	response.Points.Participation = rules.Participation
	return response
}

func managerGameName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" || len(name) > 120 {
		return "", gameError(http.StatusBadRequest, "INVALID_REQUEST", "name é obrigatório e deve ter até 120 caracteres.")
	}
	return name, nil
}

func (s *GameService) CreateManagerGame(ctx context.Context, rawKey string, request *messages.CreateManagerGameRequestDTO) (*messages.ManagerGameResponseDTO, int, error) {
	if request == nil {
		return nil, 0, gameError(http.StatusBadRequest, "INVALID_REQUEST", "name é obrigatório.")
	}
	name, err := managerGameName(request.Name)
	if err != nil {
		return nil, 0, err
	}
	key, err := parseManagerKey(rawKey)
	if err != nil {
		return nil, 0, err
	}
	var response *messages.ManagerGameResponseDTO
	err = s.WithTransaction(ctx, func(txCtx context.Context) error {
		actor, global, authErr := s.manager(txCtx, true)
		if authErr != nil {
			return authErr
		}
		if scopeErr := requireInteractiveManagerScope(actor, global); scopeErr != nil {
			return scopeErr
		}
		now := s.now().UTC()
		id := uuid.NewString()
		kind := activityEntities.KindCompetitive
		if actor.ManagerScope != nil && *actor.ManagerScope == "special_events" {
			kind = activityEntities.KindLive
		}
		activity := &activityEntities.Activity{ID: id, Slug: "manager-" + fmt.Sprint(actor.ID) + "-" + strings.ReplaceAll(id[:8], "-", ""), Name: name, Kind: kind, Status: activityEntities.StatusActive, MomentPoints: 5, AllowsMoment: true, CreatedAt: now, UpdatedAt: now}
		created, createErr := s.activities.Create(txCtx, activity)
		if errors.Is(createErr, appErrors.ErrConflict) {
			return gameError(http.StatusConflict, "SLUG_ALREADY_EXISTS", "Não foi possível criar esta atividade. Tente novamente.")
		}
		if createErr != nil {
			return appErrors.InternalError
		}
		if _, assignErr := s.activities.CreateManagerAssignment(txCtx, &activityEntities.ManagerAssignment{ActivityID: created.ID, UserID: actor.ID, CreatedAt: now}); assignErr != nil {
			return appErrors.InternalError
		}
		entityID := created.ID
		metadata, _ := json.Marshal(map[string]any{"name": created.Name, "scope": actor.ManagerScope})
		if _, auditErr := s.audits.Create(txCtx, &auditEntities.OperationAudit{ID: uuid.NewString(), ActorUserID: &actor.ID, Action: "manager.game.create", EntityType: "activity", EntityID: &entityID, Metadata: metadata, IdempotencyKey: key, CreatedAt: now}); auditErr != nil {
			if errors.Is(auditErr, appErrors.ErrConflict) {
				return gameError(http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "Idempotency-Key já foi usada em outra operação.")
			}
			return appErrors.InternalError
		}
		response = managerGameDTO(created)
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	return response, http.StatusCreated, nil
}

func (s *GameService) UpdateManagerGame(ctx context.Context, rawGameID, rawKey string, request *messages.UpdateManagerGameRequestDTO) (*messages.ManagerGameResponseDTO, error) {
	if request == nil {
		return nil, gameError(http.StatusBadRequest, "INVALID_REQUEST", "name é obrigatório.")
	}
	name, err := managerGameName(request.Name)
	if err != nil {
		return nil, err
	}
	gameID, err := uuid.Parse(rawGameID)
	if err != nil {
		return nil, gameError(http.StatusNotFound, "NOT_FOUND", "Jogo não encontrado.")
	}
	key, err := parseManagerKey(rawKey)
	if err != nil {
		return nil, err
	}
	var response *messages.ManagerGameResponseDTO
	err = s.WithTransaction(ctx, func(txCtx context.Context) error {
		actor, global, authErr := s.manager(txCtx, true)
		if authErr != nil {
			return authErr
		}
		if scopeErr := requireInteractiveManagerScope(actor, global); scopeErr != nil {
			return scopeErr
		}
		activity, findErr := s.activities.FindAuthorizedForUpdate(txCtx, gameID.String(), actor.ID, global)
		if errors.Is(findErr, appErrors.ErrNotFound) {
			return gameError(http.StatusNotFound, "NOT_FOUND", "Jogo não encontrado.")
		}
		if findErr != nil {
			return appErrors.InternalError
		}
		if activity.Kind != activityEntities.KindCompetitive && activity.Kind != activityEntities.KindLive {
			return gameError(http.StatusNotFound, "NOT_FOUND", "Jogo não encontrado.")
		}
		activity.Name = name
		activity.UpdatedAt = s.now().UTC()
		updated, updateErr := s.activities.Update(txCtx, activity)
		if updateErr != nil {
			return appErrors.InternalError
		}
		entityID := updated.ID
		metadata, _ := json.Marshal(map[string]any{"name": updated.Name})
		if _, auditErr := s.audits.Create(txCtx, &auditEntities.OperationAudit{ID: uuid.NewString(), ActorUserID: &actor.ID, Action: "manager.game.update", EntityType: "activity", EntityID: &entityID, Metadata: metadata, IdempotencyKey: key, CreatedAt: updated.UpdatedAt}); auditErr != nil {
			if errors.Is(auditErr, appErrors.ErrConflict) {
				return gameError(http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "Idempotency-Key já foi usada em outra operação.")
			}
			return appErrors.InternalError
		}
		response = managerGameDTO(updated)
		return nil
	})
	return response, err
}

func (s *GameService) auditManagerOperation(ctx context.Context, actorID uint64, key, action, runID string, metadata any, now time.Time) error {
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return appErrors.InternalError
	}
	entityID := runID
	_, err = s.audits.Create(ctx, &auditEntities.OperationAudit{ID: uuid.NewString(), ActorUserID: &actorID, Action: action, EntityType: "activity_run", EntityID: &entityID, Metadata: encoded, IdempotencyKey: key, CreatedAt: now})
	if errors.Is(err, appErrors.ErrConflict) {
		return gameError(http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "Idempotency-Key já foi usada em outra operação.")
	}
	if err != nil {
		return appErrors.InternalError
	}
	return nil
}

func (s *GameService) CreateRun(ctx context.Context, rawKey string, request *messages.CreateRunRequestDTO) (*messages.ManagerRunResponseDTO, int, error) {
	if request == nil {
		return nil, 0, gameError(http.StatusBadRequest, "INVALID_REQUEST", "gameId é obrigatório.")
	}
	gameID, err := uuid.Parse(request.GameID)
	if err != nil {
		return nil, 0, gameError(http.StatusNotFound, "NOT_FOUND", "Jogo não encontrado.")
	}
	key, err := parseManagerKey(rawKey)
	if err != nil {
		return nil, 0, err
	}
	operation := "manager.activity-run.create"
	hash := intentHash(operation, struct {
		GameID string `json:"gameId"`
	}{GameID: gameID.String()})
	var response *messages.ManagerRunResponseDTO
	status := http.StatusCreated
	err = s.WithTransaction(ctx, func(txCtx context.Context) error {
		actor, global, authErr := s.manager(txCtx, true)
		if authErr != nil {
			return authErr
		}
		if scopeErr := requireInteractiveManagerScope(actor, global); scopeErr != nil {
			return scopeErr
		}
		prior, priorErr := s.findPriorManagerOperation(txCtx, actor.ID, key, operation, hash)
		if priorErr != nil {
			return priorErr
		}
		if prior != nil && prior.ResultRef != nil {
			run, findErr := s.games.FindRunForManager(txCtx, *prior.ResultRef, actor.ID, global, false)
			if findErr != nil {
				return appErrors.InternalError
			}
			response = managerRunDTO(run, []gameEntities.RunParticipant{})
			response.Status = valueOr(prior.ResultStatus, response.Status)
			response.StartedAt = utcPointer(prior.ResultStartedAt)
			response.EndedAt = utcPointer(prior.ResultEndedAt)
			status = prior.HTTPStatus
			return nil
		}
		now := s.now().UTC()
		activity, findErr := s.games.FindManageableActivityForUpdate(txCtx, gameID.String(), actor.ID, global, now)
		if errors.Is(findErr, appErrors.ErrNotFound) {
			return gameError(http.StatusNotFound, "NOT_FOUND", "Jogo não encontrado.")
		}
		if findErr != nil {
			return appErrors.InternalError
		}
		if _, openErr := s.games.FindOpenRunByActivityForUpdate(txCtx, activity.ID); openErr == nil {
			return gameError(http.StatusConflict, "RUN_STATE_CONFLICT", "Já existe uma partida aberta para este jogo.")
		} else if !errors.Is(openErr, appErrors.ErrNotFound) {
			return appErrors.InternalError
		}
		run := &gameEntities.ActivityRun{ID: uuid.NewString(), ActivityID: activity.ID, StartedBy: actor.ID, Status: gameEntities.RunStatusDraft, PointRules: gameEntities.DefaultPointRules(), CreatedAt: now, UpdatedAt: now, Activity: activity}
		created, createErr := s.games.CreateRun(txCtx, run)
		if errors.Is(createErr, appErrors.ErrConflict) {
			return gameError(http.StatusConflict, "RUN_STATE_CONFLICT", "Já existe uma partida aberta para este jogo.")
		}
		if createErr != nil {
			return appErrors.InternalError
		}
		created.Activity = activity
		resultRef := created.ID
		resultStatus := string(created.Status)
		if createErr = s.games.CreateManagerOperation(txCtx, &gameEntities.ManagerOperation{ID: uuid.NewString(), ActorUserID: actor.ID, IdempotencyKey: key, Operation: operation, ActivityID: activity.ID, ActivityRunID: &resultRef, IntentHash: hash, ResultRef: &resultRef, ResultStatus: &resultStatus, HTTPStatus: status, CreatedAt: now}); createErr != nil {
			return appErrors.InternalError
		}
		if auditErr := s.auditManagerOperation(txCtx, actor.ID, key, operation, created.ID, map[string]any{"status": resultStatus}, now); auditErr != nil {
			return auditErr
		}
		response = managerRunDTO(created, []gameEntities.RunParticipant{})
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	return response, status, nil
}

func valueOr(value *string, fallback string) string {
	if value == nil {
		return fallback
	}
	return *value
}

func (s *GameService) RotateQR(ctx context.Context, rawRunID, rawKey string) (*messages.QRResponseDTO, int, error) {
	runID, err := uuid.Parse(rawRunID)
	if err != nil {
		return nil, 0, gameError(http.StatusNotFound, "NOT_FOUND", "Partida não encontrada.")
	}
	key, err := parseManagerKey(rawKey)
	if err != nil {
		return nil, 0, err
	}
	if err := s.requireQRSecret(); err != nil {
		return nil, 0, err
	}
	operation := "manager.activity-run.qr.rotate"
	hash := intentHash(operation, struct {
		RunID string `json:"runId"`
	}{RunID: runID.String()})
	var response *messages.QRResponseDTO
	status := http.StatusCreated
	err = s.WithTransaction(ctx, func(txCtx context.Context) error {
		actor, global, authErr := s.manager(txCtx, true)
		if authErr != nil {
			return authErr
		}
		run, findErr := s.games.FindRunForManager(txCtx, runID.String(), actor.ID, global, true)
		if errors.Is(findErr, appErrors.ErrNotFound) {
			return gameError(http.StatusNotFound, "NOT_FOUND", "Partida não encontrada.")
		}
		if findErr != nil {
			return appErrors.InternalError
		}
		prior, priorErr := s.findPriorManagerOperation(txCtx, actor.ID, key, operation, hash)
		if priorErr != nil {
			return priorErr
		}
		if prior != nil && prior.ResultRef != nil && prior.ResultExpiresAt != nil {
			token := s.qrToken(*prior.ResultRef)
			response = &messages.QRResponseDTO{RunID: run.ID, QRID: *prior.ResultRef, QRToken: token, ExpiresAt: prior.ResultExpiresAt.UTC()}
			status = prior.HTTPStatus
			return nil
		}
		if run.Status != gameEntities.RunStatusDraft {
			return gameError(http.StatusConflict, "RUN_STATE_CONFLICT", "QR só pode ser gerado enquanto a partida está em draft.")
		}
		now := s.now().UTC()
		if disableErr := s.games.DisableActiveQR(txCtx, run.ID, now); disableErr != nil {
			return appErrors.InternalError
		}
		qrID := uuid.NewString()
		token := s.qrToken(qrID)
		expiresAt := now.Add(qrLifetime)
		_, createErr := s.games.CreateQR(txCtx, &gameEntities.QRCode{ID: qrID, ActivityID: run.ActivityID, ActivityRunID: run.ID, TokenHash: s.qrHash(token), ExpiresAt: expiresAt, Status: gameEntities.QRCodeStatusActive, CreatedAt: now, UpdatedAt: now})
		if createErr != nil {
			return appErrors.InternalError
		}
		resultRef := qrID
		resultStatus := string(run.Status)
		if createErr = s.games.CreateManagerOperation(txCtx, &gameEntities.ManagerOperation{ID: uuid.NewString(), ActorUserID: actor.ID, IdempotencyKey: key, Operation: operation, ActivityID: run.ActivityID, ActivityRunID: &run.ID, IntentHash: hash, ResultRef: &resultRef, ResultStatus: &resultStatus, ResultExpiresAt: &expiresAt, HTTPStatus: status, CreatedAt: now}); createErr != nil {
			return appErrors.InternalError
		}
		if auditErr := s.auditManagerOperation(txCtx, actor.ID, key, operation, run.ID, map[string]any{"qrId": qrID, "expiresAt": expiresAt}, now); auditErr != nil {
			return auditErr
		}
		response = &messages.QRResponseDTO{RunID: run.ID, QRID: qrID, QRToken: token, ExpiresAt: expiresAt}
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	return response, status, nil
}

type transitionSpec struct {
	operation string
	allowed   []gameEntities.RunStatus
	target    gameEntities.RunStatus
	setStart  bool
	setEnd    bool
}

func (s *GameService) transitionRun(ctx context.Context, rawRunID, rawKey string, spec transitionSpec) (*messages.ManagerRunResponseDTO, error) {
	runID, err := uuid.Parse(rawRunID)
	if err != nil {
		return nil, gameError(http.StatusNotFound, "NOT_FOUND", "Partida não encontrada.")
	}
	key, err := parseManagerKey(rawKey)
	if err != nil {
		return nil, err
	}
	hash := intentHash(spec.operation, struct {
		RunID string `json:"runId"`
	}{RunID: runID.String()})
	var response *messages.ManagerRunResponseDTO
	err = s.WithTransaction(ctx, func(txCtx context.Context) error {
		actor, global, authErr := s.manager(txCtx, true)
		if authErr != nil {
			return authErr
		}
		run, findErr := s.games.FindRunForManager(txCtx, runID.String(), actor.ID, global, true)
		if errors.Is(findErr, appErrors.ErrNotFound) {
			return gameError(http.StatusNotFound, "NOT_FOUND", "Partida não encontrada.")
		}
		if findErr != nil {
			return appErrors.InternalError
		}
		prior, priorErr := s.findPriorManagerOperation(txCtx, actor.ID, key, spec.operation, hash)
		if priorErr != nil {
			return priorErr
		}
		if prior != nil {
			response = managerRunDTO(run, []gameEntities.RunParticipant{})
			response.Status = valueOr(prior.ResultStatus, response.Status)
			response.StartedAt = utcPointer(prior.ResultStartedAt)
			response.EndedAt = utcPointer(prior.ResultEndedAt)
			return nil
		}
		valid := false
		for _, allowed := range spec.allowed {
			if run.Status == allowed {
				valid = true
				break
			}
		}
		if !valid {
			return gameError(http.StatusConflict, "RUN_STATE_CONFLICT", "A partida não pode executar esta transição no estado atual.")
		}
		now := s.now().UTC()
		startedAt := run.StartedAt
		var startedUpdate *time.Time
		if spec.setStart && run.StartedAt == nil {
			startedAt = &now
			startedUpdate = &now
		}
		var endedAt *time.Time
		if spec.setEnd {
			endedAt = &now
		}
		if updateErr := s.games.TransitionRun(txCtx, run.ID, run.Status, spec.target, startedUpdate, endedAt, now); updateErr != nil {
			if errors.Is(updateErr, appErrors.ErrConflict) {
				return gameError(http.StatusConflict, "RUN_STATE_CONFLICT", "A partida mudou de estado durante a operação.")
			}
			return appErrors.InternalError
		}
		if run.Status == gameEntities.RunStatusDraft {
			if disableErr := s.games.DisableActiveQR(txCtx, run.ID, now); disableErr != nil {
				return appErrors.InternalError
			}
		}
		if spec.target == gameEntities.RunStatusCancelled {
			if updateErr := s.games.CompleteParticipationStates(txCtx, run.ID, gameEntities.ParticipationStatusCancelled); updateErr != nil {
				return appErrors.InternalError
			}
		}
		resultStatus := string(spec.target)
		if createErr := s.games.CreateManagerOperation(txCtx, &gameEntities.ManagerOperation{ID: uuid.NewString(), ActorUserID: actor.ID, IdempotencyKey: key, Operation: spec.operation, ActivityID: run.ActivityID, ActivityRunID: &run.ID, IntentHash: hash, ResultRef: &run.ID, ResultStatus: &resultStatus, ResultStartedAt: startedAt, ResultEndedAt: endedAt, HTTPStatus: http.StatusOK, CreatedAt: now}); createErr != nil {
			return appErrors.InternalError
		}
		if auditErr := s.auditManagerOperation(txCtx, actor.ID, key, spec.operation, run.ID, map[string]any{"fromStatus": string(run.Status), "toStatus": resultStatus}, now); auditErr != nil {
			return auditErr
		}
		run.Status = spec.target
		run.StartedAt = startedAt
		run.EndedAt = endedAt
		response = managerRunDTO(run, []gameEntities.RunParticipant{})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (s *GameService) StartRun(ctx context.Context, runID, key string) (*messages.ManagerRunResponseDTO, error) {
	return s.transitionRun(ctx, runID, key, transitionSpec{operation: "manager.activity-run.start", allowed: []gameEntities.RunStatus{gameEntities.RunStatusDraft}, target: gameEntities.RunStatusActive, setStart: true})
}

func (s *GameService) PauseRun(ctx context.Context, runID, key string) (*messages.ManagerRunResponseDTO, error) {
	return s.transitionRun(ctx, runID, key, transitionSpec{operation: "manager.activity-run.pause", allowed: []gameEntities.RunStatus{gameEntities.RunStatusActive}, target: gameEntities.RunStatusPaused})
}

func (s *GameService) ResumeRun(ctx context.Context, runID, key string) (*messages.ManagerRunResponseDTO, error) {
	return s.transitionRun(ctx, runID, key, transitionSpec{operation: "manager.activity-run.resume", allowed: []gameEntities.RunStatus{gameEntities.RunStatusPaused}, target: gameEntities.RunStatusActive})
}

func (s *GameService) CancelRun(ctx context.Context, runID, key string) (*messages.ManagerRunResponseDTO, error) {
	return s.transitionRun(ctx, runID, key, transitionSpec{operation: "manager.activity-run.cancel", allowed: []gameEntities.RunStatus{gameEntities.RunStatusDraft, gameEntities.RunStatusActive, gameEntities.RunStatusPaused, gameEntities.RunStatusResults}, target: gameEntities.RunStatusCancelled, setEnd: true})
}

func canonicalResults(request *messages.FinalizeRunResultsRequestDTO) ([]messages.RunResultRequestDTO, error) {
	if request == nil || request.Results == nil {
		return nil, gameError(http.StatusBadRequest, "INVALID_REQUEST", "results é obrigatório.")
	}
	results := append([]messages.RunResultRequestDTO(nil), request.Results...)
	seen := map[string]struct{}{}
	podium := map[string]int{}
	for i := range results {
		id, err := uuid.Parse(results[i].ParticipantID)
		if err != nil {
			return nil, gameError(http.StatusBadRequest, "INVALID_REQUEST", "participantId inválido.")
		}
		results[i].ParticipantID = id.String()
		if _, duplicate := seen[results[i].ParticipantID]; duplicate {
			return nil, gameError(http.StatusBadRequest, "INVALID_REQUEST", "results contém participante duplicado.")
		}
		seen[results[i].ParticipantID] = struct{}{}
		result := gameEntities.Result(results[i].Result)
		switch result {
		case gameEntities.ResultFirst, gameEntities.ResultSecond, gameEntities.ResultThird, gameEntities.ResultParticipation:
		default:
			return nil, gameError(http.StatusBadRequest, "INVALID_REQUEST", "result inválido.")
		}
		if result != gameEntities.ResultParticipation {
			podium[string(result)]++
			if podium[string(result)] > 1 {
				return nil, gameError(http.StatusBadRequest, "INVALID_REQUEST", "Cada posição do pódio aceita no máximo um participante.")
			}
		}
	}
	sort.Slice(results, func(i, j int) bool { return results[i].ParticipantID < results[j].ParticipantID })
	return results, nil
}

func (s *GameService) FinalizeRun(ctx context.Context, rawRunID, rawKey string, request *messages.FinalizeRunResultsRequestDTO) (*messages.ManagerRunResponseDTO, error) {
	runID, err := uuid.Parse(rawRunID)
	if err != nil {
		return nil, gameError(http.StatusNotFound, "NOT_FOUND", "Partida não encontrada.")
	}
	key, err := parseManagerKey(rawKey)
	if err != nil {
		return nil, err
	}
	results, err := canonicalResults(request)
	if err != nil {
		return nil, err
	}
	operation := "manager.activity-run.results.finalize"
	hash := intentHash(operation, struct {
		RunID   string                         `json:"runId"`
		Results []messages.RunResultRequestDTO `json:"results"`
	}{RunID: runID.String(), Results: results})
	var response *messages.ManagerRunResponseDTO
	err = s.WithTransaction(ctx, func(txCtx context.Context) error {
		actor, global, authErr := s.manager(txCtx, true)
		if authErr != nil {
			return authErr
		}
		run, findErr := s.games.FindRunForManager(txCtx, runID.String(), actor.ID, global, true)
		if errors.Is(findErr, appErrors.ErrNotFound) {
			return gameError(http.StatusNotFound, "NOT_FOUND", "Partida não encontrada.")
		}
		if findErr != nil {
			return appErrors.InternalError
		}
		prior, priorErr := s.findPriorManagerOperation(txCtx, actor.ID, key, operation, hash)
		if priorErr != nil {
			return priorErr
		}
		if prior != nil {
			participants, listErr := s.games.ListRunParticipants(txCtx, run.ID)
			if listErr != nil {
				return appErrors.InternalError
			}
			response = managerRunDTO(run, participants)
			response.Status = valueOr(prior.ResultStatus, response.Status)
			response.EndedAt = utcPointer(prior.ResultEndedAt)
			return nil
		}
		if run.Status == gameEntities.RunStatusCompleted {
			participants, listErr := s.games.ListRunParticipants(txCtx, run.ID)
			if listErr != nil {
				return appErrors.InternalError
			}
			response = managerRunDTO(run, participants)
			return nil
		}
		if run.Status != gameEntities.RunStatusActive && run.Status != gameEntities.RunStatusPaused && run.Status != gameEntities.RunStatusResults {
			return gameError(http.StatusConflict, "RUN_STATE_CONFLICT", "A partida não está pronta para resultados.")
		}
		originalStatus := run.Status
		now := s.now().UTC()
		if run.Status == gameEntities.RunStatusActive || run.Status == gameEntities.RunStatusPaused {
			if transitionErr := s.games.TransitionRun(txCtx, run.ID, run.Status, gameEntities.RunStatusResults, nil, nil, now); transitionErr != nil {
				if errors.Is(transitionErr, appErrors.ErrConflict) {
					return gameError(http.StatusConflict, "RUN_STATE_CONFLICT", "A partida mudou de estado durante a operação.")
				}
				return appErrors.InternalError
			}
			run.Status = gameEntities.RunStatusResults
		}
		participants, listErr := s.games.ListRunParticipants(txCtx, run.ID)
		if listErr != nil {
			return appErrors.InternalError
		}
		if len(results) != len(participants) {
			return gameError(http.StatusBadRequest, "INVALID_REQUEST", "results deve conter exatamente os participantes da partida.")
		}
		byID := make(map[string]gameEntities.RunParticipant, len(participants))
		userIDs := make([]uint64, len(participants))
		for i := range participants {
			byID[participants[i].ID] = participants[i]
			userIDs[i] = participants[i].UserID
		}
		for _, item := range results {
			if _, ok := byID[item.ParticipantID]; !ok {
				return gameError(http.StatusBadRequest, "INVALID_REQUEST", "results deve conter exatamente os participantes da partida.")
			}
		}
		if lockErr := s.games.LockUsers(txCtx, userIDs); lockErr != nil {
			return appErrors.InternalError
		}
		for _, item := range results {
			participant := byID[item.ParticipantID]
			result := gameEntities.Result(item.Result)
			points := run.PointRules.PointsFor(result)
			runRef := run.ID
			participationRef := participant.ParticipationID
			entry := &gameEntities.PointEntry{ID: uuid.NewString(), UserID: participant.UserID, ActivityID: run.ActivityID, ActivityRunID: &runRef, ParticipationID: &participationRef, Origin: "activity_run_results", Reason: appMappers.PointReason(result), Delta: points, CreatedAt: now}
			if awardErr := s.games.ApplyAward(txCtx, participant.ID, result, points, entry); awardErr != nil {
				if errors.Is(awardErr, appErrors.ErrConflict) {
					return gameError(http.StatusConflict, "RUN_STATE_CONFLICT", "Os resultados desta partida já foram aplicados.")
				}
				return appErrors.InternalError
			}
		}
		if transitionErr := s.games.TransitionRun(txCtx, run.ID, gameEntities.RunStatusResults, gameEntities.RunStatusCompleted, nil, &now, now); transitionErr != nil {
			if errors.Is(transitionErr, appErrors.ErrConflict) {
				return gameError(http.StatusConflict, "RUN_STATE_CONFLICT", "A partida mudou de estado durante a operação.")
			}
			return appErrors.InternalError
		}
		if updateErr := s.games.CompleteParticipationStates(txCtx, run.ID, gameEntities.ParticipationStatusCompleted); updateErr != nil {
			return appErrors.InternalError
		}
		resultStatus := string(gameEntities.RunStatusCompleted)
		if createErr := s.games.CreateManagerOperation(txCtx, &gameEntities.ManagerOperation{ID: uuid.NewString(), ActorUserID: actor.ID, IdempotencyKey: key, Operation: operation, ActivityID: run.ActivityID, ActivityRunID: &run.ID, IntentHash: hash, ResultRef: &run.ID, ResultStatus: &resultStatus, ResultStartedAt: run.StartedAt, ResultEndedAt: &now, HTTPStatus: http.StatusOK, CreatedAt: now}); createErr != nil {
			return appErrors.InternalError
		}
		if auditErr := s.auditManagerOperation(txCtx, actor.ID, key, operation, run.ID, map[string]any{"fromStatus": string(originalStatus), "toStatus": resultStatus, "participants": len(participants)}, now); auditErr != nil {
			return auditErr
		}
		run.Status = gameEntities.RunStatusCompleted
		run.EndedAt = &now
		participants, listErr = s.games.ListRunParticipants(txCtx, run.ID)
		if listErr != nil {
			return appErrors.InternalError
		}
		response = managerRunDTO(run, participants)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return response, nil
}
