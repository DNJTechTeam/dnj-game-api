package services

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"testing"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	userEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/common"
	"github.com/dnjtechteam/dnj-game-api/internal/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ctxWithUser(id uint64) context.Context {
	return context.WithValue(context.Background(), common.UserIDContextKey, strconv.FormatUint(id, 10))
}

// TestMediaMoments_AuthHelperDefensiveBranches exercises the auth-helper error paths that a
// real Postgres-backed integration test cannot reliably force: a generic (non-not-found)
// repository failure while resolving the actor, and a lookup that comes back with a nil user.
// Both must fail closed (never fall through as authenticated) without leaking the underlying
// repository error to the caller.
func TestMediaMoments_AuthHelperDefensiveBranches(t *testing.T) {
	ctx := ctxWithUser(42)

	t.Run("requireDefaultActor generic repository failure is redacted", func(t *testing.T) {
		users := mocks.NewMockUserRepositoryInterface(t)
		users.On("FindByID", ctx, uint64(42)).Return(nil, errors.New("connection reset")).Once()
		_, err := requireDefaultActor(ctx, users, false)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("requireDefaultActor locked lookup generic failure is redacted", func(t *testing.T) {
		users := mocks.NewMockUserRepositoryInterface(t)
		users.On("FindByIDForUpdate", ctx, uint64(42)).Return(nil, errors.New("connection reset")).Once()
		_, err := requireDefaultActor(ctx, users, true)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("requireDefaultActor nil user without error is unauthenticated", func(t *testing.T) {
		users := mocks.NewMockUserRepositoryInterface(t)
		users.On("FindByID", ctx, uint64(42)).Return(nil, nil).Once()
		_, err := requireDefaultActor(ctx, users, false)
		var apiErr *appErrors.APIServiceError
		require.ErrorAs(t, err, &apiErr)
		assert.Equal(t, "UNAUTHENTICATED", apiErr.Code)
	})

	t.Run("requireAdminActor generic repository failure is redacted", func(t *testing.T) {
		users := mocks.NewMockUserRepositoryInterface(t)
		users.On("FindByID", ctx, uint64(42)).Return(nil, errors.New("connection reset")).Once()
		_, err := requireAdminActor(ctx, users)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("requireAdminActor nil user without error is unauthenticated", func(t *testing.T) {
		users := mocks.NewMockUserRepositoryInterface(t)
		users.On("FindByID", ctx, uint64(42)).Return(nil, nil).Once()
		_, err := requireAdminActor(ctx, users)
		var apiErr *appErrors.APIServiceError
		require.ErrorAs(t, err, &apiErr)
		assert.Equal(t, "UNAUTHENTICATED", apiErr.Code)
	})

	t.Run("requireAdminActor non-admin role is forbidden", func(t *testing.T) {
		users := mocks.NewMockUserRepositoryInterface(t)
		users.On("FindByID", ctx, uint64(42)).Return(&userEntities.User{ID: 42, Role: userEntities.RoleDefault, OnboardingComplete: true}, nil).Once()
		_, err := requireAdminActor(ctx, users)
		var apiErr *appErrors.APIServiceError
		require.ErrorAs(t, err, &apiErr)
		assert.Equal(t, http.StatusForbidden, apiErr.Status)
	})
}

// TestMediaMoments_FindIdempotencyOperationDefensiveBranches covers the generic-repository-failure
// paths in findIdempotencyOperation: a non-not-found error looking up the operation itself, and a
// failure checking the legacy operation tables. Both are only reachable via a repository fault
// that a real Postgres round-trip won't produce, so they are exercised here with a mock.
func TestMediaMoments_FindIdempotencyOperationDefensiveBranches(t *testing.T) {
	ctx := ctxWithUser(42)

	t.Run("generic FindOperation failure is redacted", func(t *testing.T) {
		repo := mocks.NewMockMediaRepository(t)
		repo.On("FindOperation", ctx, uint64(42), "key").Return(nil, errors.New("connection reset")).Once()
		_, err := findIdempotencyOperation(ctx, repo, 42, "key", "op", "hash")
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("legacy operation lookup failure is redacted", func(t *testing.T) {
		repo := mocks.NewMockMediaRepository(t)
		repo.On("FindOperation", ctx, uint64(42), "key").Return(nil, appErrors.ErrNotFound).Once()
		repo.On("FindLegacyOperation", ctx, uint64(42), "key").Return(false, errors.New("connection reset")).Once()
		_, err := findIdempotencyOperation(ctx, repo, 42, "key", "op", "hash")
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("legacy key reuse is rejected", func(t *testing.T) {
		repo := mocks.NewMockMediaRepository(t)
		repo.On("FindOperation", ctx, uint64(42), "key").Return(nil, appErrors.ErrNotFound).Once()
		repo.On("FindLegacyOperation", ctx, uint64(42), "key").Return(true, nil).Once()
		_, err := findIdempotencyOperation(ctx, repo, 42, "key", "op", "hash")
		var apiErr *appErrors.APIServiceError
		require.ErrorAs(t, err, &apiErr)
		assert.Equal(t, "IDEMPOTENCY_KEY_REUSED", apiErr.Code)
	})
}
