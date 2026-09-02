package repositories

import (
	"context"
	"errors"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	gameEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/game/entities"
	mediaEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/media/entities"
	momentEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/moment/entities"
	momentInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/moment/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/mappers"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const momentPageLimit = 20

type MomentRepository struct{ *BaseRepository[models.Moment] }

func (r *MomentRepository) FindActiveMomentChallengeForUpdate(ctx context.Context, now time.Time) (string, int, error) {
	type row struct {
		ID           string
		MomentPoints int
	}
	var rows []row
	err := r.getDB(ctx).Table("activities").
		Select("id,moment_points").
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("kind = ? AND status = ? AND allows_moment = TRUE AND (starts_at IS NULL OR starts_at <= ?) AND (ends_at IS NULL OR ends_at > ?)", "challenge", "active", now.UTC(), now.UTC()).
		Order("starts_at ASC NULLS LAST").
		Limit(2).
		Find(&rows).Error
	if err != nil {
		return "", 0, handleRepositoryError(err)
	}
	if len(rows) == 0 {
		return "", 0, appErrors.ErrNotFound
	}
	if len(rows) > 1 {
		return "", 0, appErrors.ErrConflict
	}
	return rows[0].ID, rows[0].MomentPoints, nil
}

func (r *MomentRepository) HasMomentForActivity(ctx context.Context, userID uint64, activityID string) (bool, error) {
	var count int64
	if err := r.getDB(ctx).Model(&models.Moment{}).Where("user_id = ? AND activity_id = ? AND origin = ?", userID, activityID, string(momentEntities.OriginChallenge)).Count(&count).Error; err != nil {
		return false, handleRepositoryError(err)
	}
	return count > 0, nil
}

func NewMomentRepository(db *gorm.DB) momentInterfaces.Repository {
	return &MomentRepository{BaseRepository: NewBaseRepository[models.Moment](db)}
}

func (r *MomentRepository) FindParticipationForUpdate(
	ctx context.Context,
	id string,
) (*gameEntities.Participation, error) {
	var row struct {
		models.Participation
		ActivityName string
		SpaceID      *string
		SpaceName    *string
	}
	err := r.getDB(ctx).
		Table("participations").
		Select("participations.*,activities.name AS activity_name,spaces.id AS space_id,spaces.name AS space_name").
		Joins("JOIN activities ON activities.id=participations.activity_id").
		Joins("LEFT JOIN spaces ON spaces.id=activities.space_id").
		Clauses(clause.Locking{Strength: "UPDATE", Table: clause.Table{Name: "participations"}}).
		Where("participations.id=?", id).
		First(&row).
		Error
	if err != nil {
		return nil, handleRepositoryError(err)
	}
	return &gameEntities.Participation{
		ID:             row.ID,
		UserID:         row.UserID,
		ActivityID:     row.ActivityID,
		ActivityRunID:  row.ActivityRunID,
		QRCodeID:       row.QRCodeID,
		CheckedInAt:    row.CheckedInAt,
		Status:         gameEntities.ParticipationStatus(row.Status),
		CanShareMoment: row.CanShareMoment,
		CheckInPoints:  row.CheckInPoints,
		CreatedAt:      row.CreatedAt,
		ActivityName:   row.ActivityName,
		SpaceID:        row.SpaceID,
		SpaceName:      row.SpaceName,
	}, nil
}

func (r *MomentRepository) FindActivityForUpdate(
	ctx context.Context,
	id string,
) (string, bool, *time.Time, *time.Time, int, string, *string, error) {
	var row struct {
		Status           string
		AllowsMoment     bool
		StartsAt, EndsAt *time.Time
		MomentPoints     int
		Name             string
		PlaceName        *string
	}
	err := r.getDB(ctx).
		Table("activities").
		Select("activities.status,activities.allows_moment,activities.starts_at,activities.ends_at,activities.moment_points,activities.name,spaces.name AS place_name").
		Joins("LEFT JOIN spaces ON spaces.id=activities.space_id").
		Clauses(clause.Locking{Strength: "UPDATE", Table: clause.Table{Name: "activities"}}).
		Where("activities.id=?", id).
		Take(&row).
		Error
	if err != nil {
		return "", false, nil, nil, 0, "", nil, handleRepositoryError(err)
	}
	return row.Status, row.AllowsMoment, row.StartsAt, row.EndsAt, row.MomentPoints, row.Name, row.PlaceName, nil
}
func (r *MomentRepository) CreateMoment(ctx context.Context, item *momentEntities.Moment) error {
	return handleRepositoryError(r.getDB(ctx).Create(mappers.MapMomentEntityToModel(item)).Error)
}

type momentProjection struct {
	models.Moment
	AuthorName              string
	GroupID                 *uint64
	ActivityName, PlaceName *string
	AssetState              string
	AssetRetentionDueAt     time.Time
	LikesCount              int
	LikedByCurrentUser      bool
	AuthorEligible          bool
}

func projectMoment(row *momentProjection) *momentEntities.Moment {
	item := mappers.MapMomentToEntity(&row.Moment)
	item.AuthorName = row.AuthorName
	item.GroupID = row.GroupID
	item.ActivityName = row.ActivityName
	item.PlaceName = row.PlaceName
	item.AssetAvailable = row.AssetState == string(mediaEntities.AssetAvailable)
	item.AssetRetentionDueAt = row.AssetRetentionDueAt
	item.LikesCount = row.LikesCount
	item.LikedByCurrentUser = row.LikedByCurrentUser
	item.AuthorEligible = row.AuthorEligible
	return item
}
func projectionQuery(db *gorm.DB, actor uint64) *gorm.DB {
	return db.Table("moments").
		Select(`moments.*,users.name AS author_name,users.group_id,activities.name AS activity_name,spaces.name AS place_name,media_assets.state AS asset_state,media_assets.retention_due_at AS asset_retention_due_at,(users.deleted_at IS NULL AND users.onboarding_complete = TRUE AND users.role = 'DEFAULT') AS author_eligible,(SELECT COUNT(*) FROM moment_likes ml WHERE ml.moment_id=moments.id) AS likes_count,EXISTS(SELECT 1 FROM moment_likes mine_like WHERE mine_like.moment_id=moments.id AND mine_like.user_id=?) AS liked_by_current_user`, actor).
		Joins("JOIN users ON users.id=moments.user_id").
		Joins("JOIN media_assets ON media_assets.id=moments.media_asset_id").
		Joins("LEFT JOIN activities ON activities.id=moments.activity_id").
		Joins("LEFT JOIN spaces ON spaces.id=activities.space_id")
}

func (r *MomentRepository) FindMoment(
	ctx context.Context,
	id string,
	actor uint64,
	lock bool,
) (*momentEntities.Moment, error) {
	var row momentProjection
	q := projectionQuery(r.getDB(ctx), actor).Where("moments.id=?", id)
	if lock {
		q = q.Clauses(clause.Locking{Strength: "UPDATE", Table: clause.Table{Name: "moments"}})
	}
	if err := q.Take(&row).Error; err != nil {
		return nil, handleRepositoryError(err)
	}
	return projectMoment(&row), nil
}

func (r *MomentRepository) ListMoments(
	ctx context.Context,
	scope string,
	actor uint64,
	groupID *uint64,
	cursor *momentEntities.Cursor,
	now time.Time,
) (*momentEntities.Page, error) {
	q := projectionQuery(r.getDB(ctx), actor)
	switch scope {
	case "mine":
		q = q.Where("moments.user_id=?", actor)
	case "feed":
		q = q.Where(
			"moments.publication_status='public' AND moments.moderation_status<>'rejected' AND media_assets.state='available' AND media_assets.retention_due_at>? AND users.deleted_at IS NULL AND users.onboarding_complete=TRUE AND users.role='DEFAULT'",
			now,
		)
	case "group":
		if groupID == nil {
			return &momentEntities.Page{Items: []momentEntities.Moment{}}, nil
		}
		q = q.Where(
			"moments.publication_status='public' AND moments.moderation_status<>'rejected' AND media_assets.state='available' AND media_assets.retention_due_at>? AND users.deleted_at IS NULL AND users.onboarding_complete=TRUE AND users.role='DEFAULT' AND users.group_id=?",
			now,
			*groupID,
		)
	default:
		return nil, appErrors.ErrConflict
	}
	if cursor != nil {
		q = q.Where(
			"moments.captured_at < ? OR (moments.captured_at = ? AND moments.id < ?)",
			cursor.CapturedAt,
			cursor.CapturedAt,
			cursor.ID,
		)
	}
	var rows []momentProjection
	if err := q.Order("moments.captured_at DESC").Order("moments.id DESC").Limit(momentPageLimit + 1).Find(&rows).Error; err != nil {
		return nil, handleRepositoryError(err)
	}
	hasNext := len(rows) > momentPageLimit
	if hasNext {
		rows = rows[:momentPageLimit]
	}
	items := make([]momentEntities.Moment, len(rows))
	for i := range rows {
		items[i] = *projectMoment(&rows[i])
	}
	return &momentEntities.Page{Items: items, HasNext: hasNext}, nil
}

func (r *MomentRepository) ToggleLike(
	ctx context.Context,
	momentID string,
	userID uint64,
	now time.Time,
) (bool, int, error) {
	var existing models.MomentLike
	err := r.getDB(ctx).Where("moment_id=? AND user_id=?", momentID, userID).Take(&existing).Error
	liked := false
	if err == nil {
		if deleteErr := r.getDB(ctx).Where("moment_id=? AND user_id=?", momentID, userID).Delete(&models.MomentLike{}).Error; deleteErr != nil {
			return false, 0, handleRepositoryError(deleteErr)
		}
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		if createErr := r.getDB(ctx).Create(&models.MomentLike{MomentID: momentID, UserID: userID, CreatedAt: now}).Error; createErr != nil {
			return false, 0, handleRepositoryError(createErr)
		}
		liked = true
	} else {
		return false, 0, handleRepositoryError(err)
	}
	var count int64
	if err := r.getDB(ctx).Model(&models.MomentLike{}).Where("moment_id=?", momentID).Count(&count).Error; err != nil {
		return false, 0, handleRepositoryError(err)
	}
	return liked, int(count), nil
}

func (r *MomentRepository) ListModeration(
	ctx context.Context,
	queue string,
	page uint64,
	now time.Time,
) (*momentEntities.ModerationPage, error) {
	q := projectionQuery(
		r.getDB(ctx),
		0,
	).Where("moments.publication_status='public' AND moments.moderation_status='pending' AND media_assets.state='available' AND media_assets.retention_due_at>? AND users.deleted_at IS NULL", now)
	if queue == "general" {
		q = q.Where("moments.origin='free'")
	} else {
		q = q.Where("moments.origin='challenge'")
	}
	var rows []momentProjection
	if err := q.Order("moments.captured_at ASC").Order("moments.id ASC").Limit(51).Offset(int(page) * 50).Find(&rows).Error; err != nil {
		return nil, handleRepositoryError(err)
	}
	has := len(rows) > 50
	if has {
		rows = rows[:50]
	}
	items := make([]momentEntities.Moment, len(rows))
	for i := range rows {
		items[i] = *projectMoment(&rows[i])
	}
	return &momentEntities.ModerationPage{Items: items, HasNext: has}, nil
}

func (r *MomentRepository) AwardMoment(
	ctx context.Context,
	momentID string,
	userID uint64,
	activityID string,
	points int,
	now time.Time,
) error {
	if points <= 0 {
		return r.getDB(ctx).
			Model(&models.Moment{}).
			Where("id=?", momentID).
			Update("reward_status", string(momentEntities.RewardDenied)).
			Error
	}
	entryID := uuid.NewString()
	result := r.getDB(ctx).
		Exec(`INSERT INTO point_entries (id,user_id,activity_id,activity_run_id,participation_id,moment_id,origin,reason,delta,created_at) SELECT ?,m.user_id,m.activity_id,NULL,m.participation_id,m.id,'moment','moment_challenge_award',?,? FROM moments m WHERE m.id=? AND m.user_id=? AND m.activity_id=? ON CONFLICT (moment_id,user_id,reason) WHERE moment_id IS NOT NULL DO NOTHING`, entryID, points, now, momentID, userID, activityID)
	if result.Error != nil {
		return handleRepositoryError(result.Error)
	}
	if result.RowsAffected != 1 {
		return appErrors.ErrConflict
	}
	if err := r.getDB(ctx).Model(&models.User{}).Where("id=?", userID).Update("points", gorm.Expr("points + ?", points)).Error; err != nil {
		return handleRepositoryError(err)
	}
	if err := r.getDB(ctx).
		Model(&models.Moment{}).
		Where("id=?", momentID).
		Updates(map[string]any{"reward_status": string(momentEntities.RewardAwarded), "points_awarded": points, "updated_at": now}).
		Error; err != nil {
		return handleRepositoryError(err)
	}
	if err := r.getDB(ctx).
		Model(&models.Participation{}).
		Where("id = (SELECT participation_id FROM moments WHERE id = ? AND user_id = ?)", momentID, userID).
		Update("can_share_moment", false).
		Error; err != nil {
		return handleRepositoryError(err)
	}
	return nil
}

func (r *MomentRepository) ReverseMomentAward(
	ctx context.Context,
	momentID string,
	userID uint64,
	now time.Time,
) (bool, error) {
	var row models.Moment
	if err := r.getDB(ctx).Where("id=? AND user_id=?", momentID, userID).Take(&row).Error; err != nil {
		return false, handleRepositoryError(err)
	}
	if row.RewardStatus == string(momentEntities.RewardReversed) {
		return false, nil
	}
	if row.RewardStatus != string(momentEntities.RewardAwarded) || row.PointsAwarded <= 0 {
		return false, appErrors.ErrConflict
	}
	var user models.User
	if err := r.getDB(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", userID).Take(&user).Error; err != nil {
		return false, handleRepositoryError(err)
	}
	if user.Points < row.PointsAwarded {
		return false, appErrors.ErrConflict
	}
	result := r.getDB(ctx).
		Exec(`INSERT INTO point_entries (id,user_id,activity_id,activity_run_id,participation_id,moment_id,origin,reason,delta,created_at) VALUES (?,?,?,?,?,?,?,?,?,?) ON CONFLICT (moment_id,user_id,reason) WHERE moment_id IS NOT NULL DO NOTHING`, uuid.NewString(), userID, row.ActivityID, nil, row.ParticipationID, row.ID, "moment", "moment_moderation_reversal", -row.PointsAwarded, now)
	if result.Error != nil {
		return false, handleRepositoryError(result.Error)
	}
	if result.RowsAffected != 1 {
		return false, nil
	}
	if err := r.getDB(ctx).Model(&models.User{}).Where("id=? AND points>=?", userID, row.PointsAwarded).Update("points", gorm.Expr("points - ?", row.PointsAwarded)).Error; err != nil {
		return false, handleRepositoryError(err)
	}
	if err := r.getDB(ctx).Model(&models.Moment{}).Where("id=?", momentID).Updates(map[string]any{"reward_status": string(momentEntities.RewardReversed), "updated_at": now}).Error; err != nil {
		return false, handleRepositoryError(err)
	}
	return true, nil
}

func (r *MomentRepository) ApplyModeration(
	ctx context.Context,
	momentID, action string,
	actor uint64,
	key string,
	now time.Time,
) (*momentEntities.Moment, *mediaEntities.Asset, bool, error) {
	var ref struct{ MediaAssetID string }
	if err := r.getDB(ctx).Table("moments").Select("media_asset_id").Where("id=?", momentID).Take(&ref).Error; err != nil {
		return nil, nil, false, handleRepositoryError(err)
	}
	var asset models.MediaAsset
	if err := r.getDB(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", ref.MediaAssetID).Take(&asset).Error; err != nil {
		return nil, nil, false, handleRepositoryError(err)
	}
	var row models.Moment
	if err := r.getDB(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", momentID).Take(&row).Error; err != nil {
		return nil, nil, false, handleRepositoryError(err)
	}
	if row.ParticipationID != nil {
		var p models.Participation
		if err := r.getDB(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", *row.ParticipationID).Take(&p).Error; err != nil {
			return nil, nil, false, handleRepositoryError(err)
		}
	}
	changed := false
	assetJustDeleted := false
	if action == "deny_points" {
		if row.RewardStatus == string(momentEntities.RewardAwarded) {
			reversed, err := r.ReverseMomentAward(ctx, row.ID, row.UserID, now)
			if err != nil {
				return nil, nil, false, err
			}
			changed = changed || reversed
			row.RewardStatus = string(momentEntities.RewardReversed)
		} else if row.RewardStatus != string(momentEntities.RewardReversed) {
			return nil, nil, false, appErrors.ErrConflict
		}
	} else if action == "delete_photo" {
		if row.RewardStatus == string(momentEntities.RewardAwarded) {
			reversed, err := r.ReverseMomentAward(ctx, row.ID, row.UserID, now)
			if err != nil {
				return nil, nil, false, err
			}
			changed = changed || reversed
			row.RewardStatus = string(momentEntities.RewardReversed)
		}
		if asset.State != string(mediaEntities.AssetDeleted) {
			asset.State = string(mediaEntities.AssetDeleted)
			asset.DeletedAt = &now
			asset.UpdatedAt = now
			if err := r.getDB(ctx).Save(&asset).Error; err != nil {
				return nil, nil, false, handleRepositoryError(err)
			}
			changed = true
			assetJustDeleted = true
		}
	} else if action == "approve" {
		if row.ModerationStatus == string(momentEntities.ModerationRejected) {
			return nil, nil, false, appErrors.ErrConflict
		}
		if row.ModerationStatus != string(momentEntities.ModerationApproved) {
			row.ModerationStatus = string(momentEntities.ModerationApproved)
			row.UpdatedAt = now
			if err := r.getDB(ctx).Save(&row).Error; err != nil {
				return nil, nil, false, handleRepositoryError(err)
			}
			changed = true
			// Approval is intentionally non-interruptive. Rejection, deletion and
			// point reversal below remain the only moderation notification cases.
		}
		return mappers.MapMomentToEntity(&row), mappers.MapMediaAssetToEntity(&asset), changed, nil
	}
	moderationJustChanged := row.PublicationStatus != string(momentEntities.PublicationPrivate) ||
		row.ModerationStatus != string(momentEntities.ModerationRejected)
	if moderationJustChanged {
		row.PublicationStatus = string(momentEntities.PublicationPrivate)
		row.ModerationStatus = string(momentEntities.ModerationRejected)
		row.UpdatedAt = now
		if err := r.getDB(ctx).Save(&row).Error; err != nil {
			return nil, nil, false, handleRepositoryError(err)
		}
		changed = true
	}
	// A second decision (e.g. delete_photo after an earlier deny_points already
	// rejected the moment) still needs to reach the owner: the moderation status
	// transition above only fires once, but the photo can be deleted later.
	if moderationJustChanged || assetJustDeleted {
		body := "Sua foto não atendeu às regras de publicação."
		if action == "delete_photo" {
			body = "Sua foto foi removida da galeria."
		}
		if err := writeDerivedNotification(
			ctx, r.getDB(ctx), row.UserID, "moment_moderation",
			"Sua foto foi moderada", body, "moment", row.ID, now,
		); err != nil {
			return nil, nil, false, err
		}
	}
	return mappers.MapMomentToEntity(&row), mappers.MapMediaAssetToEntity(&asset), changed, nil
}

func (r *MomentRepository) CreateModerationDecision(
	ctx context.Context,
	item *momentEntities.ModerationDecision,
) (bool, error) {
	row := &models.MomentModerationDecision{
		ID:             item.ID,
		MomentID:       item.MomentID,
		ActorUserID:    item.ActorUserID,
		Action:         item.Action,
		IdempotencyKey: item.IdempotencyKey,
		CreatedAt:      item.CreatedAt,
	}
	result := r.getDB(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(row)
	if result.Error != nil {
		return false, handleRepositoryError(result.Error)
	}
	return result.RowsAffected == 1, nil
}
