package repositories

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	activityEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/activity/entities"
	favoriteEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/favorite/entities"
	gameEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/game/entities"
	gameInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/game/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/mappers"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const gamePageLimit = 10

type GameRepository struct {
	*BaseRepository[models.ActivityRun]
}

func NewGameRepository(db *gorm.DB) gameInterfaces.GameRepositoryInterface {
	return &GameRepository{BaseRepository: NewBaseRepository[models.ActivityRun](db)}
}

func (r *GameRepository) ListPublicGames(ctx context.Context, generatedAt time.Time, page uint64) (*messages.PaginatedResponse[activityEntities.PublicActivity], error) {
	query := publiclyVisibleActivities(publicActivityQuery(r.getDB(ctx)).Where("activities.kind = ?", string(activityEntities.KindCompetitive)), generatedAt)
	query = orderPublicActivities(query).Limit(gamePageLimit + 1).Offset(int(page) * gamePageLimit)
	var rows []publicActivityRow
	if err := query.Find(&rows).Error; err != nil {
		return nil, handleRepositoryError(err)
	}
	hasNext := len(rows) > gamePageLimit
	if hasNext {
		rows = rows[:gamePageLimit]
	}
	data := make([]activityEntities.PublicActivity, len(rows))
	for index := range rows {
		data[index] = mapPublicActivityRow(&rows[index])
	}
	return &messages.PaginatedResponse[activityEntities.PublicActivity]{Data: data, Pagination: messages.Pagination{CurrentPage: messages.Uint64StringFromUint64(page + 1), HasNextPage: hasNext, Limit: gamePageLimit}}, nil
}

func (r *GameRepository) FindPublicGame(ctx context.Context, activityID string, generatedAt time.Time) (*activityEntities.PublicActivity, error) {
	var row publicActivityRow
	query := publiclyVisibleActivities(publicActivityQuery(r.getDB(ctx)).Where("activities.id = ? AND activities.kind = ?", activityID, string(activityEntities.KindCompetitive)), generatedAt)
	if err := query.Take(&row).Error; err != nil {
		return nil, handleRepositoryError(err)
	}
	item := mapPublicActivityRow(&row)
	return &item, nil
}

func manageableGameQuery(db *gorm.DB, actorUserID uint64, global bool, generatedAt time.Time) *gorm.DB {
	query := publiclyVisibleActivities(publicActivityQuery(db).Where("activities.kind = ?", string(activityEntities.KindCompetitive)), generatedAt)
	if !global {
		query = query.Joins("JOIN activity_manager_assignments ON activity_manager_assignments.activity_id = activities.id AND activity_manager_assignments.user_id = ?", actorUserID)
	}
	return query
}

func (r *GameRepository) ListManageableGames(ctx context.Context, actorUserID uint64, global bool, generatedAt time.Time) ([]activityEntities.PublicActivity, error) {
	var rows []publicActivityRow
	if err := orderPublicActivities(manageableGameQuery(r.getDB(ctx), actorUserID, global, generatedAt)).Find(&rows).Error; err != nil {
		return nil, handleRepositoryError(err)
	}
	data := make([]activityEntities.PublicActivity, len(rows))
	for index := range rows {
		data[index] = mapPublicActivityRow(&rows[index])
	}
	return data, nil
}

func (r *GameRepository) FindManageableActivityForUpdate(ctx context.Context, activityID string, actorUserID uint64, global bool, generatedAt time.Time) (*activityEntities.Activity, error) {
	var row models.Activity
	query := r.getDB(ctx).Model(&models.Activity{}).Clauses(clause.Locking{Strength: "UPDATE"}).Where("activities.id = ? AND activities.kind = ? AND activities.status IN ('active','paused')", activityID, string(activityEntities.KindCompetitive))
	query = publiclyVisibleActivities(query, generatedAt)
	if !global {
		query = query.Joins("JOIN activity_manager_assignments ON activity_manager_assignments.activity_id = activities.id AND activity_manager_assignments.user_id = ?", actorUserID)
	}
	if err := query.Take(&row).Error; err != nil {
		return nil, handleRepositoryError(err)
	}
	return mappers.MapActivityToEntity(&row), nil
}

