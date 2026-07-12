package mappers

import (
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	"github.com/dnjtechteam/dnj-game-api/internal/domain/group/entities"
)

func MapGroupToSummaryDTO(group *entities.Group) *messages.GroupSummaryDTO {
	if group == nil {
		return nil
	}

	return &messages.GroupSummaryDTO{
		ID:        messages.Uint64StringFromUint64(group.ID),
		GroupName: group.Name,
	}
}
