package services

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	appInterfaces "github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	mediaEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/media/entities"
	mediaInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/media/interfaces"
	userInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/user/interfaces"
	"github.com/google/uuid"
)

const uploadIntentLifetime = 10 * time.Minute
const processingLease = 5 * time.Minute

type MediaService struct {
	*BaseService
	repo    mediaInterfaces.Repository
	storage mediaInterfaces.Storage
	users   userInterfaces.UserRepositoryInterface
	now     func() time.Time
}

func NewMediaService(
	base *BaseService,
	repo mediaInterfaces.Repository,
	storage mediaInterfaces.Storage,
	users userInterfaces.UserRepositoryInterface,
) appInterfaces.MediaServiceInterface {
	return &MediaService{BaseService: base, repo: repo, storage: storage, users: users, now: time.Now}
}
func retentionDueAt(created time.Time) (time.Time, error) {
	raw := strings.TrimSpace(os.Getenv("DNJ_MEDIA_RETENTION_ANCHOR_AT"))
	anchor, err := time.Parse(time.RFC3339, raw)
	if err != nil || raw == "" {
		return time.Time{}, mediaUnavailableError()
	}
	anchor = anchor.UTC()
	due := anchor.Add(90 * 24 * time.Hour)
	minimum := created.UTC().Add(90 * 24 * time.Hour)
	if created.After(anchor) && due.Before(minimum) {
		due = minimum
	}
	return due.UTC(), nil
}
func mediaUnavailableError() error {
	return mediaMomentError(http.StatusServiceUnavailable, "MEDIA_UNAVAILABLE", "Mídia temporariamente indisponível.")
}
func (s *MediaService) validateProvider() error {
	if err := s.storage.ValidateConfiguration(); err != nil {
		return mediaUnavailableError()
	}
	return nil
}

