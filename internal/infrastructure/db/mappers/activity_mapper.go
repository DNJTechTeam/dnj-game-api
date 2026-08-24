package mappers

import (
	"github.com/dnjtechteam/dnj-game-api/internal/domain/activity/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
)

func MapActivityToEntity(model *models.Activity) *entities.Activity {
	if model == nil {
		return nil
	}
	return &entities.Activity{ID: model.ID, SpaceID: model.SpaceID, Slug: model.Slug, Name: model.Name, Description: model.Description, Kind: entities.Kind(model.Kind), Status: entities.Status(model.Status), StartsAt: model.StartsAt, EndsAt: model.EndsAt, CheckInPoints: model.CheckInPoints, MomentPoints: model.MomentPoints, CooldownSeconds: model.CooldownSeconds, AllowsMoment: model.AllowsMoment, CreatedAt: model.CreatedAt, UpdatedAt: model.UpdatedAt}
}
