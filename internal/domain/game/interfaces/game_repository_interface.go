package interfaces

import (
	"context"
	"time"

	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	activityEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/activity/entities"
	favoriteEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/favorite/entities"
	gameEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/game/entities"
)

type GameRepositoryInterface interface {
	ListPublicGames(ctx context.Context, generatedAt time.Time, page uint64) (*messages.PaginatedResponse[activityEntities.PublicActivity], error)
	FindPublicGame(ctx context.Context, activityID string, generatedAt time.Time) (*activityEntities.PublicActivity, error)
	ListManageableGames(ctx context.Context, actorUserID uint64, global bool, generatedAt time.Time) ([]activityEntities.PublicActivity, error)
	FindManageableActivityForUpdate(ctx context.Context, activityID string, actorUserID uint64, global bool, generatedAt time.Time) (*activityEntities.Activity, error)

	CreateRun(ctx context.Context, run *gameEntities.ActivityRun) (*gameEntities.ActivityRun, error)
	FindOpenRunByActivityForUpdate(ctx context.Context, activityID string) (*gameEntities.ActivityRun, error)
	FindOpenRunForManager(ctx context.Context, actorUserID uint64, global bool) (*gameEntities.ActivityRun, error)
	FindRunForManager(ctx context.Context, runID string, actorUserID uint64, global bool, lock bool) (*gameEntities.ActivityRun, error)
	FindRunForParticipant(ctx context.Context, userID uint64, runID *string) (*gameEntities.ActivityRun, *gameEntities.RunParticipant, error)
	ListRunParticipants(ctx context.Context, runID string) ([]gameEntities.RunParticipant, error)
	TransitionRun(ctx context.Context, runID string, from, to gameEntities.RunStatus, startedAt, endedAt *time.Time, updatedAt time.Time) error
	CompleteParticipationStates(ctx context.Context, runID string, status gameEntities.ParticipationStatus) error

	DisableActiveQR(ctx context.Context, runID string, updatedAt time.Time) error
	CreateQR(ctx context.Context, qr *gameEntities.QRCode) (*gameEntities.QRCode, error)
	FindActiveQRByRun(ctx context.Context, runID string) (*gameEntities.QRCode, error)
	FindQRByTokenHashForUpdate(ctx context.Context, tokenHash string, generatedAt time.Time) (*gameEntities.QRCode, error)

	FindParticipantOperation(ctx context.Context, actorUserID uint64, key string) (*favoriteEntities.ParticipantOperation, error)
	CreateParticipantOperation(ctx context.Context, operation *favoriteEntities.ParticipantOperation) error
	FindManagerOperation(ctx context.Context, actorUserID uint64, key string) (*gameEntities.ManagerOperation, error)
	CreateManagerOperation(ctx context.Context, operation *gameEntities.ManagerOperation) error

	FindParticipationByID(ctx context.Context, participationID string) (*gameEntities.Participation, error)
	FindParticipationByRunAndUser(ctx context.Context, runID string, userID uint64) (*gameEntities.Participation, error)
	CreateParticipation(ctx context.Context, participation *gameEntities.Participation, participant *gameEntities.RunParticipant) error
	FindCurrentParticipation(ctx context.Context, userID uint64) (*gameEntities.Participation, error)

	LockUsers(ctx context.Context, userIDs []uint64) error
	ApplyAward(ctx context.Context, participantID string, result gameEntities.Result, points int, entry *gameEntities.PointEntry) error

	ListIndividualRankings(ctx context.Context, page uint64) (*messages.PaginatedResponse[gameEntities.IndividualRanking], error)
	ListGroupRankings(ctx context.Context, page uint64) (*messages.PaginatedResponse[gameEntities.GroupRanking], error)
	TopIndividualRankings(ctx context.Context, limit int) ([]gameEntities.IndividualRanking, error)
	TopGroupRankings(ctx context.Context, limit int) ([]gameEntities.GroupRanking, error)
	FindCurrentRanking(ctx context.Context, userID uint64) (*gameEntities.IndividualRanking, *gameEntities.GroupRanking, error)
	ListPointEntries(ctx context.Context, userID uint64, limit int) ([]gameEntities.PointEntry, error)
	ListPointBalanceMismatches(ctx context.Context) ([]gameEntities.PointBalanceMismatch, error)
}
