package repositories

import (
	"context"
	"encoding/json"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	specialEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/specialevent/entities"
	specialInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/specialevent/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SpecialEventRepository struct {
	*BaseRepository[models.SpecialEvent]
}

func NewSpecialEventRepository(db *gorm.DB) specialInterfaces.Repository {
	return &SpecialEventRepository{BaseRepository: NewBaseRepository[models.SpecialEvent](db)}
}

func specialEventEntity(row *models.SpecialEvent) *specialEntities.Event {
	if row == nil {
		return nil
	}
	targets := []string{}
	_ = json.Unmarshal(row.Targets, &targets)
	return &specialEntities.Event{ID: row.ID, ActivityID: row.ActivityID, ActivityRunID: row.ActivityRunID, Title: row.Title, Description: row.Description, Points: row.Points, DurationMinutes: row.DurationMinutes, Targets: targets, Status: specialEntities.Status(row.Status), TeaserAt: row.TeaserAt, EndsAt: row.EndsAt, QRToken: row.QRToken, QRExpiresAt: row.QRExpiresAt, CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
}
func specialEventModel(event *specialEntities.Event) *models.SpecialEvent {
	targets, _ := json.Marshal(event.Targets)
	return &models.SpecialEvent{ID: event.ID, ActivityID: event.ActivityID, ActivityRunID: event.ActivityRunID, Title: event.Title, Description: event.Description, Points: event.Points, DurationMinutes: event.DurationMinutes, Targets: targets, Status: string(event.Status), TeaserAt: event.TeaserAt, EndsAt: event.EndsAt, QRToken: event.QRToken, QRExpiresAt: event.QRExpiresAt, CreatedBy: event.CreatedBy, CreatedAt: event.CreatedAt, UpdatedAt: event.UpdatedAt}
}
func (r *SpecialEventRepository) Create(ctx context.Context, event *specialEntities.Event) error {
	return r.BaseRepository.Create(ctx, specialEventModel(event))
}
func (r *SpecialEventRepository) Save(ctx context.Context, event *specialEntities.Event) error {
	return r.BaseRepository.Update(ctx, specialEventModel(event))
}
func (r *SpecialEventRepository) ListForManager(ctx context.Context, userID uint64, global bool) ([]specialEntities.Event, error) {
	q := r.getDB(ctx).Model(&models.SpecialEvent{}).Where("status <> 'closed'")
	if !global {
		q = q.Joins("JOIN activity_manager_assignments ON activity_manager_assignments.activity_id = special_events.activity_id AND activity_manager_assignments.user_id = ?", userID)
	}
	var rows []models.SpecialEvent
	if err := q.Order("created_at DESC").Find(&rows).Error; err != nil {
		return nil, handleRepositoryError(err)
	}
	result := make([]specialEntities.Event, len(rows))
	for i := range rows {
		result[i] = *specialEventEntity(&rows[i])
	}
	return result, nil
}
func (r *SpecialEventRepository) FindForManager(ctx context.Context, id string, userID uint64, global, lock bool) (*specialEntities.Event, error) {
	q := r.getDB(ctx).Model(&models.SpecialEvent{}).Where("special_events.id = ?", id)
	if !global {
		q = q.Joins("JOIN activity_manager_assignments ON activity_manager_assignments.activity_id = special_events.activity_id AND activity_manager_assignments.user_id = ?", userID)
	}
	if lock {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var row models.SpecialEvent
	if err := q.Take(&row).Error; err != nil {
		return nil, handleRepositoryError(err)
	}
	return specialEventEntity(&row), nil
}
func (r *SpecialEventRepository) FindVisible(ctx context.Context, target string, now time.Time) (*specialEntities.Event, error) {
	var row models.SpecialEvent
	// JSONB containment works on PostgreSQL and CockroachDB without conflicting
	// with GORM's `?` bind placeholders. Encode the target as a JSON string array.
	targets, _ := json.Marshal([]string{target})
	err := r.getDB(ctx).Where("status IN ('teaser','active') AND ends_at > ? AND targets @> ?::jsonb", now.UTC(), string(targets)).Order("updated_at DESC").Take(&row).Error
	if err != nil {
		return nil, handleRepositoryError(err)
	}
	return specialEventEntity(&row), nil
}

var _ = appErrors.ErrNotFound
