package mappers

import (
	mediaEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/media/entities"
	momentEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/moment/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
)

func MapMediaAssetToEntity(row *models.MediaAsset) *mediaEntities.Asset {
	if row == nil {
		return nil
	}
	return &mediaEntities.Asset{
		ID:               row.ID,
		OwnerUserID:      row.OwnerUserID,
		Provider:         row.Provider,
		Bucket:           row.Bucket,
		StagingObjectKey: row.StagingObjectKey,
		StagingVersionID: row.StagingVersionID,
		FinalObjectKey:   row.FinalObjectKey,
		FinalVersionID:   row.FinalVersionID,
		ContentType:      row.ContentType,
		Bytes:            row.Bytes,
		ChecksumSHA256:   row.ChecksumSHA256,
		State:            mediaEntities.AssetState(row.State),
		UploadExpiresAt:  row.UploadExpiresAt,
		RetentionDueAt:   row.RetentionDueAt,
		AvailableAt:      row.AvailableAt,
		FailedAt:         row.FailedAt,
		DeletedAt:        row.DeletedAt,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}
}

func MapMediaAssetEntityToModel(item *mediaEntities.Asset) *models.MediaAsset {
	if item == nil {
		return nil
	}
	return &models.MediaAsset{
		ID:               item.ID,
		OwnerUserID:      item.OwnerUserID,
		Provider:         item.Provider,
		Bucket:           item.Bucket,
		StagingObjectKey: item.StagingObjectKey,
		StagingVersionID: item.StagingVersionID,
		FinalObjectKey:   item.FinalObjectKey,
		FinalVersionID:   item.FinalVersionID,
		ContentType:      item.ContentType,
		Bytes:            item.Bytes,
		ChecksumSHA256:   item.ChecksumSHA256,
		State:            string(item.State),
		UploadExpiresAt:  item.UploadExpiresAt,
		RetentionDueAt:   item.RetentionDueAt,
		AvailableAt:      item.AvailableAt,
		FailedAt:         item.FailedAt,
		DeletedAt:        item.DeletedAt,
		CreatedAt:        item.CreatedAt,
		UpdatedAt:        item.UpdatedAt,
	}
}

func MapOperationToEntity(row *models.IdempotencyOperation) *mediaEntities.Operation {
	if row == nil {
		return nil
	}
	return &mediaEntities.Operation{
		ID:               row.ID,
		ActorUserID:      row.ActorUserID,
		IdempotencyKey:   row.IdempotencyKey,
		Operation:        row.Operation,
		ResourceRef:      row.ResourceRef,
		IntentHash:       row.IntentHash,
		State:            row.State,
		ResultRef:        row.ResultRef,
		ResultBoolean:    row.ResultBoolean,
		ResultCount:      row.ResultCount,
		ResponseSnapshot: append([]byte(nil), row.ResponseSnapshot...),
		HTTPStatus:       row.HTTPStatus,
		CreatedAt:        row.CreatedAt,
		CompletedAt:      row.CompletedAt,
	}
}

func MapMomentEntityToModel(item *momentEntities.Moment) *models.Moment {
	if item == nil {
		return nil
	}
	return &models.Moment{
		ID:                item.ID,
		UserID:            item.UserID,
		ParticipationID:   item.ParticipationID,
		ActivityID:        item.ActivityID,
		MediaAssetID:      item.MediaAssetID,
		Origin:            string(item.Origin),
		PublicationStatus: string(item.PublicationStatus),
		ModerationStatus:  string(item.ModerationStatus),
		RewardStatus:      string(item.RewardStatus),
		PointsAwarded:     item.PointsAwarded,
		CapturedAt:        item.CapturedAt,
		CreatedAt:         item.CreatedAt,
		UpdatedAt:         item.UpdatedAt,
	}
}

func MapMomentToEntity(row *models.Moment) *momentEntities.Moment {
	if row == nil {
		return nil
	}
	return &momentEntities.Moment{
		ID:                row.ID,
		UserID:            row.UserID,
		ParticipationID:   row.ParticipationID,
		ActivityID:        row.ActivityID,
		MediaAssetID:      row.MediaAssetID,
		Origin:            momentEntities.Origin(row.Origin),
		PublicationStatus: momentEntities.PublicationStatus(row.PublicationStatus),
		ModerationStatus:  momentEntities.ModerationStatus(row.ModerationStatus),
		RewardStatus:      momentEntities.RewardStatus(row.RewardStatus),
		PointsAwarded:     row.PointsAwarded,
		CapturedAt:        row.CapturedAt,
		CreatedAt:         row.CreatedAt,
		UpdatedAt:         row.UpdatedAt,
	}
}