func (s *MediaService) CreateUploadIntent(
	ctx context.Context,
	rawKey string,
	request *messages.CreateUploadIntentRequestDTO,
) (*messages.UploadIntentResponseDTO, int, error) {
	if request == nil || request.Bytes <= 0 {
		return nil, 0, mediaMomentError(
			http.StatusBadRequest,
			"INVALID_REQUEST",
			"contentType, bytes e checksumSha256 são obrigatórios.",
		)
	}
	if request.Bytes > maxMediaBytes {
		return nil, 0, mediaMomentError(
			http.StatusRequestEntityTooLarge,
			"IMAGE_TOO_LARGE",
			"A imagem deve ter no máximo 10 MiB.",
		)
	}
	if request.ContentType != "image/jpeg" && request.ContentType != "image/png" {
		return nil, 0, mediaMomentError(
			http.StatusUnsupportedMediaType,
			"UNSUPPORTED_MEDIA_TYPE",
			"Apenas JPEG e PNG são aceitos.",
		)
	}
	decoded, err := base64.StdEncoding.DecodeString(request.ChecksumSHA256)
	if err != nil || len(decoded) != sha256.Size ||
		base64.StdEncoding.EncodeToString(decoded) != request.ChecksumSHA256 {
		return nil, 0, mediaMomentError(
			http.StatusBadRequest,
			"INVALID_REQUEST",
			"checksumSha256 deve ser SHA-256 em Base64 canônico.",
		)
	}
	key, err := parseIdempotencyKey(rawKey)
	if err != nil {
		return nil, 0, err
	}
	if _, err = requireDefaultActor(ctx, s.users, false); err != nil {
		return nil, 0, err
	}
	now := utcNow(s.now)
	operation := "media.upload-intent.create"
	hash := intentHash(operation, request)
	var asset *mediaEntities.Asset
	status := http.StatusCreated
	err = s.WithTransaction(ctx, func(tx context.Context) error {
		actor, authErr := requireDefaultActor(tx, s.users, true)
		if authErr != nil {
			return authErr
		}
		prior, priorErr := findIdempotencyOperation(tx, s.repo, actor.ID, key, operation, hash)
		if priorErr != nil {
			return priorErr
		}
		if prior != nil {
			if prior.State != "completed" || prior.ResultRef == nil {
				return appErrors.InternalError
			}
			asset, priorErr = s.repo.FindAsset(tx, *prior.ResultRef, false)
			if priorErr != nil {
				return appErrors.InternalError
			}
			status = prior.HTTPStatus
			return nil
		}
		if providerErr := s.validateProvider(); providerErr != nil {
			return providerErr
		}
		retention, retentionErr := retentionDueAt(now)
		if retentionErr != nil {
			return retentionErr
		}
		extension := "jpg"
		if request.ContentType == "image/png" {
			extension = "png"
		}
		id := uuid.NewString()
		asset = &mediaEntities.Asset{
			ID:               id,
			OwnerUserID:      actor.ID,
			Provider:         "s3",
			Bucket:           strings.TrimSpace(os.Getenv("S3_BUCKET")),
			StagingObjectKey: "staging/" + uuid.NewString(),
			FinalObjectKey:   "media/" + uuid.NewString() + "." + extension,
			ContentType:      request.ContentType,
			Bytes:            request.Bytes,
			ChecksumSHA256:   request.ChecksumSHA256,
			State:            mediaEntities.AssetPendingUpload,
			UploadExpiresAt:  now.Add(uploadIntentLifetime),
			RetentionDueAt:   retention,
			CreatedAt:        now,
			UpdatedAt:        now,
		}
		if createErr := s.repo.CreateAsset(tx, asset); createErr != nil {
			return appErrors.InternalError
		}
		ref := asset.ID
		completed := now
		if createErr := createIdempotencyOperation(tx, s.repo, &mediaEntities.Operation{ID: uuid.NewString(), ActorUserID: actor.ID, IdempotencyKey: key, Operation: operation, IntentHash: hash, State: "completed", ResultRef: &ref, HTTPStatus: http.StatusCreated, ResponseSnapshot: []byte(`{}`), CreatedAt: now, CompletedAt: &completed}); createErr != nil {
			return createErr
		}
		_, _ = s.repo.CreateCleanupJob(
			tx,
			&mediaEntities.CleanupJob{
				ID:            uuid.NewString(),
				MediaAssetID:  asset.ID,
				Kind:          "retention",
				State:         "pending",
				DueAt:         retention,
				MaxAttempts:   8,
				NextAttemptAt: retention,
				CreatedAt:     now,
				UpdatedAt:     now,
			},
		)
		return nil
	})
	if errors.Is(err, errIdempotencyRace) {
		return s.CreateUploadIntent(ctx, rawKey, request)
	}
	if err != nil {
		return nil, 0, err
	}
	signed, err := s.storage.PresignUpload(ctx, asset, asset.CreatedAt)
	if err != nil {
		return nil, 0, mediaUnavailableError()
	}
	return &messages.UploadIntentResponseDTO{
		ID:        asset.ID,
		UploadURL: signed.URL,
		Method:    signed.Method,
		Headers:   signed.Headers,
		ExpiresAt: asset.UploadExpiresAt.UTC(),
	}, status, nil
}

func mediaAssetDTO(asset *mediaEntities.Asset) *messages.MediaAssetResponseDTO {
	var availableAt *time.Time
	if asset.AvailableAt != nil {
		value := asset.AvailableAt.UTC()
		availableAt = &value
	}
	return &messages.MediaAssetResponseDTO{
		ID:             asset.ID,
		ContentType:    asset.ContentType,
		Bytes:          asset.Bytes,
		State:          string(asset.State),
		AvailableAt:    availableAt,
		RetentionDueAt: asset.RetentionDueAt.UTC(),
	}
}

