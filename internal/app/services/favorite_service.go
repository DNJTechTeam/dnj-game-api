package services

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	appInterfaces "github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
	appMappers "github.com/dnjtechteam/dnj-game-api/internal/app/mappers"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	activityInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/activity/interfaces"
	favoriteEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/favorite/entities"
	favoriteInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/favorite/interfaces"
	userEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	userInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/user/interfaces"
	"github.com/google/uuid"
)

const (
	favoritePutOperation    = "favorite.put"
	favoriteDeleteOperation = "favorite.delete"
)

type FavoriteService struct {
	*BaseService
	favorites  favoriteInterfaces.FavoriteRepositoryInterface
	activities activityInterfaces.ActivityRepositoryInterface
	users      userInterfaces.UserRepositoryInterface
	now        func() time.Time
}

func NewFavoriteService(base *BaseService, favorites favoriteInterfaces.FavoriteRepositoryInterface, activities activityInterfaces.ActivityRepositoryInterface, users userInterfaces.UserRepositoryInterface) appInterfaces.FavoriteServiceInterface {
	return &FavoriteService{BaseService: base, favorites: favorites, activities: activities, users: users, now: time.Now}
}

func favoriteError(status int, code, message string) error {
	return appErrors.NewAPIServiceError(status, code, message, nil)
}

func ensureFavoriteActor(user *userEntities.User, err error) error {
	if errors.Is(err, appErrors.ErrNotFound) {
		return favoriteError(http.StatusUnauthorized, "UNAUTHENTICATED", "Autenticação necessária.")
	}
	if err != nil || user == nil {
		return appErrors.InternalError
	}
	if !user.OnboardingComplete {
		return favoriteError(http.StatusConflict, "ONBOARDING_REQUIRED", "Conclua o onboarding antes de usar favoritos.")
	}
	return nil
}

func (s *FavoriteService) List(ctx context.Context, filter *messages.ListFavoritesFilterDTO) (*messages.PaginatedResponse[messages.PublicActivityResponseDTO], error) {
	if filter == nil {
		return nil, favoriteError(http.StatusBadRequest, "INVALID_REQUEST", "Filtros inválidos.")
	}
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	user, findErr := s.users.FindByID(ctx, userID)
	if actorErr := ensureFavoriteActor(user, findErr); actorErr != nil {
		return nil, actorErr
	}
	generatedAt := s.now().UTC()
	result, err := s.favorites.ListVisible(ctx, userID, generatedAt, filter.GetPage())
	if err != nil {
		return nil, appErrors.InternalError
	}
	data := make([]messages.PublicActivityResponseDTO, len(result.Data))
	for index := range result.Data {
		data[index] = *appMappers.MapPublicActivityToResponseDTO(&result.Data[index], deriveActivityState(&result.Data[index].Activity, generatedAt))
	}
	return &messages.PaginatedResponse[messages.PublicActivityResponseDTO]{Data: data, Pagination: result.Pagination}, nil
}

func favoriteIntent(operation, activityID string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(operation+"\n"+activityID)))
}

func (s *FavoriteService) write(ctx context.Context, rawActivityID, rawKey, operation string) error {
	key, err := uuid.Parse(rawKey)
	if err != nil {
		return favoriteError(http.StatusBadRequest, "INVALID_REQUEST", "Idempotency-Key deve ser um UUID válido.")
	}
	activity, err := uuid.Parse(rawActivityID)
	if err != nil {
		if operation == favoriteDeleteOperation {
			return favoriteError(http.StatusBadRequest, "INVALID_REQUEST", "activityId deve ser um UUID válido.")
		}
		return favoriteError(http.StatusNotFound, "NOT_FOUND", "Atividade não encontrada.")
	}
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return err
	}
	activityID := activity.String()
	keyID := key.String()
	intent := favoriteIntent(operation, activityID)
	return s.WithTransaction(ctx, func(txCtx context.Context) error {
		user, findErr := s.users.FindByIDForUpdate(txCtx, userID)
		if actorErr := ensureFavoriteActor(user, findErr); actorErr != nil {
			return actorErr
		}
		prior, priorErr := s.favorites.FindOperation(txCtx, userID, keyID)
		if priorErr == nil {
			if prior.Operation != operation || prior.ActivityID != activityID || prior.IntentHash != intent {
				return favoriteError(http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "Idempotency-Key já foi usado em outra intenção.")
			}
			return nil
		}
		if !errors.Is(priorErr, appErrors.ErrNotFound) {
			return appErrors.InternalError
		}
		now := s.now().UTC()
		if operation == favoritePutOperation {
			if _, visibilityErr := s.activities.FindPublicByID(txCtx, activityID, now); errors.Is(visibilityErr, appErrors.ErrNotFound) {
				return favoriteError(http.StatusNotFound, "NOT_FOUND", "Atividade não encontrada.")
			} else if visibilityErr != nil {
				return appErrors.InternalError
			}
			if _, createErr := s.favorites.Create(txCtx, &favoriteEntities.Favorite{UserID: userID, ActivityID: activityID, CreatedAt: now}); createErr != nil {
				return appErrors.InternalError
			}
		} else {
			if _, deleteErr := s.favorites.Delete(txCtx, userID, activityID); deleteErr != nil {
				return appErrors.InternalError
			}
		}
		_, createErr := s.favorites.CreateOperation(txCtx, &favoriteEntities.ParticipantOperation{ID: uuid.NewString(), ActorUserID: userID, IdempotencyKey: keyID, Operation: operation, ActivityID: activityID, IntentHash: intent, HTTPStatus: http.StatusNoContent, CreatedAt: now})
		if errors.Is(createErr, appErrors.ErrConflict) {
			return favoriteError(http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "Idempotency-Key já foi usado em outra intenção.")
		}
		if createErr != nil {
			return appErrors.InternalError
		}
		return nil
	})
}

func (s *FavoriteService) Put(ctx context.Context, activityID, idempotencyKey string) error {
	return s.write(ctx, activityID, idempotencyKey, favoritePutOperation)
}

func (s *FavoriteService) Delete(ctx context.Context, activityID, idempotencyKey string) error {
	return s.write(ctx, activityID, idempotencyKey, favoriteDeleteOperation)
}
