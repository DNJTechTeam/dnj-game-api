package services

import (
	"context"
	"strings"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/app/mappers"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	groupInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/group/interfaces"
)

const groupSearchLimit = 20

type GroupService struct {
	*BaseService
	groupRepository groupInterfaces.GroupRepositoryInterface
}

func NewGroupService(
	baseService *BaseService,
	groupRepository groupInterfaces.GroupRepositoryInterface,
) interfaces.GroupServiceInterface {
	return &GroupService{
		BaseService:     baseService,
		groupRepository: groupRepository,
	}
}

func (s *GroupService) Search(ctx context.Context, query string) ([]*messages.GroupSummaryDTO, error) {
	trimmed := strings.TrimSpace(query)
	if len(trimmed) < 3 {
		return nil, appErrors.NewError("Erro ao buscar grupos.", []*appErrors.FieldError{
			appErrors.NewFieldError("search", "o termo de busca deve ter pelo menos 3 caracteres"),
		})
	}

	groups, err := s.groupRepository.Search(ctx, trimmed, groupSearchLimit)
	if err != nil {
		return nil, appErrors.InternalError
	}

	summaries := make([]*messages.GroupSummaryDTO, len(groups))
	for i, group := range groups {
		summaries[i] = mappers.MapGroupToSummaryDTO(group)
	}
	return summaries, nil
}
