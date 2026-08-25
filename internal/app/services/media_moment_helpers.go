package services

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	mediaEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/media/entities"
	mediaInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/media/interfaces"
	userEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	userInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/user/interfaces"
	"github.com/google/uuid"
)

var errIdempotencyRace = errors.New("idempotency operation concurrently reserved")

func mediaMomentError(status int, code string, message string) error {
	return appErrors.NewAPIServiceError(status, code, message, nil)
}

func parseIdempotencyKey(raw string) (string, error) {
	key, err := uuid.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", mediaMomentError(
			http.StatusBadRequest,
			"INVALID_REQUEST",
			"Idempotency-Key deve ser um UUID válido.",
		)
	}
	return key.String(), nil
}

func requireDefaultActor(
	ctx context.Context,
	users userInterfaces.UserRepositoryInterface,
	lock bool,
) (*userEntities.User, error) {
	id, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}

	var user *userEntities.User
	if lock {
		user, err = users.FindByIDForUpdate(ctx, id)
	} else {
		user, err = users.FindByID(ctx, id)
	}
	if errors.Is(err, appErrors.ErrNotFound) {
		return nil, mediaMomentError(
			http.StatusUnauthorized,
			"UNAUTHENTICATED",
			"Autenticação necessária.",
		)
	}
	if err != nil {
		return nil, appErrors.InternalError
	}
	if user == nil {
		return nil, mediaMomentError(
			http.StatusUnauthorized,
			"UNAUTHENTICATED",
			"Autenticação necessária.",
		)
	}
	if !user.OnboardingComplete {
		return nil, mediaMomentError(
			http.StatusConflict,
			"ONBOARDING_REQUIRED",
			"Conclua o onboarding antes de continuar.",
		)
	}
	if user.Role != userEntities.RoleDefault {
		return nil, mediaMomentError(
			http.StatusForbidden,
			"FORBIDDEN",
			"Operação não permitida para este papel.",
		)
	}
	return user, nil
}

func requireAdminActor(
	ctx context.Context,
	users userInterfaces.UserRepositoryInterface,
) (*userEntities.User, error) {
	id, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}

	user, err := users.FindByID(ctx, id)
	if errors.Is(err, appErrors.ErrNotFound) {
		return nil, mediaMomentError(
			http.StatusUnauthorized,
			"UNAUTHENTICATED",
			"Autenticação necessária.",
		)
	}
	if err != nil {
		return nil, appErrors.InternalError
	}
	if user == nil {
		return nil, mediaMomentError(
			http.StatusUnauthorized,
			"UNAUTHENTICATED",
			"Autenticação necessária.",
		)
	}
	if user.Role != userEntities.RoleAdmin {
		return nil, mediaMomentError(
			http.StatusForbidden,
			"FORBIDDEN",
			"Operação restrita a ADMIN.",
		)
	}
	return user, nil
}

func findIdempotencyOperation(
	ctx context.Context,
	repository mediaInterfaces.Repository,
	actorID uint64,
	key string,
	operation string,
	fingerprint string,
) (*mediaEntities.Operation, error) {
	prior, err := repository.FindOperation(ctx, actorID, key)
	if err == nil {
		if prior.Operation != operation || prior.IntentHash != fingerprint {
			return nil, mediaMomentError(
				http.StatusConflict,
				"IDEMPOTENCY_KEY_REUSED",
				"Idempotency-Key já foi usada em outra intenção.",
			)
		}
		return prior, nil
	}
	if !errors.Is(err, appErrors.ErrNotFound) {
		return nil, appErrors.InternalError
	}

	exists, err := repository.FindLegacyOperation(ctx, actorID, key)
	if err != nil {
		return nil, appErrors.InternalError
	}
	if exists {
		return nil, mediaMomentError(
			http.StatusConflict,
			"IDEMPOTENCY_KEY_REUSED",
			"Idempotency-Key já foi usada em outra intenção.",
		)
	}
	return nil, nil
}

func createIdempotencyOperation(
	ctx context.Context,
	repository mediaInterfaces.Repository,
	operation *mediaEntities.Operation,
) error {
	err := repository.CreateOperation(ctx, operation)
	if errors.Is(err, appErrors.ErrConflict) {
		return errIdempotencyRace
	}
	if err != nil {
		return appErrors.InternalError
	}
	return nil
}

type operationErrorSnapshot struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func replayOperationError(operation *mediaEntities.Operation) error {
	var snapshot operationErrorSnapshot
	if json.Unmarshal(operation.ResponseSnapshot, &snapshot) != nil || snapshot.Code == "" {
		return appErrors.InternalError
	}
	return mediaMomentError(operation.HTTPStatus, snapshot.Code, snapshot.Message)
}

func secureChecksumEqual(left string, right string) bool {
	leftBytes, leftErr := base64.StdEncoding.DecodeString(left)
	rightBytes, rightErr := base64.StdEncoding.DecodeString(right)
	return leftErr == nil &&
		rightErr == nil &&
		len(leftBytes) == 32 &&
		len(rightBytes) == 32 &&
		subtle.ConstantTimeCompare(leftBytes, rightBytes) == 1
}

func utcNow(now func() time.Time) time.Time {
	return now().UTC()
}
