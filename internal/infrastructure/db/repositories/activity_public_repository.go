package repositories

import (
	"context"
	"time"

	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	activityEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/activity/entities"
	spaceEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/space/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
	"gorm.io/gorm"
)

type publicActivityRow struct {
	ID              string     `gorm:"column:id"`
	SpaceID         *string    `gorm:"column:space_id"`
	Slug            string     `gorm:"column:slug"`
	Name            string     `gorm:"column:name"`
	Description     *string    `gorm:"column:description"`
	Kind            string     `gorm:"column:kind"`
	Status          string     `gorm:"column:status"`
	StartsAt        *time.Time `gorm:"column:starts_at"`
	EndsAt          *time.Time `gorm:"column:ends_at"`
	ActualStartedAt *time.Time `gorm:"column:actual_started_at"`
	FlexMinutes     int        `gorm:"column:flex_minutes"`
	CheckInPoints   int        `gorm:"column:check_in_points"`
	MomentPoints    int        `gorm:"column:moment_points"`
	CooldownSeconds int        `gorm:"column:cooldown_seconds"`
	AllowsMoment    bool       `gorm:"column:allows_moment"`
	CreatedAt       time.Time  `gorm:"column:created_at"`
	UpdatedAt       time.Time  `gorm:"column:updated_at"`
	JoinedSpaceID   *string    `gorm:"column:joined_space_id"`
	SpaceSlug       *string    `gorm:"column:space_slug"`
	SpaceName       *string    `gorm:"column:space_name"`
	MapReference    *string    `gorm:"column:space_map_reference"`
	SpaceCreatedAt  *time.Time `gorm:"column:space_created_at"`
	SpaceUpdatedAt  *time.Time `gorm:"column:space_updated_at"`
}

const publicActivityProjection = `activities.id, activities.space_id, activities.slug, activities.name,
	activities.description, activities.kind, activities.status, activities.starts_at, activities.ends_at,
	activities.actual_started_at, activities.flex_minutes, activities.check_in_points, activities.moment_points, activities.cooldown_seconds, activities.allows_moment,
	activities.created_at, activities.updated_at, spaces.id AS joined_space_id, spaces.slug AS space_slug,
	spaces.name AS space_name, spaces.map_reference AS space_map_reference,
	spaces.created_at AS space_created_at, spaces.updated_at AS space_updated_at`