func (s *MediaService) CompleteUpload(
	ctx context.Context,
	rawAssetID, rawKey string,
) (*messages.MediaAssetResponseDTO, int, error) {
	assetID, err := uuid.Parse(rawAssetID)
	if err != nil {
		return nil, 0, mediaMomentError(http.StatusNotFound, "NOT_FOUND", "Recurso não encontrado.")
	}
	key, err := parseIdempotencyKey(rawKey)
	if err != nil {
		return nil, 0, err
	}
	actor, err := requireDefaultActor(ctx, s.users, false)
	if err != nil {
		return nil, 0, err
	}
	operation := "media.upload.complete"
	hash := intentHash(operation, struct {
		MediaAssetID string `json:"mediaAssetId"`
	}{assetID.String()})
	var asset *mediaEntities.Asset
	var op *mediaEntities.Operation
	created := false
	var terminalErr error
	now := utcNow(s.now)
	err = s.WithTransaction(ctx, func(tx context.Context) error {
		var findErr error
		asset, findErr = s.repo.FindAsset(tx, assetID.String(), true)
		if errors.Is(findErr, appErrors.ErrNotFound) || findErr == nil && asset.OwnerUserID != actor.ID {
			return mediaMomentError(http.StatusNotFound, "NOT_FOUND", "Recurso não encontrado.")
		}
		if findErr != nil {
			return appErrors.InternalError
		}
		if _, authErr := requireDefaultActor(tx, s.users, true); authErr != nil {
			return authErr
		}
		op, findErr = findIdempotencyOperation(tx, s.repo, actor.ID, key, operation, hash)
		if findErr != nil {
			return findErr
		}
		if op != nil {
			if op.State == "completed" {
				if op.HTTPStatus >= 400 {
					return replayOperationError(op)
				}
				return nil
			}
			return nil
		}
		op = &mediaEntities.Operation{
			ID:               uuid.NewString(),
			ActorUserID:      actor.ID,
			IdempotencyKey:   key,
			Operation:        operation,
			ResourceRef:      &asset.ID,
			IntentHash:       hash,
			State:            "processing",
			ResultRef:        &asset.ID,
			ResponseSnapshot: []byte(`{}`),
			CreatedAt:        now,
		}
		if createErr := createIdempotencyOperation(tx, s.repo, op); createErr != nil {
			return createErr
		}
		created = true
		if !now.Before(asset.UploadExpiresAt) {
			if persistErr := s.completeTerminal(tx, asset, op, http.StatusGone, "UPLOAD_EXPIRED", "A intenção de upload expirou.", now); persistErr != nil {
				return persistErr
			}
			terminalErr = mediaMomentError(http.StatusGone, "UPLOAD_EXPIRED", "A intenção de upload expirou.")
			return nil
		}
		if asset.State != mediaEntities.AssetPendingUpload && asset.State != mediaEntities.AssetProcessing {
			return mediaMomentError(
				http.StatusConflict,
				"UPLOAD_STATE_CONFLICT",
				"O upload está em estado incompatível.",
			)
		}
		asset.State = mediaEntities.AssetProcessing
		asset.UpdatedAt = now
		return s.repo.UpdateAsset(tx, asset)
	})
	if errors.Is(err, errIdempotencyRace) {
		return s.CompleteUpload(ctx, rawAssetID, rawKey)
	}
	if err != nil {
		return nil, 0, err
	}
	if terminalErr != nil {
		return nil, 0, terminalErr
	}
	if op.State == "completed" {
		return mediaAssetDTO(asset), op.HTTPStatus, nil
	}
	claimToken := uuid.NewString()
	claim, acquired, err := s.repo.AcquireProcessingClaim(ctx, asset.ID, claimToken, key, now, now.Add(processingLease))
	if err != nil {
		return nil, 0, appErrors.InternalError
	}
	if !acquired {
		if claim.OperationKey == key {
			return s.waitComplete(ctx, actor.ID, key, asset)
		}
		return nil, 0, mediaMomentError(
			http.StatusConflict,
			"UPLOAD_STATE_CONFLICT",
			"O upload já está sendo confirmado.",
		)
	}
	_ = created
	result, status, err := s.processUpload(ctx, actor.ID, asset, op, claim)
	return result, status, err
}

func (s *MediaService) waitComplete(
	ctx context.Context,
	actor uint64,
	key string,
	asset *mediaEntities.Asset,
) (*messages.MediaAssetResponseDTO, int, error) {
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.NewTimer(10 * time.Second)
	defer timeout.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, 0, mediaMomentError(http.StatusConflict, "UPLOAD_STATE_CONFLICT", "Confirmação em andamento.")
		case <-timeout.C:
			return nil, 0, mediaMomentError(http.StatusConflict, "UPLOAD_STATE_CONFLICT", "Confirmação em andamento.")
		case <-ticker.C:
			op, err := s.repo.FindOperation(ctx, actor, key)
			if err == nil && op.State == "completed" {
				if op.HTTPStatus >= 400 {
					return nil, 0, replayOperationError(op)
				}
				latest, findErr := s.repo.FindAsset(ctx, asset.ID, false)
				if findErr != nil {
					return nil, 0, appErrors.InternalError
				}
				return mediaAssetDTO(latest), op.HTTPStatus, nil
			}
		}
	}
}

