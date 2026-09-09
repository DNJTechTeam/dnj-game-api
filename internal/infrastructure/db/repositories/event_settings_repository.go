package repositories

import (
	"context"
	"time"

	entities "github.com/dnjtechteam/dnj-game-api/internal/domain/eventsettings"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
	"gorm.io/gorm"
)

const eventSettingsID = "global"

type EventSettingsRepository struct {
	db *gorm.DB
}

func NewEventSettingsRepository(db *gorm.DB) *EventSettingsRepository {
	return &EventSettingsRepository{db: db}
}

func (r *EventSettingsRepository) Get(ctx context.Context) (*entities.EventSettings, error) {
	var model models.EventSettings
	err := r.db.WithContext(ctx).Where("id = ?", eventSettingsID).First(&model).Error
	if err == gorm.ErrRecordNotFound {
		return &entities.EventSettings{
			ID:            eventSettingsID,
			ScoringClosed: false,
			CreatedAt:     time.Now().UTC(),
			UpdatedAt:     time.Now().UTC(),
		}, nil
	}
	if err != nil {
		return nil, err
	}
	return &entities.EventSettings{
		ID:            model.ID,
		ScoringClosed: model.ScoringClosed,
		ClosedAt:      model.ClosedAt,
		ClosedBy:      model.ClosedBy,
		AuditNote:     model.AuditNote,
		CreatedAt:     model.CreatedAt,
		UpdatedAt:     model.UpdatedAt,
	}, nil
}

func (r *EventSettingsRepository) SetScoringClosed(ctx context.Context, closed bool, closedBy *uint64, auditNote string) error {
	now := time.Now().UTC()
	var model models.EventSettings
	err := r.db.WithContext(ctx).Where("id = ?", eventSettingsID).First(&model).Error
	if err == gorm.ErrRecordNotFound {
		model = models.EventSettings{
			ID:            eventSettingsID,
			ScoringClosed: closed,
			ClosedAt:      &now,
			ClosedBy:      closedBy,
			AuditNote:     auditNote,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		return r.db.WithContext(ctx).Create(&model).Error
	}
	if err != nil {
		return err
	}
	model.ScoringClosed = closed
	model.UpdatedAt = now
	if closed {
		model.ClosedAt = &now
		model.ClosedBy = closedBy
		model.AuditNote = auditNote
	} else {
		model.ClosedAt = nil
		model.ClosedBy = nil
		model.AuditNote = ""
	}
	return r.db.WithContext(ctx).Save(&model).Error
}
