package mappers

import (
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	spaceEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/space/entities"
)

func MapSpaceToResponseDTO(space *spaceEntities.Space) *messages.SpaceResponseDTO {
	if space == nil {
		return nil
	}
	return &messages.SpaceResponseDTO{ID: space.ID, Name: space.Name, Slug: space.Slug, MapReference: space.MapReference}
}