func mapRunModel(row *models.ActivityRun) (*gameEntities.ActivityRun, error) {
	if row == nil {
		return nil, appErrors.ErrNotFound
	}
	var rules gameEntities.PointRules
	if err := json.Unmarshal(row.PointRules, &rules); err != nil {
		return nil, appErrors.InternalError
	}
	return &gameEntities.ActivityRun{ID: row.ID, ActivityID: row.ActivityID, StartedBy: row.StartedBy, Status: gameEntities.RunStatus(row.Status), PointRules: rules, StartedAt: row.StartedAt, EndedAt: row.EndedAt, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}, nil
}

func mapRunEntity(run *gameEntities.ActivityRun) (*models.ActivityRun, error) {
	rules, err := json.Marshal(run.PointRules)
	if err != nil {
		return nil, err
	}
	return &models.ActivityRun{ID: run.ID, ActivityID: run.ActivityID, StartedBy: run.StartedBy, Status: string(run.Status), PointRules: json.RawMessage(rules), StartedAt: run.StartedAt, EndedAt: run.EndedAt, CreatedAt: run.CreatedAt, UpdatedAt: run.UpdatedAt}, nil
}

func (r *GameRepository) CreateRun(ctx context.Context, run *gameEntities.ActivityRun) (*gameEntities.ActivityRun, error) {
	row, err := mapRunEntity(run)
	if err != nil {
		return nil, appErrors.InternalError
	}
	if err := r.getDB(ctx).Create(row).Error; err != nil {
		return nil, handleRepositoryError(err)
	}
	return mapRunModel(row)
}

func (r *GameRepository) FindOpenRunByActivityForUpdate(ctx context.Context, activityID string) (*gameEntities.ActivityRun, error) {
	var row models.ActivityRun
	err := r.getDB(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("activity_id = ? AND status IN ('draft','active','paused','results')", activityID).Order("created_at DESC").Order("id DESC").First(&row).Error
	if err != nil {
		return nil, handleRepositoryError(err)
	}
	return mapRunModel(&row)
}

func managerRunQuery(db *gorm.DB, actorUserID uint64, global bool) *gorm.DB {
	query := db.Model(&models.ActivityRun{}).Select("activity_runs.*")
	if !global {
		query = query.Joins("JOIN activity_manager_assignments ON activity_manager_assignments.activity_id = activity_runs.activity_id AND activity_manager_assignments.user_id = ?", actorUserID)
	}
	return query
}

func (r *GameRepository) FindOpenRunForManager(ctx context.Context, actorUserID uint64, global bool) (*gameEntities.ActivityRun, error) {
	var row models.ActivityRun
	err := managerRunQuery(r.getDB(ctx), actorUserID, global).Where("activity_runs.status IN ('draft','active','paused','results')").Order("activity_runs.created_at DESC").Order("activity_runs.id DESC").First(&row).Error
	if err != nil {
		return nil, handleRepositoryError(err)
	}
	run, err := mapRunModel(&row)
	if err != nil {
		return nil, err
	}
	var activity models.Activity
	if err := r.getDB(ctx).Where("id = ?", row.ActivityID).Take(&activity).Error; err != nil {
		return nil, handleRepositoryError(err)
	}
	run.Activity = mappers.MapActivityToEntity(&activity)
	return run, nil
}

func (r *GameRepository) FindRunForManager(ctx context.Context, runID string, actorUserID uint64, global bool, lock bool) (*gameEntities.ActivityRun, error) {
	var row models.ActivityRun
	query := managerRunQuery(r.getDB(ctx), actorUserID, global).Where("activity_runs.id = ?", runID)
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE", Table: clause.Table{Name: "activity_runs"}})
	}
	if err := query.Take(&row).Error; err != nil {
		return nil, handleRepositoryError(err)
	}
	run, err := mapRunModel(&row)
	if err != nil {
		return nil, err
	}
	var activity models.Activity
	if err := r.getDB(ctx).Where("id = ?", row.ActivityID).Take(&activity).Error; err != nil {
		return nil, handleRepositoryError(err)
	}
	run.Activity = mappers.MapActivityToEntity(&activity)
	return run, nil
}

