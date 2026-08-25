package mappers

import (
	"testing"
	"time"

	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMediaMoments_MapperNilSafety(t *testing.T) {
	assert.Nil(t, MapMediaAssetToEntity(nil))
	assert.Nil(t, MapMediaAssetEntityToModel(nil))
	assert.Nil(t, MapOperationToEntity(nil))
	assert.Nil(t, MapMomentEntityToModel(nil))
	assert.Nil(t, MapMomentToEntity(nil))
}

func TestMediaMoments_MediaAssetRoundtrip(t *testing.T) {
	now := time.Now().UTC()
	stagingVersion, finalVersion := "staging-v1", "final-v1"
	available, failed, deleted := now, now, now
	row := &models.MediaAsset{
		ID:               "asset-1",
		OwnerUserID:      42,
		Provider:         "s3",
		Bucket:           "bucket",
		StagingObjectKey: "staging/key",
		StagingVersionID: &stagingVersion,
		FinalObjectKey:   "final/key",
		FinalVersionID:   &finalVersion,
		ContentType:      "image/jpeg",
		Bytes:            1024,
		ChecksumSHA256:   "checksum",
		State:            "available",
		UploadExpiresAt:  now,
		RetentionDueAt:   now,
		AvailableAt:      &available,
		FailedAt:         &failed,
		DeletedAt:        &deleted,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	entity := MapMediaAssetToEntity(row)
	require.NotNil(t, entity)
	assert.Equal(t, row.ID, entity.ID)
	assert.Equal(t, row.OwnerUserID, entity.OwnerUserID)
	assert.Equal(t, row.StagingVersionID, entity.StagingVersionID)
	assert.Equal(t, string(entity.State), row.State)

	back := MapMediaAssetEntityToModel(entity)
	require.NotNil(t, back)
	assert.Equal(t, row, back)
}

func TestMediaMoments_OperationRoundtrip(t *testing.T) {
	now := time.Now().UTC()
	completed := now
	resultRef := "asset-1"
	resultBoolean := true
	resultCount := 3
	row := &models.IdempotencyOperation{
		ID:               "op-1",
		ActorUserID:      7,
		IdempotencyKey:   "11111111-1111-1111-1111-111111111111",
		Operation:        "media.upload.complete",
		ResourceRef:      &resultRef,
		IntentHash:       "hash",
		State:            "completed",
		ResultRef:        &resultRef,
		ResultBoolean:    &resultBoolean,
		ResultCount:      &resultCount,
		ResponseSnapshot: []byte(`{"a":1}`),
		HTTPStatus:       200,
		CreatedAt:        now,
		CompletedAt:      &completed,
	}
	entity := MapOperationToEntity(row)
	require.NotNil(t, entity)
	assert.Equal(t, row.ID, entity.ID)
	assert.Equal(t, []byte(row.ResponseSnapshot), []byte(entity.ResponseSnapshot))
	// The snapshot must be an independent copy, not an alias of the row's slice.
	entity.ResponseSnapshot[0] = 'X'
	assert.NotEqual(t, entity.ResponseSnapshot[0], row.ResponseSnapshot[0])
}

func TestMediaMoments_MomentRoundtrip(t *testing.T) {
	now := time.Now().UTC()
	participationID := "participation-1"
	activityID := "activity-1"
	row := &models.Moment{
		ID:                "moment-1",
		UserID:            9,
		ParticipationID:   &participationID,
		ActivityID:        &activityID,
		MediaAssetID:      "asset-1",
		Origin:            "challenge",
		PublicationStatus: "public",
		ModerationStatus:  "approved",
		RewardStatus:      "awarded",
		PointsAwarded:     10,
		CapturedAt:        now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	entity := MapMomentToEntity(row)
	require.NotNil(t, entity)
	assert.Equal(t, row.ID, entity.ID)
	assert.Equal(t, row.ParticipationID, entity.ParticipationID)
	assert.Equal(t, string(entity.Origin), row.Origin)

	back := MapMomentEntityToModel(entity)
	require.NotNil(t, back)
	assert.Equal(t, row, back)
}