func (s *MediaService) processUpload(
	ctx context.Context,
	actor uint64,
	asset *mediaEntities.Asset,
	op *mediaEntities.Operation,
	claim *mediaEntities.ProcessingClaim,
) (*messages.MediaAssetResponseDTO, int, error) {
	release := func(category string) {
		claim.LeaseExpiresAt = utcNow(s.now)
		claim.LastErrorCategory = &category
		claim.UpdatedAt = claim.LeaseExpiresAt
		_ = s.repo.UpdateProcessingClaim(ctx, claim)
	}
	if claim.FinalVersionID != nil {
		if _, err := s.storage.HeadFinal(ctx, asset, *claim.FinalVersionID); err == nil {
			return s.finalizeUpload(ctx, actor, asset, op, claim)
		}
		claim.FinalVersionID = nil
	}
	if claim.StagingVersionID == nil {
		head, err := s.storage.HeadStaging(ctx, asset)
		if errors.Is(err, mediaEntities.ErrObjectNotFound) {
			release("incomplete")
			return nil, 0, mediaMomentError(http.StatusConflict, "UPLOAD_INCOMPLETE", "O objeto ainda não foi enviado.")
		}
		if err != nil {
			release("provider")
			return nil, 0, mediaUnavailableError()
		}
		if head.VersionID == "" {
			release("versioning")
			return nil, 0, mediaUnavailableError()
		}
		if head.Bytes != asset.Bytes || head.ContentType != asset.ContentType ||
			!secureChecksumEqual(head.ChecksumSHA256, asset.ChecksumSHA256) {
			return s.failInvalid(
				ctx,
				asset,
				op,
				claim,
				http.StatusUnprocessableEntity,
				"IMAGE_INVALID",
				"Metadados do upload não conferem.",
			)
		}
		claim.StagingVersionID = &head.VersionID
		claim.Stage = "staging_verified"
		claim.LeaseExpiresAt = utcNow(s.now).Add(processingLease)
		claim.UpdatedAt = utcNow(s.now)
		if err := s.repo.UpdateProcessingClaim(ctx, claim); err != nil {
			return nil, 0, mediaMomentError(
				http.StatusConflict,
				"UPLOAD_STATE_CONFLICT",
				"A confirmação perdeu o lease.",
			)
		}
	}
	body, err := s.storage.DownloadStaging(ctx, asset, *claim.StagingVersionID, maxMediaBytes)
	if errors.Is(err, mediaEntities.ErrObjectTooLarge) {
		return s.failInvalid(
			ctx,
			asset,
			op,
			claim,
			http.StatusRequestEntityTooLarge,
			"IMAGE_TOO_LARGE",
			"A imagem deve ter no máximo 10 MiB.",
		)
	}
	if err != nil {
		release("provider")
		return nil, 0, mediaUnavailableError()
	}
	digest := sha256.Sum256(body.Bytes)
	if int64(len(body.Bytes)) != asset.Bytes ||
		!secureChecksumEqual(base64.StdEncoding.EncodeToString(digest[:]), asset.ChecksumSHA256) {
		return s.failInvalid(
			ctx,
			asset,
			op,
			claim,
			http.StatusUnprocessableEntity,
			"IMAGE_INVALID",
			"Checksum do upload não confere.",
		)
	}
	sanitized, err := sanitizeImage(body.Bytes, asset.ContentType)
	if err != nil {
		return s.failInvalid(
			ctx,
			asset,
			op,
			claim,
			http.StatusUnprocessableEntity,
			"IMAGE_INVALID",
			"A imagem é inválida ou excede os limites permitidos.",
		)
	}
	final, err := s.storage.PutFinal(ctx, asset, sanitized, asset.ContentType)
	if err != nil {
		release("provider")
		return nil, 0, mediaUnavailableError()
	}
	claim.FinalVersionID = &final.VersionID
	claim.Stage = "final_written"
	claim.LeaseExpiresAt = utcNow(s.now).Add(processingLease)
	claim.UpdatedAt = utcNow(s.now)
	if err := s.repo.UpdateProcessingClaim(ctx, claim); err != nil {
		return nil, 0, mediaMomentError(http.StatusConflict, "UPLOAD_STATE_CONFLICT", "A confirmação perdeu o lease.")
	}
	return s.finalizeUpload(ctx, actor, asset, op, claim)
}