type participantRunRow struct {
	models.ActivityRun
	ParticipantID        string    `gorm:"column:participant_id"`
	ParticipationID      string    `gorm:"column:participation_id"`
	ParticipantUserID    uint64    `gorm:"column:participant_user_id"`
	CheckedInAt          time.Time `gorm:"column:participant_checked_in_at"`
	Result               *string   `gorm:"column:participant_result"`
	PointsAwarded        int       `gorm:"column:participant_points_awarded"`
	ParticipantCreatedAt time.Time `gorm:"column:participant_created_at"`
}

func (r *GameRepository) FindRunForParticipant(ctx context.Context, userID uint64, runID *string) (*gameEntities.ActivityRun, *gameEntities.RunParticipant, error) {
	var row participantRunRow
	query := r.getDB(ctx).Table("activity_runs").Select(`activity_runs.*, activity_run_participants.id AS participant_id,
		activity_run_participants.participation_id, activity_run_participants.user_id AS participant_user_id,
		activity_run_participants.checked_in_at AS participant_checked_in_at, activity_run_participants.result AS participant_result,
		activity_run_participants.points_awarded AS participant_points_awarded, activity_run_participants.created_at AS participant_created_at`).Joins("JOIN activity_run_participants ON activity_run_participants.activity_run_id = activity_runs.id AND activity_run_participants.user_id = ?", userID)
	if runID != nil {
		query = query.Where("activity_runs.id = ?", *runID)
	} else {
		query = query.Where("activity_runs.status IN ('draft','active','paused','results')").Order("activity_run_participants.checked_in_at DESC").Order("activity_runs.id DESC")
	}
	if err := query.Take(&row).Error; err != nil {
		return nil, nil, handleRepositoryError(err)
	}
	run, err := mapRunModel(&row.ActivityRun)
	if err != nil {
		return nil, nil, err
	}
	participant := &gameEntities.RunParticipant{ID: row.ParticipantID, ActivityRunID: run.ID, UserID: row.ParticipantUserID, ParticipationID: row.ParticipationID, CheckedInAt: row.CheckedInAt, PointsAwarded: row.PointsAwarded, CreatedAt: row.ParticipantCreatedAt}
	if row.Result != nil {
		result := gameEntities.Result(*row.Result)
		participant.Result = &result
	}
	var activity models.Activity
	if err := r.getDB(ctx).Where("id = ?", run.ActivityID).Take(&activity).Error; err != nil {
		return nil, nil, handleRepositoryError(err)
	}
	run.Activity = mappers.MapActivityToEntity(&activity)
	return run, participant, nil
}

type runParticipantRow struct {
	ID              string
	ActivityRunID   string
	UserID          uint64
	ParticipationID string
	Name            string
	CheckedInAt     time.Time
	Result          *string
	PointsAwarded   int
	CreatedAt       time.Time
}

func (r *GameRepository) ListRunParticipants(ctx context.Context, runID string) ([]gameEntities.RunParticipant, error) {
	var rows []runParticipantRow
	err := r.getDB(ctx).Table("activity_run_participants").Select("activity_run_participants.*, users.name").Joins("JOIN users ON users.id = activity_run_participants.user_id").Where("activity_run_participants.activity_run_id = ?", runID).Order("activity_run_participants.checked_in_at ASC").Order("activity_run_participants.user_id ASC").Find(&rows).Error
	if err != nil {
		return nil, handleRepositoryError(err)
	}
	data := make([]gameEntities.RunParticipant, len(rows))
	for i := range rows {
		data[i] = gameEntities.RunParticipant{ID: rows[i].ID, ActivityRunID: rows[i].ActivityRunID, UserID: rows[i].UserID, ParticipationID: rows[i].ParticipationID, Name: rows[i].Name, CheckedInAt: rows[i].CheckedInAt, PointsAwarded: rows[i].PointsAwarded, CreatedAt: rows[i].CreatedAt}
		if rows[i].Result != nil {
			result := gameEntities.Result(*rows[i].Result)
			data[i].Result = &result
		}
	}
	return data, nil
}

