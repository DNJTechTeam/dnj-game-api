package mappers

import (
	"github.com/dnjtechteam/dnj-game-api/internal/domain/activity/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
)

func MapActivityToEntity(model *models.Activity) *entities.Activity {
	if model == nil {
		return nil
	}
	return &entities.Activity{ID: model.ID, SpaceID: model.SpaceID, Slug: model.Slug, Name: model.Name, Description: model.Description, Kind: entities.Kind(model.Kind), Status: entities.Status(model.Status), StartsAt: model.StartsAt, EndsAt: model.EndsAt, ActualStartedAt: model.ActualStartedAt, FlexMinutes: model.FlexMinutes, CheckInPoints: model.CheckInPoints, MomentPoints: model.MomentPoints, CooldownSeconds: model.CooldownSeconds, AllowsMoment: model.AllowsMoment, CreatedAt: model.CreatedAt, UpdatedAt: model.UpdatedAt}
}

func MapActivityEntityToModel(entity *entities.Activity) *models.Activity {
	if entity == nil {
		return nil
	}
	return &models.Activity{ID: entity.ID, SpaceID: entity.SpaceID, Slug: entity.Slug, Name: entity.Name, Description: entity.Description, Kind: string(entity.Kind), Status: string(entity.Status), StartsAt: entity.StartsAt, EndsAt: entity.EndsAt, ActualStartedAt: entity.ActualStartedAt, FlexMinutes: entity.FlexMinutes, CheckInPoints: entity.CheckInPoints, MomentPoints: entity.MomentPoints, CooldownSeconds: entity.CooldownSeconds, AllowsMoment: entity.AllowsMoment, CreatedAt: entity.CreatedAt, UpdatedAt: entity.UpdatedAt}
}