func (s *MediaService) finalizeUpload(
	ctx context.Context,
	actor uint64,
	asset *mediaEntities.Asset,
	op *mediaEntities.Operation,
	claim *mediaEntities.ProcessingClaim,
) (*messages.MediaAssetResponseDTO, int, error) {
	now := utcNow(s.now)
	err := s.WithTransaction(ctx, func(tx context.Context) error {
		locked, err := s.repo.FindAsset(tx, asset.ID, true)
		if err != nil {
			return appErrors.InternalError
		}
		if locked.OwnerUserID != actor {
			return mediaMomentError(http.StatusNotFound, "NOT_FOUND", "Recurso não encontrado.")
		}
		if _, err := requireDefaultActor(tx, s.users, true); err != nil {
			return err
		}
		locked.State = mediaEntities.AssetAvailable
		locked.StagingVersionID = claim.StagingVersionID
		locked.FinalVersionID = claim.FinalVersionID
		locked.AvailableAt = &now
		locked.UpdatedAt = now
		if err := s.repo.UpdateAsset(tx, locked); err != nil {
			return appErrors.InternalError
		}
		asset = locked
		_, _ = s.repo.CreateCleanupJob(
			tx,
			&mediaEntities.CleanupJob{
				ID:            uuid.NewString(),
				MediaAssetID:  asset.ID,
				Kind:          "staging",
				State:         "pending",
				DueAt:         now,
				MaxAttempts:   8,
				NextAttemptAt: now,
				CreatedAt:     now,
				UpdatedAt:     now,
			},
		)
		if err := s.repo.CompleteOperation(tx, op.ID, http.StatusOK, &asset.ID, nil, nil, []byte(`{}`), now); err != nil {
			return appErrors.InternalError
		}
		claim.Stage = "completed"
		claim.CompletedAt = &now
		claim.LeaseExpiresAt = now
		claim.UpdatedAt = now
		return s.repo.UpdateProcessingClaim(tx, claim)
	})
	if err != nil {
		return nil, 0, err
	}
	return mediaAssetDTO(asset), http.StatusOK, nil
}

func (s *MediaService) failInvalid(
	ctx context.Context,
	asset *mediaEntities.Asset,
	op *mediaEntities.Operation,
	claim *mediaEntities.ProcessingClaim,
	status int,
	code, message string,
) (*messages.MediaAssetResponseDTO, int, error) {
	now := utcNow(s.now)
	err := s.WithTransaction(
		ctx,
		func(tx context.Context) error { return s.completeTerminal(tx, asset, op, status, code, message, now) },
	)
	claim.Stage = "failed"
	claim.CompletedAt = &now
	claim.LeaseExpiresAt = now
	claim.UpdatedAt = now
	_ = s.repo.UpdateProcessingClaim(ctx, claim)
	if err != nil {
		return nil, 0, appErrors.InternalError
	}
	return nil, 0, mediaMomentError(status, code, message)
}

func (s *MediaService) completeTerminal(
	ctx context.Context,
	asset *mediaEntities.Asset,
	op *mediaEntities.Operation,
	status int,
	code, message string,
	now time.Time,
) error {
	asset.State = mediaEntities.AssetFailed
	asset.FailedAt = &now
	asset.UpdatedAt = now
	if err := s.repo.UpdateAsset(ctx, asset); err != nil {
		return appErrors.InternalError
	}
	_, _ = s.repo.CreateCleanupJob(
		ctx,
		&mediaEntities.CleanupJob{
			ID:            uuid.NewString(),
			MediaAssetID:  asset.ID,
			Kind:          "failed_upload",
			State:         "pending",
			DueAt:         now,
			MaxAttempts:   8,
			NextAttemptAt: now,
			CreatedAt:     now,
			UpdatedAt:     now,
		},
	)
	snapshot, _ := json.Marshal(operationErrorSnapshot{Code: code, Message: message})
	if op == nil {
		return nil
	}
	if err := s.repo.CompleteOperation(ctx, op.ID, status, &asset.ID, nil, nil, snapshot, now); err != nil {
		return appErrors.InternalError
	}
	op.State = "completed"
	op.HTTPStatus = status
	op.ResponseSnapshot = snapshot
	return nil
}
