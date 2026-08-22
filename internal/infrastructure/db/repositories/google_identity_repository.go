package repositories

import (
	"context"
	"errors"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/domain/identity/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/domain/identity/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/mappers"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
	"gorm.io/gorm"
)

type GoogleIdentityRepository struct {
	*BaseRepository[models.GoogleIdentity]
}

func NewGoogleIdentityRepository(db *gorm.DB) interfaces.GoogleIdentityRepositoryInterface {
	return &GoogleIdentityRepository{BaseRepository: NewBaseRepository[models.GoogleIdentity](db)}
}

func (r *GoogleIdentityRepository) Create(ctx context.Context, identity *entities.GoogleIdentity) (*entities.GoogleIdentity, error) {
	model := mappers.MapEntityToGoogleIdentity(identity)
	if err := r.BaseRepository.Create(ctx, model); err != nil {
		return nil, err
	}
	return mappers.MapGoogleIdentityToEntity(model), nil
}

func (r *GoogleIdentityRepository) FindByProviderAndSubject(ctx context.Context, provider, subject string) (*entities.GoogleIdentity, error) {
	model, err := r.BaseRepository.FindOneBy(ctx, map[string]any{"provider": provider, "subject": subject})
	if errors.Is(err, appErrors.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return mappers.MapGoogleIdentityToEntity(model), nil
}
