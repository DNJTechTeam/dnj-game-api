package services

import (
	"context"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	appInterfaces "github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
	appMappers "github.com/dnjtechteam/dnj-game-api/internal/app/mappers"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	spaceInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/space/interfaces"
)

type SpaceService struct {
	spaces spaceInterfaces.SpaceRepositoryInterface
}

func NewSpaceService(spaces spaceInterfaces.SpaceRepositoryInterface) appInterfaces.SpaceServiceInterface {
	return &SpaceService{spaces: spaces}
}

func (s *SpaceService) List(ctx context.Context, filter *messages.ListSpacesFilterDTO) (*messages.PaginatedResponse[messages.SpaceResponseDTO], error) {
	result, err := s.spaces.List(ctx, filter.GetPage())
	if err != nil {
		return nil, appErrors.InternalError
	}
	items := make([]messages.SpaceResponseDTO, len(result.Data))
	for index := range result.Data {
		items[index] = *appMappers.MapSpaceToResponseDTO(&result.Data[index])
	}
	return &messages.PaginatedResponse[messages.SpaceResponseDTO]{Data: items, Pagination: result.Pagination}, nil
}