func (r *GameRepository) TransitionRun(ctx context.Context, runID string, from, to gameEntities.RunStatus, startedAt, endedAt *time.Time, updatedAt time.Time) error {
	updates := map[string]any{"status": string(to), "updated_at": updatedAt}
	if startedAt != nil {
		updates["started_at"] = *startedAt
	}
	if endedAt != nil {
		updates["ended_at"] = *endedAt
	}
	result := r.getDB(ctx).Model(&models.ActivityRun{}).Where("id = ? AND status = ?", runID, string(from)).Updates(updates)
	if result.Error != nil {
		return handleRepositoryError(result.Error)
	}
	if result.RowsAffected != 1 {
		return appErrors.ErrConflict
	}
	return nil
}

func (r *GameRepository) CompleteParticipationStates(ctx context.Context, runID string, status gameEntities.ParticipationStatus) error {
	return handleRepositoryError(r.getDB(ctx).Model(&models.Participation{}).Where("activity_run_id = ?", runID).Update("status", string(status)).Error)
}

func (r *GameRepository) DisableActiveQR(ctx context.Context, runID string, updatedAt time.Time) error {
	return handleRepositoryError(r.getDB(ctx).Model(&models.ActivityRunQRCode{}).Where("activity_run_id = ? AND status = ?", runID, string(gameEntities.QRCodeStatusActive)).Updates(map[string]any{"status": string(gameEntities.QRCodeStatusDisabled), "updated_at": updatedAt}).Error)
}

func (r *GameRepository) CreateQR(ctx context.Context, qr *gameEntities.QRCode) (*gameEntities.QRCode, error) {
	row := &models.ActivityRunQRCode{ID: qr.ID, ActivityID: qr.ActivityID, ActivityRunID: qr.ActivityRunID, TokenHash: qr.TokenHash, ExpiresAt: qr.ExpiresAt, Status: string(qr.Status), CreatedAt: qr.CreatedAt, UpdatedAt: qr.UpdatedAt}
	if err := r.getDB(ctx).Create(row).Error; err != nil {
		return nil, handleRepositoryError(err)
	}
	return qr, nil
}

func (r *GameRepository) FindQRByTokenHashForUpdate(ctx context.Context, tokenHash string, generatedAt time.Time) (*gameEntities.QRCode, error) {
	var row struct {
		models.ActivityRunQRCode
		AllowsMoment bool `gorm:"column:allows_moment"`
	}
	query := r.getDB(ctx).Model(&models.ActivityRunQRCode{}).Select("activity_run_qr_codes.*, activities.allows_moment AS allows_moment").Clauses(clause.Locking{Strength: "UPDATE", Table: clause.Table{Name: "activity_run_qr_codes"}}).Joins("JOIN activity_runs ON activity_runs.id = activity_run_qr_codes.activity_run_id").Joins("JOIN activities ON activities.id = activity_run_qr_codes.activity_id").Where("activity_run_qr_codes.token_hash = ? AND activity_runs.status = 'draft' AND activities.kind = ?", tokenHash, string(activityEntities.KindCompetitive))
	query = publiclyVisibleActivities(query, generatedAt)
	if err := query.Take(&row).Error; err != nil {
		return nil, handleRepositoryError(err)
	}
	return &gameEntities.QRCode{ID: row.ID, ActivityID: row.ActivityID, ActivityRunID: row.ActivityRunID, AllowsMoment: row.AllowsMoment, TokenHash: row.TokenHash, ExpiresAt: row.ExpiresAt, Status: gameEntities.QRCodeStatus(row.Status), CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}, nil
}

