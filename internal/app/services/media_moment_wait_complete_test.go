package services

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	mediaEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/media/entities"
	userEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMediaMoments_CompleteUploadWaitsForInFlightSameKeyConfirmationThenReplays exercises
// the polling wait path a request takes when its own idempotency key is already held by
// another in-flight confirmation of the same asset (the processing claim's operation key
// matches). This is distinct from the DB-transaction-level race covered elsewhere: here the
// idempotency operation row already exists in "processing" state and the processing claim is
// already held, so CompleteUpload must poll until the original request finishes and then
// replay its result rather than erroring or reprocessing the object itself.
func TestMediaMoments_CompleteUploadWaitsForInFlightSameKeyConfirmationThenReplays(t *testing.T) {
	mediaService, _, storage := setupMediaMomentServices(t)
	owner, ownerCtx := seedMediaMomentUser(t, "media-wait-complete@example.com", userEntities.RoleDefault, true)
	body := mediaMomentImage(t, "image/jpeg")
	intent, _, err := mediaService.CreateUploadIntent(ownerCtx, uuid.NewString(), &messages.CreateUploadIntentRequestDTO{
		ContentType: "image/jpeg", Bytes: int64(len(body)), ChecksumSHA256: checksum(body),
	})
	require.NoError(t, err)
	storage.staged[intent.ID] = body

	key := uuid.NewString()
	now := utcNow(mediaService.now)
	hash := intentHash("media.upload.complete", struct {
		MediaAssetID string `json:"mediaAssetId"`
	}{intent.ID})

	// Simulate another in-flight confirmation: the idempotency operation is already
	// recorded as "processing" and the processing claim for this asset is already held
	// under the same idempotency key.
	opID := uuid.NewString()
	require.NoError(t, mediaService.repo.CreateOperation(ownerCtx, &mediaEntities.Operation{
		ID:               opID,
		ActorUserID:      owner.ID,
		IdempotencyKey:   key,
		Operation:        "media.upload.complete",
		ResourceRef:      &intent.ID,
		IntentHash:       hash,
		State:            "processing",
		ResultRef:        &intent.ID,
		ResponseSnapshot: []byte(`{}`),
		CreatedAt:        now,
	}))
	claimToken := uuid.NewString()
	_, acquired, err := mediaService.repo.AcquireProcessingClaim(ownerCtx, intent.ID, claimToken, key, now, now.Add(5*time.Minute))
	require.NoError(t, err)
	require.True(t, acquired)

	type result struct {
		asset  *messages.MediaAssetResponseDTO
		status int
		err    error
	}
	results := make(chan result, 1)
	go func() {
		asset, status, completeErr := mediaService.CompleteUpload(ownerCtx, intent.ID, key)
		results <- result{asset, status, completeErr}
	}()

	// Give the goroutine time to enter the polling wait before the in-flight
	// confirmation "finishes" from the outside.
	time.Sleep(75 * time.Millisecond)

	asset, err := mediaService.repo.FindAsset(context.Background(), intent.ID, false)
	require.NoError(t, err)
	availableAt := now
	asset.State = mediaEntities.AssetAvailable
	asset.AvailableAt = &availableAt
	asset.UpdatedAt = now
	require.NoError(t, mediaService.repo.UpdateAsset(context.Background(), asset))
	require.NoError(t, mediaService.repo.CompleteOperation(
		context.Background(), opID, http.StatusOK, &intent.ID, nil, nil, []byte(`{}`), now,
	))

	select {
	case r := <-results:
		require.NoError(t, r.err)
		assert.Equal(t, http.StatusOK, r.status)
		assert.Equal(t, intent.ID, r.asset.ID)
		assert.Equal(t, "available", r.asset.State)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for CompleteUpload to observe the completed in-flight confirmation")
	}
}
