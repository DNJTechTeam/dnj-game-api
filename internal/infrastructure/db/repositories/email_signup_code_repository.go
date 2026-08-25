package repositories

import (
	"context"
	"errors"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/domain/emailsignupcode/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/domain/emailsignupcode/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/mappers"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type EmailSignupCodeRepository struct {
	*BaseRepository[models.EmailSignupCode]
}

func NewEmailSignupCodeRepository(db *gorm.DB) interfaces.EmailSignupCodeRepositoryInterface {
	return &EmailSignupCodeRepository{BaseRepository: NewBaseRepository[models.EmailSignupCode](db)}
}

// FindByEmailForUpdate locks the row so a resend and a verify racing on the
// same email serialize instead of interleaving the cooldown/attempts check.
func (r *EmailSignupCodeRepository) FindByEmailForUpdate(ctx context.Context, email string) (*entities.EmailSignupCode, error) {
	var model models.EmailSignupCode
	err := r.getDB(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("email = ?", email).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, appErrors.InternalError
	}
	return mappers.MapEmailSignupCodeToEntity(&model), nil
}

func (r *EmailSignupCodeRepository) Create(ctx context.Context, code *entities.EmailSignupCode) (*entities.EmailSignupCode, error) {
	model := mappers.MapEntityToEmailSignupCode(code)
	if err := r.BaseRepository.Create(ctx, model); err != nil {
		return nil, err
	}
	return mappers.MapEmailSignupCodeToEntity(model), nil
}

func (r *EmailSignupCodeRepository) Update(ctx context.Context, code *entities.EmailSignupCode) (*entities.EmailSignupCode, error) {
	model := mappers.MapEntityToEmailSignupCode(code)
	if err := r.BaseRepository.Update(ctx, model); err != nil {
		return nil, err
	}
	return mappers.MapEmailSignupCodeToEntity(model), nil
}