func (r *GameRepository) FindParticipantOperation(ctx context.Context, actorUserID uint64, key string) (*favoriteEntities.ParticipantOperation, error) {
	var row models.ParticipantOperation
	if err := r.getDB(ctx).Where("actor_user_id = ? AND idempotency_key = ?", actorUserID, key).Take(&row).Error; err != nil {
		return nil, handleRepositoryError(err)
	}
	return mappers.MapParticipantOperationToEntity(&row), nil
}

func (r *GameRepository) CreateParticipantOperation(ctx context.Context, operation *favoriteEntities.ParticipantOperation) error {
	return handleRepositoryError(r.getDB(ctx).Create(mappers.MapParticipantOperationEntityToModel(operation)).Error)
}

func mapManagerOperation(row *models.ManagerOperation) *gameEntities.ManagerOperation {
	return &gameEntities.ManagerOperation{ID: row.ID, ActorUserID: row.ActorUserID, IdempotencyKey: row.IdempotencyKey, Operation: row.Operation, ActivityID: row.ActivityID, ActivityRunID: row.ActivityRunID, IntentHash: row.IntentHash, ResultRef: row.ResultRef, ResultStatus: row.ResultStatus, ResultStartedAt: row.ResultStartedAt, ResultEndedAt: row.ResultEndedAt, ResultExpiresAt: row.ResultExpiresAt, HTTPStatus: row.HTTPStatus, CreatedAt: row.CreatedAt}
}

func (r *GameRepository) FindManagerOperation(ctx context.Context, actorUserID uint64, key string) (*gameEntities.ManagerOperation, error) {
	var row models.ManagerOperation
	if err := r.getDB(ctx).Where("actor_user_id = ? AND idempotency_key = ?", actorUserID, key).Take(&row).Error; err != nil {
		return nil, handleRepositoryError(err)
	}
	return mapManagerOperation(&row), nil
}

func (r *GameRepository) CreateManagerOperation(ctx context.Context, operation *gameEntities.ManagerOperation) error {
	row := &models.ManagerOperation{ID: operation.ID, ActorUserID: operation.ActorUserID, IdempotencyKey: operation.IdempotencyKey, Operation: operation.Operation, ActivityID: operation.ActivityID, ActivityRunID: operation.ActivityRunID, IntentHash: operation.IntentHash, ResultRef: operation.ResultRef, ResultStatus: operation.ResultStatus, ResultStartedAt: operation.ResultStartedAt, ResultEndedAt: operation.ResultEndedAt, ResultExpiresAt: operation.ResultExpiresAt, HTTPStatus: operation.HTTPStatus, CreatedAt: operation.CreatedAt}
	return handleRepositoryError(r.getDB(ctx).Create(row).Error)
}

type participationRow struct {
	models.Participation
	ActivityName string  `gorm:"column:activity_name"`
	SpaceID      *string `gorm:"column:space_id"`
	SpaceName    *string `gorm:"column:space_name"`
}

func mapParticipationRow(row *participationRow) *gameEntities.Participation {
	return &gameEntities.Participation{ID: row.ID, UserID: row.UserID, ActivityID: row.ActivityID, ActivityRunID: row.ActivityRunID, QRCodeID: row.QRCodeID, CheckedInAt: row.CheckedInAt, Status: gameEntities.ParticipationStatus(row.Status), CanShareMoment: row.CanShareMoment, CheckInPoints: row.CheckInPoints, CreatedAt: row.CreatedAt, ActivityName: row.ActivityName, SpaceID: row.SpaceID, SpaceName: row.SpaceName}
}

func participationProjection(db *gorm.DB) *gorm.DB {
	return db.Table("participations").Select("participations.*, activities.name AS activity_name, spaces.id AS space_id, spaces.name AS space_name").Joins("JOIN activities ON activities.id = participations.activity_id").Joins("LEFT JOIN spaces ON spaces.id = activities.space_id")
}