func mapPublicActivityRow(row *publicActivityRow) activityEntities.PublicActivity {
	activity := activityEntities.Activity{ID: row.ID, SpaceID: row.SpaceID, Slug: row.Slug, Name: row.Name, Description: row.Description, Kind: activityEntities.Kind(row.Kind), Status: activityEntities.Status(row.Status), StartsAt: row.StartsAt, EndsAt: row.EndsAt, ActualStartedAt: row.ActualStartedAt, FlexMinutes: row.FlexMinutes, CheckInPoints: row.CheckInPoints, MomentPoints: row.MomentPoints, CooldownSeconds: row.CooldownSeconds, AllowsMoment: row.AllowsMoment, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
	var space *spaceEntities.Space
	if row.JoinedSpaceID != nil && row.SpaceSlug != nil && row.SpaceName != nil {
		space = &spaceEntities.Space{ID: *row.JoinedSpaceID, Slug: *row.SpaceSlug, Name: *row.SpaceName, MapReference: row.MapReference}
		if row.SpaceCreatedAt != nil {
			space.CreatedAt = *row.SpaceCreatedAt
		}
		if row.SpaceUpdatedAt != nil {
			space.UpdatedAt = *row.SpaceUpdatedAt
		}
	}
	return activityEntities.PublicActivity{Activity: activity, Space: space}
}

func publicActivityQuery(db *gorm.DB) *gorm.DB {
	return db.Model(&models.Activity{}).Select(publicActivityProjection).Joins("LEFT JOIN spaces ON spaces.id = activities.space_id")
}

func publiclyVisibleActivities(query *gorm.DB, generatedAt time.Time) *gorm.DB {
	return query.Where(`activities.status IN ('active','paused','completed') OR
		(activities.kind = 'schedule' AND activities.status = 'draft' AND activities.starts_at > ?
		 AND activities.ends_at IS NOT NULL AND activities.starts_at < activities.ends_at)`, generatedAt.UTC())
}

func orderPublicActivities(query *gorm.DB) *gorm.DB {
	return query.Order("activities.starts_at ASC NULLS LAST").Order("activities.name ASC").Order("activities.id ASC")
}

func (r *ActivityRepository) ListSchedule(ctx context.Context, sectorSlug string, limit int) ([]activityEntities.PublicActivity, error) {
	query := publicActivityQuery(r.getDB(ctx)).Where("activities.kind = ? AND activities.status <> ? AND activities.starts_at IS NOT NULL AND activities.ends_at IS NOT NULL AND activities.starts_at < activities.ends_at", string(activityEntities.KindSchedule), string(activityEntities.StatusArchived))
	if sectorSlug != "" {
		query = query.Where("spaces.slug = ?", sectorSlug)
	}
	query = orderPublicActivities(query)
	if limit > 0 {
		query = query.Limit(limit)
	}
	var rows []publicActivityRow
	if err := query.Find(&rows).Error; err != nil {
		return nil, handleRepositoryError(err)
	}
	items := make([]activityEntities.PublicActivity, len(rows))
	for index := range rows {
		items[index] = mapPublicActivityRow(&rows[index])
	}
	return items, nil
}

func (r *ActivityRepository) ListManagerSchedule(ctx context.Context, actorUserID uint64, global bool) ([]activityEntities.PublicActivity, error) {
	query := publicActivityQuery(r.getDB(ctx)).Where("activities.kind = ? AND activities.status <> ? AND activities.starts_at IS NOT NULL AND activities.ends_at IS NOT NULL AND activities.starts_at < activities.ends_at", string(activityEntities.KindSchedule), string(activityEntities.StatusArchived))
	if !global {
		query = query.Joins("JOIN activity_manager_assignments ON activity_manager_assignments.activity_id = activities.id AND activity_manager_assignments.user_id = ?", actorUserID)
	}
	query = orderPublicActivities(query)
	var rows []publicActivityRow
	if err := query.Find(&rows).Error; err != nil {
		return nil, handleRepositoryError(err)
	}
	items := make([]activityEntities.PublicActivity, len(rows))
	for index := range rows {
		items[index] = mapPublicActivityRow(&rows[index])
	}
	return items, nil
}

func (r *ActivityRepository) ListPublic(ctx context.Context, kind *activityEntities.Kind, spaceID *string, generatedAt time.Time, page uint64) (*messages.PaginatedResponse[activityEntities.PublicActivity], error) {
	const limit = 10
	query := publiclyVisibleActivities(publicActivityQuery(r.getDB(ctx)), generatedAt)
	if kind != nil {
		query = query.Where("activities.kind = ?", string(*kind))
	}
	if spaceID != nil {
		query = query.Where("activities.space_id = ?", *spaceID)
	}
	query = orderPublicActivities(query).Limit(limit + 1).Offset(int(page) * limit)
	var rows []publicActivityRow
	if err := query.Find(&rows).Error; err != nil {
		return nil, handleRepositoryError(err)
	}
	hasNext := len(rows) > limit
	if hasNext {
		rows = rows[:limit]
	}
	items := make([]activityEntities.PublicActivity, len(rows))
	for index := range rows {
		items[index] = mapPublicActivityRow(&rows[index])
	}
	return &messages.PaginatedResponse[activityEntities.PublicActivity]{Data: items, Pagination: messages.Pagination{CurrentPage: messages.Uint64StringFromUint64(page + 1), HasNextPage: hasNext, Limit: limit}}, nil
}

func (r *ActivityRepository) FindPublicByID(ctx context.Context, activityID string, generatedAt time.Time) (*activityEntities.PublicActivity, error) {
	var row publicActivityRow
	query := publiclyVisibleActivities(publicActivityQuery(r.getDB(ctx)).Where("activities.id = ?", activityID), generatedAt)
	if err := query.Take(&row).Error; err != nil {
		return nil, handleRepositoryError(err)
	}
	item := mapPublicActivityRow(&row)
	return &item, nil
}