func (r *GameRepository) FindParticipationByID(ctx context.Context, participationID string) (*gameEntities.Participation, error) {
	var row participationRow
	if err := participationProjection(r.getDB(ctx)).Where("participations.id = ?", participationID).Take(&row).Error; err != nil {
		return nil, handleRepositoryError(err)
	}
	return mapParticipationRow(&row), nil
}

func (r *GameRepository) FindParticipationByRunAndUser(ctx context.Context, runID string, userID uint64) (*gameEntities.Participation, error) {
	var row participationRow
	if err := participationProjection(r.getDB(ctx)).Where("participations.activity_run_id = ? AND participations.user_id = ?", runID, userID).Take(&row).Error; err != nil {
		return nil, handleRepositoryError(err)
	}
	return mapParticipationRow(&row), nil
}

func (r *GameRepository) CreateParticipation(ctx context.Context, participation *gameEntities.Participation, participant *gameEntities.RunParticipant) error {
	row := &models.Participation{ID: participation.ID, UserID: participation.UserID, ActivityID: participation.ActivityID, ActivityRunID: participation.ActivityRunID, QRCodeID: participation.QRCodeID, CheckedInAt: participation.CheckedInAt, Status: string(participation.Status), CanShareMoment: participation.CanShareMoment, CheckInPoints: participation.CheckInPoints, CreatedAt: participation.CreatedAt}
	if err := r.getDB(ctx).Create(row).Error; err != nil {
		return handleRepositoryError(err)
	}
	participantRow := &models.ActivityRunParticipant{ID: participant.ID, ActivityRunID: participant.ActivityRunID, UserID: participant.UserID, ParticipationID: participant.ParticipationID, CheckedInAt: participant.CheckedInAt, PointsAwarded: participant.PointsAwarded, CreatedAt: participant.CreatedAt}
	return handleRepositoryError(r.getDB(ctx).Create(participantRow).Error)
}

func (r *GameRepository) FindCurrentParticipation(ctx context.Context, userID uint64) (*gameEntities.Participation, error) {
	var row participationRow
	query := participationProjection(r.getDB(ctx)).Joins("JOIN activity_runs ON activity_runs.id = participations.activity_run_id").Where("participations.user_id = ? AND participations.status = 'active' AND activity_runs.status IN ('draft','active','paused','results')", userID).Order("participations.checked_in_at DESC").Order("participations.id DESC")
	if err := query.Take(&row).Error; err != nil {
		return nil, handleRepositoryError(err)
	}
	return mapParticipationRow(&row), nil
}

func (r *GameRepository) LockUsers(ctx context.Context, userIDs []uint64) error {
	ids := append([]uint64(nil), userIDs...)
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		var user models.User
		if err := r.getDB(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).Take(&user).Error; err != nil {
			return handleRepositoryError(err)
		}
	}
	return nil
}

func (r *GameRepository) ApplyAward(ctx context.Context, participantID string, result gameEntities.Result, points int, entry *gameEntities.PointEntry) error {
	update := r.getDB(ctx).Model(&models.ActivityRunParticipant{}).Where("id = ? AND result IS NULL", participantID).Updates(map[string]any{"result": string(result), "points_awarded": points})
	if update.Error != nil {
		return handleRepositoryError(update.Error)
	}
	if update.RowsAffected != 1 {
		return appErrors.ErrConflict
	}
	activityID := entry.ActivityID
	row := &models.PointEntry{ID: entry.ID, UserID: entry.UserID, ActivityID: &activityID, ActivityRunID: entry.ActivityRunID, ParticipationID: entry.ParticipationID, Origin: entry.Origin, Reason: entry.Reason, Delta: entry.Delta, CreatedAt: entry.CreatedAt}
	if err := r.getDB(ctx).Create(row).Error; err != nil {
		return handleRepositoryError(err)
	}
	balance := r.getDB(ctx).Model(&models.User{}).Where("id = ? AND points + ? >= 0", entry.UserID, entry.Delta).UpdateColumn("points", gorm.Expr("points + ?", entry.Delta))
	if balance.Error != nil {
		return handleRepositoryError(balance.Error)
	}
	if balance.RowsAffected != 1 {
		return appErrors.ErrConflict
	}
	return nil
}

type individualRankingRow struct {
	UserID    uint64 `gorm:"column:user_id"`
	Name      string
	GroupName *string
	Points    int
	Position  uint64
}

const individualRankingCTE = `WITH ranked AS (
	SELECT users.id AS user_id, users.name, groups.name AS group_name, users.points,
		ROW_NUMBER() OVER (ORDER BY users.points DESC, users.name ASC, users.id ASC) AS position
	FROM users
	LEFT JOIN group_memberships ON group_memberships.user_id = users.id
	LEFT JOIN groups ON groups.id = group_memberships.group_id
	WHERE users.onboarding_complete = TRUE AND users.role = 'DEFAULT'
)`

func (r *GameRepository) listIndividual(ctx context.Context, limit int, offset int) ([]gameEntities.IndividualRanking, error) {
	var rows []individualRankingRow
	if err := r.getDB(ctx).Raw(individualRankingCTE+` SELECT * FROM ranked ORDER BY position LIMIT ? OFFSET ?`, limit, offset).Scan(&rows).Error; err != nil {
		return nil, handleRepositoryError(err)
	}
	data := make([]gameEntities.IndividualRanking, len(rows))
	for i := range rows {
		data[i] = gameEntities.IndividualRanking{UserID: rows[i].UserID, Name: rows[i].Name, GroupName: rows[i].GroupName, Points: rows[i].Points, Position: rows[i].Position}
	}
	return data, nil
}

func (r *GameRepository) ListIndividualRankings(ctx context.Context, page uint64) (*messages.PaginatedResponse[gameEntities.IndividualRanking], error) {
	data, err := r.listIndividual(ctx, gamePageLimit+1, int(page)*gamePageLimit)
	if err != nil {
		return nil, err
	}
	hasNext := len(data) > gamePageLimit
	if hasNext {
		data = data[:gamePageLimit]
	}
	return &messages.PaginatedResponse[gameEntities.IndividualRanking]{Data: data, Pagination: messages.Pagination{CurrentPage: messages.Uint64StringFromUint64(page + 1), HasNextPage: hasNext, Limit: gamePageLimit}}, nil
}

type groupRankingRow struct {
	GroupID  uint64
	Name     string
	Members  int
	Points   int
	Position uint64
}

const groupRankingCTE = `WITH totals AS (
	SELECT groups.id AS group_id, groups.name, COUNT(users.id) AS members, COALESCE(SUM(users.points), 0) AS points
	FROM groups
	LEFT JOIN group_memberships ON group_memberships.group_id = groups.id
	LEFT JOIN users ON users.id = group_memberships.user_id AND users.onboarding_complete = TRUE AND users.role = 'DEFAULT'
	GROUP BY groups.id, groups.name
), ranked AS (
	SELECT totals.*, ROW_NUMBER() OVER (ORDER BY totals.points DESC, totals.name ASC, totals.group_id ASC) AS position FROM totals
)`

func (r *GameRepository) listGroups(ctx context.Context, limit int, offset int) ([]gameEntities.GroupRanking, error) {
	var rows []groupRankingRow
	if err := r.getDB(ctx).Raw(groupRankingCTE+` SELECT * FROM ranked ORDER BY position LIMIT ? OFFSET ?`, limit, offset).Scan(&rows).Error; err != nil {
		return nil, handleRepositoryError(err)
	}
	data := make([]gameEntities.GroupRanking, len(rows))
	for i := range rows {
		data[i] = gameEntities.GroupRanking{GroupID: rows[i].GroupID, Name: rows[i].Name, Members: rows[i].Members, Points: rows[i].Points, Position: rows[i].Position}
	}
	return data, nil
}

func (r *GameRepository) ListGroupRankings(ctx context.Context, page uint64) (*messages.PaginatedResponse[gameEntities.GroupRanking], error) {
	data, err := r.listGroups(ctx, gamePageLimit+1, int(page)*gamePageLimit)
	if err != nil {
		return nil, err
	}
	hasNext := len(data) > gamePageLimit
	if hasNext {
		data = data[:gamePageLimit]
	}
	return &messages.PaginatedResponse[gameEntities.GroupRanking]{Data: data, Pagination: messages.Pagination{CurrentPage: messages.Uint64StringFromUint64(page + 1), HasNextPage: hasNext, Limit: gamePageLimit}}, nil
}

func (r *GameRepository) TopIndividualRankings(ctx context.Context, limit int) ([]gameEntities.IndividualRanking, error) {
	return r.listIndividual(ctx, limit, 0)
}
func (r *GameRepository) TopGroupRankings(ctx context.Context, limit int) ([]gameEntities.GroupRanking, error) {
	return r.listGroups(ctx, limit, 0)
}

func (r *GameRepository) FindCurrentRanking(ctx context.Context, userID uint64) (*gameEntities.IndividualRanking, *gameEntities.GroupRanking, error) {
	var individualRows []individualRankingRow
	if err := r.getDB(ctx).Raw(individualRankingCTE+` SELECT * FROM ranked WHERE user_id = ?`, userID).Scan(&individualRows).Error; err != nil {
		return nil, nil, handleRepositoryError(err)
	}
	if len(individualRows) == 0 {
		return nil, nil, appErrors.ErrNotFound
	}
	individual := &gameEntities.IndividualRanking{UserID: individualRows[0].UserID, Name: individualRows[0].Name, GroupName: individualRows[0].GroupName, Points: individualRows[0].Points, Position: individualRows[0].Position}
	var groupRows []groupRankingRow
	if err := r.getDB(ctx).Raw(groupRankingCTE+` SELECT ranked.* FROM ranked JOIN group_memberships ON group_memberships.group_id = ranked.group_id WHERE group_memberships.user_id = ?`, userID).Scan(&groupRows).Error; err != nil {
		return nil, nil, handleRepositoryError(err)
	}
	if len(groupRows) == 0 {
		return individual, nil, nil
	}
	group := &gameEntities.GroupRanking{GroupID: groupRows[0].GroupID, Name: groupRows[0].Name, Members: groupRows[0].Members, Points: groupRows[0].Points, Position: groupRows[0].Position}
	return individual, group, nil
}

func (r *GameRepository) ListPointEntries(ctx context.Context, userID uint64, limit int) ([]gameEntities.PointEntry, error) {
	var rows []models.PointEntry
	if err := r.getDB(ctx).Where("user_id = ?", userID).Order("created_at DESC").Order("id DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, handleRepositoryError(err)
	}
	data := make([]gameEntities.PointEntry, len(rows))
	for i := range rows {
		activityID := ""
		if rows[i].ActivityID != nil {
			activityID = *rows[i].ActivityID
		}
		data[i] = gameEntities.PointEntry{ID: rows[i].ID, UserID: rows[i].UserID, ActivityID: activityID, ActivityRunID: rows[i].ActivityRunID, ParticipationID: rows[i].ParticipationID, Origin: rows[i].Origin, Reason: rows[i].Reason, Delta: rows[i].Delta, CreatedAt: rows[i].CreatedAt}
	}
	return data, nil
}

func (r *GameRepository) ListPointBalanceMismatches(ctx context.Context) ([]gameEntities.PointBalanceMismatch, error) {
	var rows []gameEntities.PointBalanceMismatch
	err := r.getDB(ctx).Raw(`SELECT users.id AS user_id,
		COALESCE(SUM(point_entries.delta), 0) AS ledger_points,
		users.points AS materialized_points
		FROM users
		LEFT JOIN point_entries ON point_entries.user_id = users.id
		GROUP BY users.id, users.points
		HAVING COALESCE(SUM(point_entries.delta), 0) <> users.points
		ORDER BY users.id`).Scan(&rows).Error
	if err != nil {
		return nil, handleRepositoryError(err)
	}
	return rows, nil
}

var _ gameInterfaces.GameRepositoryInterface = (*GameRepository)(nil)
