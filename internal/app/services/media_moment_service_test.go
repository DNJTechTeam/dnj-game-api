package services

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"sync"
	"testing"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	gameEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/game/entities"
	mediaEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/media/entities"
	userEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/repositories"
	"github.com/dnjtechteam/dnj-game-api/internal/mocks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var mediaMomentNow = time.Date(2026, 8, 24, 19, 0, 0, 0, time.UTC)

type memoryMediaStorage struct {
	mu               sync.Mutex
	configurationErr error
	presignUploadErr error
	headErr          error
	downloadErr      error
	putErr           error
	deleteErr        error
	staged           map[string][]byte
	deleted          []string
	headStarted      chan struct{}
	headRelease      chan struct{}
	headStartedOnce  sync.Once
	headVersion      string
}

func newMemoryMediaStorage() *memoryMediaStorage {
	return &memoryMediaStorage{staged: make(map[string][]byte), headVersion: "staging-version-1"}
}

func (s *memoryMediaStorage) ValidateConfiguration() error { return s.configurationErr }

func (s *memoryMediaStorage) PresignUpload(
	_ context.Context,
	asset *mediaEntities.Asset,
	at time.Time,
) (*mediaEntities.PresignedRequest, error) {
	if s.presignUploadErr != nil {
		return nil, s.presignUploadErr
	}
	if s.configurationErr != nil {
		return nil, s.configurationErr
	}
	return &mediaEntities.PresignedRequest{
		URL:       "https://storage.example/" + asset.ID + "?signed=redacted",
		Method:    "PUT",
		Headers:   map[string]string{"Content-Type": asset.ContentType, "x-amz-checksum-sha256": asset.ChecksumSHA256},
		ExpiresAt: at.Add(uploadIntentLifetime),
	}, nil
}

func (s *memoryMediaStorage) PresignDownload(
	_ context.Context,
	asset *mediaEntities.Asset,
	_ time.Time,
	_ time.Duration,
) (string, error) {
	if s.configurationErr != nil {
		return "", s.configurationErr
	}
	return "https://storage.example/read/" + asset.ID + "?signed=redacted", nil
}

func (s *memoryMediaStorage) HeadStaging(
	_ context.Context,
	asset *mediaEntities.Asset,
) (*mediaEntities.ObjectMetadata, error) {
	if s.headStarted != nil {
		s.headStartedOnce.Do(func() { close(s.headStarted) })
		<-s.headRelease
	}
	if s.headErr != nil {
		return nil, s.headErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	body, ok := s.staged[asset.ID]
	if !ok {
		return nil, mediaEntities.ErrObjectNotFound
	}
	digest := sha256.Sum256(body)
	return &mediaEntities.ObjectMetadata{
		ContentType:    asset.ContentType,
		Bytes:          int64(len(body)),
		ChecksumSHA256: base64.StdEncoding.EncodeToString(digest[:]),
		VersionID:      s.headVersion,
	}, nil
}

func (s *memoryMediaStorage) DownloadStaging(
	_ context.Context,
	asset *mediaEntities.Asset,
	versionID string,
	limit int64,
) (*mediaEntities.ObjectBody, error) {
	if s.downloadErr != nil {
		return nil, s.downloadErr
	}
	if versionID != s.headVersion || versionID == "" {
		return nil, mediaEntities.ErrObjectNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	body := append([]byte(nil), s.staged[asset.ID]...)
	if int64(len(body)) > limit {
		return nil, mediaEntities.ErrObjectTooLarge
	}
	return &mediaEntities.ObjectBody{Bytes: body}, nil
}

func (s *memoryMediaStorage) PutFinal(
	_ context.Context,
	_ *mediaEntities.Asset,
	_ []byte,
	_ string,
) (*mediaEntities.ObjectMetadata, error) {
	if s.putErr != nil {
		return nil, s.putErr
	}
	return &mediaEntities.ObjectMetadata{VersionID: "final-version-1"}, nil
}

func (s *memoryMediaStorage) HeadFinal(
	_ context.Context,
	_ *mediaEntities.Asset,
	versionID string,
) (*mediaEntities.ObjectMetadata, error) {
	if versionID != "final-version-1" {
		return nil, mediaEntities.ErrObjectNotFound
	}
	return &mediaEntities.ObjectMetadata{VersionID: versionID}, nil
}

func (s *memoryMediaStorage) DeleteObjectVersions(_ context.Context, _ string, key string) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deleted = append(s.deleted, key)
	return nil
}

func mediaMomentImage(t *testing.T, contentType string) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 16, 12))
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			img.Set(x, y, color.NRGBA{R: uint8(x * 7), G: uint8(y * 9), B: 80, A: 255})
		}
	}
	var output bytes.Buffer
	var err error
	if contentType == "image/png" {
		err = png.Encode(&output, img)
	} else {
		err = jpeg.Encode(&output, img, &jpeg.Options{Quality: 92})
	}
	require.NoError(t, err)
	return output.Bytes()
}

func checksum(body []byte) string {
	digest := sha256.Sum256(body)
	return base64.StdEncoding.EncodeToString(digest[:])
}

func setupMediaMomentServices(
	t *testing.T,
) (*MediaService, *MomentService, *memoryMediaStorage) {
	t.Helper()
	TestSuite.DefaultSetup(t)
	t.Setenv("S3_BUCKET", "dnj-private-test")
	t.Setenv("DNJ_MEDIA_RETENTION_ANCHOR_AT", "2026-08-20T23:00:00-03:00")
	t.Setenv("DNJ_CURSOR_HMAC_SECRET", "media-moment-cursor-test-secret")
	TestSuite.TruncateTable(t, &models.User{})

	mediaRepository := repositories.NewMediaRepository(TestSuite.DbConn)
	momentRepository := repositories.NewMomentRepository(TestSuite.DbConn)
	storage := newMemoryMediaStorage()
	mediaService := NewMediaService(
		TestSuite.BaseService,
		mediaRepository,
		storage,
		TestSuite.UserRepository,
	).(*MediaService)
	momentService := NewMomentService(
		TestSuite.BaseService,
		momentRepository,
		mediaRepository,
		storage,
		TestSuite.UserRepository,
		TestSuite.OperationAuditRepository,
	).(*MomentService)
	mediaService.now = func() time.Time { return mediaMomentNow }
	momentService.now = func() time.Time { return mediaMomentNow }
	return mediaService, momentService, storage
}

func seedMediaMomentUser(
	t *testing.T,
	email string,
	role userEntities.UserRole,
	onboarding bool,
) (*userEntities.User, context.Context) {
	t.Helper()
	user, err := TestSuite.UserRepository.Create(TestSuite.Ctx, &userEntities.User{
		Email:              email,
		Name:               "Participante Público",
		Role:               role,
		OnboardingComplete: onboarding,
	})
	require.NoError(t, err)
	return user, TestSuite.ContextWithUser(user.ID)
}

func createAvailableAsset(
	t *testing.T,
	service *MediaService,
	storage *memoryMediaStorage,
	ctx context.Context,
	contentType string,
) *messages.MediaAssetResponseDTO {
	t.Helper()
	body := mediaMomentImage(t, contentType)
	intent, status, err := service.CreateUploadIntent(ctx, uuid.NewString(), &messages.CreateUploadIntentRequestDTO{
		ContentType:    contentType,
		Bytes:          int64(len(body)),
		ChecksumSHA256: checksum(body),
	})
	require.NoError(t, err)
	require.Equal(t, 201, status)
	storage.staged[intent.ID] = body
	asset, status, err := service.CompleteUpload(ctx, intent.ID, uuid.NewString())
	require.NoError(t, err)
	require.Equal(t, 200, status)
	return asset
}

func seedChallengeParticipation(
	t *testing.T,
	userID uint64,
	allowsMoment bool,
	status string,
) string {
	t.Helper()
	activityID := uuid.NewString()
	runID := uuid.NewString()
	qrID := uuid.NewString()
	participationID := uuid.NewString()
	require.NoError(t, TestSuite.DbConn.Create(&models.Activity{
		ID:           activityID,
		Slug:         "moment-" + activityID,
		Name:         "Desafio da Foto",
		Kind:         "challenge",
		Status:       status,
		StartsAt:     timePointer(mediaMomentNow.Add(-time.Hour)),
		EndsAt:       timePointer(mediaMomentNow),
		MomentPoints: 25,
		AllowsMoment: allowsMoment,
		CreatedAt:    mediaMomentNow.Add(-time.Hour),
		UpdatedAt:    mediaMomentNow,
	}).Error)
	require.NoError(t, TestSuite.DbConn.Create(&models.ActivityRun{
		ID:         runID,
		ActivityID: activityID,
		StartedBy:  userID,
		Status:     string(gameEntities.RunStatusActive),
		PointRules: json.RawMessage(`{"first":0,"second":0,"third":0,"participation":0}`),
		StartedAt:  timePointer(mediaMomentNow.Add(-time.Hour)),
		CreatedAt:  mediaMomentNow.Add(-time.Hour),
		UpdatedAt:  mediaMomentNow,
	}).Error)
	require.NoError(t, TestSuite.DbConn.Create(&models.ActivityRunQRCode{
		ID:            qrID,
		ActivityID:    activityID,
		ActivityRunID: runID,
		TokenHash:     checksum([]byte(qrID)),
		ExpiresAt:     mediaMomentNow.Add(time.Hour),
		Status:        string(gameEntities.QRCodeStatusActive),
		CreatedAt:     mediaMomentNow.Add(-time.Hour),
		UpdatedAt:     mediaMomentNow,
	}).Error)
	require.NoError(t, TestSuite.DbConn.Create(&models.Participation{
		ID:             participationID,
		UserID:         userID,
		ActivityID:     activityID,
		ActivityRunID:  runID,
		QRCodeID:       qrID,
		CheckedInAt:    mediaMomentNow.Add(-30 * time.Minute),
		Status:         string(gameEntities.ParticipationStatusActive),
		CanShareMoment: allowsMoment,
		CreatedAt:      mediaMomentNow.Add(-30 * time.Minute),
	}).Error)
	return participationID
}

func seedActiveMomentChallenge(t *testing.T) string {
	t.Helper()
	activityID := uuid.NewString()
	require.NoError(t, TestSuite.DbConn.Create(&models.Activity{
		ID:           activityID,
		Slug:         "active-moment-" + activityID,
		Name:         "Desafio do Momento",
		Kind:         "challenge",
		Status:       "active",
		StartsAt:     timePointer(mediaMomentNow.Add(-time.Hour)),
		EndsAt:       timePointer(mediaMomentNow.Add(time.Hour)),
		MomentPoints: 50,
		AllowsMoment: true,
		CreatedAt:    mediaMomentNow.Add(-time.Hour),
		UpdatedAt:    mediaMomentNow,
	}).Error)
	return activityID
}

func TestMediaMoments_ChallengeModeAwardsWithoutQRParticipation(t *testing.T) {
	mediaService, momentService, storage := setupMediaMomentServices(t)
	participant, participantCtx := seedMediaMomentUser(t, "challenge-no-qr@example.com", userEntities.RoleDefault, true)
	activityID := seedActiveMomentChallenge(t)
	asset := createAvailableAsset(t, mediaService, storage, participantCtx, "image/jpeg")

	moment, status, err := momentService.Create(participantCtx, uuid.NewString(), &messages.CreateMomentRequestDTO{
		MediaAssetID: asset.ID, PublishConsent: true, ChallengeMode: true,
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, status)
	assert.Equal(t, "challenge", moment.Origin)
	assert.Nil(t, moment.ParticipationID)
	assert.Equal(t, 50, moment.PointsAwarded)

	var points uint64
	require.NoError(t, TestSuite.DbConn.Table("users").Select("points").Where("id = ?", participant.ID).Scan(&points).Error)
	assert.Equal(t, uint64(50), points)
	var activityCount int64
	require.NoError(t, TestSuite.DbConn.Table("moments").Where("id = ? AND activity_id = ?", moment.ID, activityID).Count(&activityCount).Error)
	assert.EqualValues(t, 1, activityCount)

	secondAsset := createAvailableAsset(t, mediaService, storage, participantCtx, "image/jpeg")
	_, _, err = momentService.Create(participantCtx, uuid.NewString(), &messages.CreateMomentRequestDTO{
		MediaAssetID: secondAsset.ID, PublishConsent: true, ChallengeMode: true,
	})
	require.Error(t, err)
	var apiErr *appErrors.APIServiceError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, "MOMENT_ALREADY_COMPLETED", apiErr.Code)
}

func TestMediaMoments_FullLifecycleUsesDurableState(t *testing.T) {
	mediaService, momentService, storage := setupMediaMomentServices(t)
	participant, participantCtx := seedMediaMomentUser(t, "moment-owner@example.com", userEntities.RoleDefault, true)
	_, adminCtx := seedMediaMomentUser(t, "moment-admin@example.com", userEntities.RoleAdmin, true)

	asset := createAvailableAsset(t, mediaService, storage, participantCtx, "image/jpeg")
	participationID := seedChallengeParticipation(t, participant.ID, true, "active")
	createKey := uuid.NewString()
	moment, status, err := momentService.Create(participantCtx, createKey, &messages.CreateMomentRequestDTO{
		MediaAssetID:    asset.ID,
		PublishConsent:  true,
		ParticipationID: &participationID,
	})
	require.NoError(t, err)
	assert.Equal(t, 201, status)
	assert.Equal(t, "challenge", moment.Origin)
	assert.Equal(t, 25, moment.PointsAwarded)
	assert.Equal(t, "public", moment.PublicationStatus)
	assert.Equal(t, "pending", moment.ModerationStatus)
	assert.NotEmpty(t, moment.ImageURL)

	replayed, replayStatus, err := momentService.Create(participantCtx, createKey, &messages.CreateMomentRequestDTO{
		MediaAssetID:    asset.ID,
		PublishConsent:  true,
		ParticipationID: &participationID,
	})
	require.NoError(t, err)
	assert.Equal(t, 201, replayStatus)
	assert.Equal(t, moment, replayed)

	// A moment born pending publishes immediately — curation never blocks the feed.
	feed, err := momentService.List(participantCtx, "feed", "")
	require.NoError(t, err)
	require.Len(t, feed.Items, 1)
	assert.Equal(t, moment.ID, feed.Items[0].ID)

	likeKey := uuid.NewString()
	liked, err := momentService.ToggleLike(participantCtx, moment.ID, likeKey)
	require.NoError(t, err)
	assert.True(t, liked.Liked)
	assert.Equal(t, 1, liked.LikesCount)
	likedReplay, err := momentService.ToggleLike(participantCtx, moment.ID, likeKey)
	require.NoError(t, err)
	assert.Equal(t, liked, likedReplay)

	// It still sits in the pending curation queue for an admin to work through.
	moderation, err := momentService.ListModeration(adminCtx, "challenge", 0)
	require.NoError(t, err)
	require.Len(t, moderation.Data, 1)
	assert.ElementsMatch(t, []string{"approve", "deny_points", "delete_photo"}, moderation.Data[0].AvailableActions)

	approveKey := uuid.NewString()
	approved, err := momentService.Moderate(adminCtx, moment.ID, approveKey, &messages.ModerationRequestDTO{Action: "approve"})
	require.NoError(t, err)
	assert.Equal(t, "approved", approved.ModerationStatus)
	assert.Equal(t, "public", approved.PublicationStatus)
	assert.Equal(t, "awarded", approved.RewardStatus)
	replayedApproval, err := momentService.Moderate(adminCtx, moment.ID, approveKey, &messages.ModerationRequestDTO{Action: "approve"})
	require.NoError(t, err)
	assert.Equal(t, approved, replayedApproval)

	moderation, err = momentService.ListModeration(adminCtx, "challenge", 0)
	require.NoError(t, err)
	assert.Empty(t, moderation.Data)

	feed, err = momentService.List(participantCtx, "feed", "")
	require.NoError(t, err)
	require.Len(t, feed.Items, 1)
	assert.Equal(t, moment.ID, feed.Items[0].ID)

	moderationKey := uuid.NewString()
	decision, err := momentService.Moderate(adminCtx, moment.ID, moderationKey, &messages.ModerationRequestDTO{Action: "deny_points"})
	require.NoError(t, err)
	assert.Equal(t, "reversed", decision.RewardStatus)
	assert.Equal(t, "private", decision.PublicationStatus)
	assert.Equal(t, "rejected", decision.ModerationStatus)

	replayedDecision, err := momentService.Moderate(adminCtx, moment.ID, moderationKey, &messages.ModerationRequestDTO{Action: "deny_points"})
	require.NoError(t, err)
	assert.Equal(t, decision, replayedDecision)

	var persistedUser models.User
	require.NoError(t, TestSuite.DbConn.First(&persistedUser, participant.ID).Error)
	assert.Zero(t, persistedUser.Points)
	var entries []models.PointEntry
	require.NoError(t, TestSuite.DbConn.Where("moment_id = ?", moment.ID).Order("created_at").Find(&entries).Error)
	require.Len(t, entries, 2)
	assert.ElementsMatch(t, []int{25, -25}, []int{entries[0].Delta, entries[1].Delta})
	var audits int64
	require.NoError(t, TestSuite.DbConn.Model(&models.OperationAudit{}).Where("action = ?", "moment.moderated").Count(&audits).Error)
	assert.EqualValues(t, 2, audits) // approve + deny_points

	feed, err = momentService.List(participantCtx, "feed", "")
	require.NoError(t, err)
	assert.Empty(t, feed.Items)
	mine, err := momentService.List(participantCtx, "mine", "")
	require.NoError(t, err)
	require.Len(t, mine.Items, 1)
	assert.NotNil(t, mine.Items[0].ModerationMessage)
	_, err = momentService.ToggleLike(participantCtx, moment.ID, uuid.NewString())
	assertAPIErrorCode(t, err, "NOT_FOUND")
}

func TestMediaMoments_ValidationVisibilityAndProviderFailures(t *testing.T) {
	mediaService, momentService, storage := setupMediaMomentServices(t)
	_, ctx := seedMediaMomentUser(t, "moment-errors@example.com", userEntities.RoleDefault, true)

	tests := []struct {
		name    string
		key     string
		request *messages.CreateUploadIntentRequestDTO
		code    string
	}{
		{"nil request", uuid.NewString(), nil, "INVALID_REQUEST"},
		{"zero bytes", uuid.NewString(), &messages.CreateUploadIntentRequestDTO{ContentType: "image/jpeg", ChecksumSHA256: checksum([]byte("a"))}, "INVALID_REQUEST"},
		{"missing key", "not-a-uuid", &messages.CreateUploadIntentRequestDTO{ContentType: "image/jpeg", Bytes: 1, ChecksumSHA256: checksum([]byte("a"))}, "INVALID_REQUEST"},
		{"unsupported", uuid.NewString(), &messages.CreateUploadIntentRequestDTO{ContentType: "image/gif", Bytes: 1, ChecksumSHA256: checksum([]byte("a"))}, "UNSUPPORTED_MEDIA_TYPE"},
		{"too large", uuid.NewString(), &messages.CreateUploadIntentRequestDTO{ContentType: "image/jpeg", Bytes: maxMediaBytes + 1, ChecksumSHA256: checksum([]byte("a"))}, "IMAGE_TOO_LARGE"},
		{"bad checksum", uuid.NewString(), &messages.CreateUploadIntentRequestDTO{ContentType: "image/jpeg", Bytes: 1, ChecksumSHA256: "bad"}, "INVALID_REQUEST"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := mediaService.CreateUploadIntent(ctx, test.key, test.request)
			assertAPIErrorCode(t, err, test.code)
		})
	}

	storage.configurationErr = errors.New("provider details must stay private")
	_, _, err := mediaService.CreateUploadIntent(ctx, uuid.NewString(), &messages.CreateUploadIntentRequestDTO{
		ContentType:    "image/png",
		Bytes:          1,
		ChecksumSHA256: checksum([]byte("a")),
	})
	assertAPIErrorCode(t, err, "MEDIA_UNAVAILABLE")
	storage.configurationErr = nil
	storage.presignUploadErr = errors.New("signing unavailable")
	presignKey := uuid.NewString()
	presignRequest := &messages.CreateUploadIntentRequestDTO{
		ContentType: "image/jpeg", Bytes: 1, ChecksumSHA256: checksum([]byte("a")),
	}
	_, _, err = mediaService.CreateUploadIntent(ctx, presignKey, presignRequest)
	assertAPIErrorCode(t, err, "MEDIA_UNAVAILABLE")
	storage.presignUploadErr = nil
	recoveredIntent, status, err := mediaService.CreateUploadIntent(ctx, presignKey, presignRequest)
	require.NoError(t, err)
	assert.Equal(t, 201, status)
	assert.NotEmpty(t, recoveredIntent.UploadURL)

	body := mediaMomentImage(t, "image/png")
	intent, _, err := mediaService.CreateUploadIntent(ctx, uuid.NewString(), &messages.CreateUploadIntentRequestDTO{
		ContentType:    "image/png",
		Bytes:          int64(len(body)),
		ChecksumSHA256: checksum(body),
	})
	require.NoError(t, err)
	_, _, err = mediaService.CompleteUpload(ctx, intent.ID, uuid.NewString())
	assertAPIErrorCode(t, err, "UPLOAD_INCOMPLETE")

	assertAPIErrorCode(t, func() error {
		_, listErr := momentService.List(ctx, "unknown", "")
		return listErr
	}(), "INVALID_REQUEST")
	assertAPIErrorCode(t, func() error {
		_, listErr := momentService.List(ctx, "feed", "invalid.cursor")
		return listErr
	}(), "INVALID_REQUEST")
	_, err = momentService.ToggleLike(ctx, uuid.NewString(), uuid.NewString())
	assertAPIErrorCode(t, err, "NOT_FOUND")
	_, _, err = momentService.Create(ctx, uuid.NewString(), nil)
	assertAPIErrorCode(t, err, "INVALID_REQUEST")
	_, _, err = momentService.Create(ctx, uuid.NewString(), &messages.CreateMomentRequestDTO{
		MediaAssetID: "invalid", PublishConsent: true,
	})
	assertAPIErrorCode(t, err, "NOT_FOUND")
	invalidParticipation := "invalid"
	_, _, err = momentService.Create(ctx, uuid.NewString(), &messages.CreateMomentRequestDTO{
		MediaAssetID: intent.ID, ParticipationID: &invalidParticipation,
	})
	assertAPIErrorCode(t, err, "NOT_FOUND")
	_, _, err = momentService.Create(ctx, uuid.NewString(), &messages.CreateMomentRequestDTO{
		MediaAssetID: intent.ID,
	})
	assertAPIErrorCode(t, err, "UPLOAD_STATE_CONFLICT")
}

func TestMediaMoments_AuthenticationAndIdempotencyHelperFailures(t *testing.T) {
	TestSuite.DefaultSetup(t)
	ctx := TestSuite.ContextWithUser(42)
	databaseErr := errors.New("database unavailable")

	t.Run("default actor database and state failures", func(t *testing.T) {
		users := mocks.NewMockUserRepositoryInterface(t)
		users.On("FindByID", mock.Anything, uint64(42)).Return(nil, databaseErr).Once()
		_, err := requireDefaultActor(ctx, users, false)
		assert.ErrorIs(t, err, appErrors.InternalError)

		users.On("FindByID", mock.Anything, uint64(42)).Return(&userEntities.User{
			ID: 42, Role: userEntities.RoleDefault,
		}, nil).Once()
		_, err = requireDefaultActor(ctx, users, false)
		assertAPIErrorCode(t, err, "ONBOARDING_REQUIRED")

		users.On("FindByID", mock.Anything, uint64(42)).Return(&userEntities.User{
			ID: 42, Role: userEntities.RoleAdmin, OnboardingComplete: true,
		}, nil).Once()
		_, err = requireDefaultActor(ctx, users, false)
		assertAPIErrorCode(t, err, "FORBIDDEN")
	})

	t.Run("admin actor database and missing identity failures", func(t *testing.T) {
		users := mocks.NewMockUserRepositoryInterface(t)
		_, err := requireAdminActor(context.Background(), users)
		assertAPIErrorCode(t, err, "UNAUTHENTICATED")
		users.On("FindByID", mock.Anything, uint64(42)).Return(nil, databaseErr).Once()
		_, err = requireAdminActor(ctx, users)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})

	t.Run("global idempotency rejects legacy and changed intent", func(t *testing.T) {
		repository := mocks.NewMockMediaRepository(t)
		key := uuid.NewString()
		repository.On("FindOperation", mock.Anything, uint64(42), key).
			Return(nil, appErrors.ErrNotFound).Once()
		repository.On("FindLegacyOperation", mock.Anything, uint64(42), key).Return(true, nil).Once()
		_, err := findIdempotencyOperation(ctx, repository, 42, key, "moment.create", "hash")
		assertAPIErrorCode(t, err, "IDEMPOTENCY_KEY_REUSED")

		repository.On("FindOperation", mock.Anything, uint64(42), key).
			Return(&mediaEntities.Operation{Operation: "moment.like.toggle", IntentHash: "other"}, nil).Once()
		_, err = findIdempotencyOperation(ctx, repository, 42, key, "moment.create", "hash")
		assertAPIErrorCode(t, err, "IDEMPOTENCY_KEY_REUSED")
	})

	t.Run("reservation and replay hide internal failures", func(t *testing.T) {
		repository := mocks.NewMockMediaRepository(t)
		operation := &mediaEntities.Operation{ID: uuid.NewString()}
		repository.On("CreateOperation", mock.Anything, operation).Return(appErrors.ErrConflict).Once()
		assert.ErrorIs(t, createIdempotencyOperation(ctx, repository, operation), errIdempotencyRace)
		repository.On("CreateOperation", mock.Anything, operation).Return(databaseErr).Once()
		assert.ErrorIs(t, createIdempotencyOperation(ctx, repository, operation), appErrors.InternalError)
		assert.ErrorIs(
			t,
			replayOperationError(&mediaEntities.Operation{ResponseSnapshot: []byte(`{}`)}),
			appErrors.InternalError,
		)
	})
}

func TestMediaMoments_ConcurrentIdempotencyAndUniqueness(t *testing.T) {
	mediaService, momentService, storage := setupMediaMomentServices(t)
	_, ctx := seedMediaMomentUser(t, "moment-concurrency@example.com", userEntities.RoleDefault, true)
	body := mediaMomentImage(t, "image/jpeg")
	request := &messages.CreateUploadIntentRequestDTO{
		ContentType:    "image/jpeg",
		Bytes:          int64(len(body)),
		ChecksumSHA256: checksum(body),
	}
	key := uuid.NewString()

	const workers = 8
	responses := make(chan *messages.UploadIntentResponseDTO, workers)
	errorsChannel := make(chan error, workers)
	var group sync.WaitGroup
	for index := 0; index < workers; index++ {
		group.Add(1)
		go func() {
			defer group.Done()
			response, status, err := mediaService.CreateUploadIntent(ctx, key, request)
			if err == nil && status != 201 {
				err = fmt.Errorf("unexpected status %d", status)
			}
			responses <- response
			errorsChannel <- err
		}()
	}
	group.Wait()
	close(responses)
	close(errorsChannel)
	for err := range errorsChannel {
		require.NoError(t, err)
	}
	var original *messages.UploadIntentResponseDTO
	for response := range responses {
		require.NotNil(t, response)
		if original == nil {
			original = response
			continue
		}
		assert.Equal(t, original, response)
	}
	var assets int64
	require.NoError(t, TestSuite.DbConn.Model(&models.MediaAsset{}).Count(&assets).Error)
	assert.EqualValues(t, 1, assets)

	storage.staged[original.ID] = body
	_, _, err := mediaService.CompleteUpload(ctx, original.ID, uuid.NewString())
	require.NoError(t, err)
	moment, _, err := momentService.Create(ctx, uuid.NewString(), &messages.CreateMomentRequestDTO{
		MediaAssetID:   original.ID,
		PublishConsent: true,
	})
	require.NoError(t, err)

	likeErrors := make(chan error, 2)
	for index := 0; index < 2; index++ {
		go func() {
			_, toggleErr := momentService.ToggleLike(ctx, moment.ID, uuid.NewString())
			likeErrors <- toggleErr
		}()
	}
	for index := 0; index < 2; index++ {
		require.NoError(t, <-likeErrors)
	}
	var likes int64
	require.NoError(t, TestSuite.DbConn.Model(&models.MomentLike{}).Where("moment_id = ?", moment.ID).Count(&likes).Error)
	assert.Zero(t, likes)

	secondAsset := createAvailableAsset(t, mediaService, storage, ctx, "image/png")
	createErrors := make(chan error, 2)
	for _, assetID := range []string{secondAsset.ID, secondAsset.ID} {
		assetID := assetID
		go func() {
			_, _, createErr := momentService.Create(ctx, uuid.NewString(), &messages.CreateMomentRequestDTO{
				MediaAssetID:   assetID,
				PublishConsent: false,
			})
			createErrors <- createErr
		}()
	}
	var conflicts int
	for index := 0; index < 2; index++ {
		if createErr := <-createErrors; createErr != nil {
			conflicts++
		}
	}
	assert.Equal(t, 1, conflicts)
}

func TestMediaMoments_GroupRoleAndRemovedAuthorChangesAreImmediate(t *testing.T) {
	mediaService, momentService, storage := setupMediaMomentServices(t)
	owner, ownerCtx := seedMediaMomentUser(t, "moment-group-owner@example.com", userEntities.RoleDefault, true)
	viewer, viewerCtx := seedMediaMomentUser(t, "moment-group-viewer@example.com", userEntities.RoleDefault, true)
	TestSuite.TruncateTable(t, &models.Group{})
	require.NoError(t, TestSuite.DbConn.Create(&models.Group{Name: "Grupo Um"}).Error)
	require.NoError(t, TestSuite.DbConn.Create(&models.Group{Name: "Grupo Dois"}).Error)
	var groups []models.Group
	require.NoError(t, TestSuite.DbConn.Order("id").Find(&groups).Error)
	require.Len(t, groups, 2)
	require.NoError(t, TestSuite.DbConn.Model(&models.User{}).
		Where("id IN ?", []uint64{owner.ID, viewer.ID}).
		Update("group_id", groups[0].ID).Error)

	asset := createAvailableAsset(t, mediaService, storage, ownerCtx, "image/jpeg")
	moment, _, err := momentService.Create(ownerCtx, uuid.NewString(), &messages.CreateMomentRequestDTO{
		MediaAssetID:   asset.ID,
		PublishConsent: true,
	})
	require.NoError(t, err)
	groupPage, err := momentService.List(viewerCtx, "group", "")
	require.NoError(t, err)
	require.Len(t, groupPage.Items, 1)
	assert.Equal(t, moment.ID, groupPage.Items[0].ID)

	require.NoError(t, TestSuite.DbConn.Model(&models.User{}).Where("id = ?", viewer.ID).
		Update("group_id", groups[1].ID).Error)
	groupPage, err = momentService.List(viewerCtx, "group", "")
	require.NoError(t, err)
	assert.Empty(t, groupPage.Items)

	require.NoError(t, TestSuite.DbConn.Model(&models.User{}).Where("id = ?", owner.ID).
		Update("role", string(userEntities.RoleEventManager)).Error)
	feed, err := momentService.List(viewerCtx, "feed", "")
	require.NoError(t, err)
	assert.Empty(t, feed.Items)
	_, err = momentService.List(ownerCtx, "mine", "")
	assertAPIErrorCode(t, err, "FORBIDDEN")

	require.NoError(t, TestSuite.DbConn.Delete(&models.User{}, owner.ID).Error)
	var historical int64
	require.NoError(t, TestSuite.DbConn.Model(&models.Moment{}).Where("id = ?", moment.ID).Count(&historical).Error)
	assert.EqualValues(t, 1, historical)
}

func TestMediaMoments_CompletionFailuresAreDurableAndRecoverable(t *testing.T) {
	mediaService, _, storage := setupMediaMomentServices(t)
	_, ctx := seedMediaMomentUser(t, "media-recovery@example.com", userEntities.RoleDefault, true)

	createIntent := func(t *testing.T, body []byte) *messages.UploadIntentResponseDTO {
		t.Helper()
		intent, _, err := mediaService.CreateUploadIntent(ctx, uuid.NewString(), &messages.CreateUploadIntentRequestDTO{
			ContentType:    "image/jpeg",
			Bytes:          int64(len(body)),
			ChecksumSHA256: checksum(body),
		})
		require.NoError(t, err)
		storage.staged[intent.ID] = body
		return intent
	}

	t.Run("malformed image and retry preserve terminal error", func(t *testing.T) {
		body := []byte("not-a-real-jpeg-but-long-enough")
		intent := createIntent(t, body)
		key := uuid.NewString()
		_, _, err := mediaService.CompleteUpload(ctx, intent.ID, key)
		assertAPIErrorCode(t, err, "IMAGE_INVALID")
		_, _, err = mediaService.CompleteUpload(ctx, intent.ID, key)
		assertAPIErrorCode(t, err, "IMAGE_INVALID")
		_, _, err = mediaService.CompleteUpload(ctx, intent.ID, uuid.NewString())
		assertAPIErrorCode(t, err, "UPLOAD_STATE_CONFLICT")
	})

	t.Run("expired intent is gone and replayable", func(t *testing.T) {
		body := mediaMomentImage(t, "image/jpeg")
		intent := createIntent(t, body)
		mediaService.now = func() time.Time { return mediaMomentNow.Add(uploadIntentLifetime) }
		key := uuid.NewString()
		_, _, err := mediaService.CompleteUpload(ctx, intent.ID, key)
		assertAPIErrorCode(t, err, "UPLOAD_EXPIRED")
		_, _, err = mediaService.CompleteUpload(ctx, intent.ID, key)
		assertAPIErrorCode(t, err, "UPLOAD_EXPIRED")
		mediaService.now = func() time.Time { return mediaMomentNow }
	})

	t.Run("provider failures release lease for same-key resume", func(t *testing.T) {
		body := mediaMomentImage(t, "image/jpeg")
		intent := createIntent(t, body)
		key := uuid.NewString()
		storage.headErr = errors.New("provider unavailable")
		_, _, err := mediaService.CompleteUpload(ctx, intent.ID, key)
		assertAPIErrorCode(t, err, "MEDIA_UNAVAILABLE")
		storage.headErr = nil
		completed, status, err := mediaService.CompleteUpload(ctx, intent.ID, key)
		require.NoError(t, err)
		assert.Equal(t, 200, status)
		assert.Equal(t, "available", completed.State)
		replayed, replayStatus, err := mediaService.CompleteUpload(ctx, intent.ID, key)
		require.NoError(t, err)
		assert.Equal(t, 200, replayStatus)
		assert.Equal(t, completed.ID, replayed.ID)
	})

	t.Run("bucket versioning is mandatory", func(t *testing.T) {
		body := mediaMomentImage(t, "image/jpeg")
		intent := createIntent(t, body)
		storage.headVersion = ""
		_, _, err := mediaService.CompleteUpload(ctx, intent.ID, uuid.NewString())
		assertAPIErrorCode(t, err, "MEDIA_UNAVAILABLE")
		storage.headVersion = "staging-version-1"
	})

	t.Run("head checksum mismatch is terminal", func(t *testing.T) {
		body := mediaMomentImage(t, "image/jpeg")
		intent := createIntent(t, body)
		mutated := append([]byte(nil), body...)
		mutated[len(mutated)-1] ^= 0xff
		storage.staged[intent.ID] = mutated
		_, _, err := mediaService.CompleteUpload(ctx, intent.ID, uuid.NewString())
		assertAPIErrorCode(t, err, "IMAGE_INVALID")
	})

	t.Run("bounded download maps oversized body", func(t *testing.T) {
		body := mediaMomentImage(t, "image/jpeg")
		intent := createIntent(t, body)
		storage.downloadErr = mediaEntities.ErrObjectTooLarge
		_, _, err := mediaService.CompleteUpload(ctx, intent.ID, uuid.NewString())
		assertAPIErrorCode(t, err, "IMAGE_TOO_LARGE")
		storage.downloadErr = nil
	})

	t.Run("final write can resume", func(t *testing.T) {
		body := mediaMomentImage(t, "image/jpeg")
		intent := createIntent(t, body)
		key := uuid.NewString()
		storage.putErr = errors.New("provider unavailable")
		_, _, err := mediaService.CompleteUpload(ctx, intent.ID, key)
		assertAPIErrorCode(t, err, "MEDIA_UNAVAILABLE")
		storage.putErr = nil
		_, _, err = mediaService.CompleteUpload(ctx, intent.ID, key)
		require.NoError(t, err)
	})

	t.Run("foreign and malformed identifiers are uniformly hidden", func(t *testing.T) {
		body := mediaMomentImage(t, "image/jpeg")
		intent := createIntent(t, body)
		_, foreignCtx := seedMediaMomentUser(t, "media-foreign@example.com", userEntities.RoleDefault, true)
		_, _, err := mediaService.CompleteUpload(foreignCtx, intent.ID, uuid.NewString())
		assertAPIErrorCode(t, err, "NOT_FOUND")
		_, _, err = mediaService.CompleteUpload(ctx, "not-a-uuid", uuid.NewString())
		assertAPIErrorCode(t, err, "NOT_FOUND")
	})
}

func TestMediaMoments_ConcurrentSameKeyConfirmationWaitsAndReplays(t *testing.T) {
	mediaService, _, storage := setupMediaMomentServices(t)
	_, ctx := seedMediaMomentUser(t, "media-complete-race@example.com", userEntities.RoleDefault, true)
	body := mediaMomentImage(t, "image/jpeg")
	intent, _, err := mediaService.CreateUploadIntent(ctx, uuid.NewString(), &messages.CreateUploadIntentRequestDTO{
		ContentType: "image/jpeg", Bytes: int64(len(body)), ChecksumSHA256: checksum(body),
	})
	require.NoError(t, err)
	storage.staged[intent.ID] = body
	storage.headStarted = make(chan struct{})
	storage.headRelease = make(chan struct{})
	key := uuid.NewString()
	type result struct {
		asset  *messages.MediaAssetResponseDTO
		status int
		err    error
	}
	results := make(chan result, 2)
	go func() {
		asset, status, completeErr := mediaService.CompleteUpload(ctx, intent.ID, key)
		results <- result{asset, status, completeErr}
	}()
	<-storage.headStarted
	go func() {
		asset, status, completeErr := mediaService.CompleteUpload(ctx, intent.ID, key)
		results <- result{asset, status, completeErr}
	}()
	close(storage.headRelease)
	first := <-results
	second := <-results
	require.NoError(t, first.err)
	require.NoError(t, second.err)
	assert.Equal(t, 200, first.status)
	assert.Equal(t, first.asset.ID, second.asset.ID)
	assert.Equal(t, first.asset.State, second.asset.State)
	require.NotNil(t, first.asset.AvailableAt)
	require.NotNil(t, second.asset.AvailableAt)
	assert.True(t, first.asset.AvailableAt.Equal(*second.asset.AvailableAt))
}

func TestMediaMoments_EligibilityIdempotencyAndCorrectiveModeration(t *testing.T) {
	mediaService, momentService, storage := setupMediaMomentServices(t)
	owner, ownerCtx := seedMediaMomentUser(t, "moment-eligibility@example.com", userEntities.RoleDefault, true)
	_, adminCtx := seedMediaMomentUser(t, "moment-eligibility-admin@example.com", userEntities.RoleAdmin, true)

	privateAsset := createAvailableAsset(t, mediaService, storage, ownerCtx, "image/jpeg")
	privateParticipation := seedChallengeParticipation(t, owner.ID, true, "active")
	privateMoment, _, err := momentService.Create(ownerCtx, uuid.NewString(), &messages.CreateMomentRequestDTO{
		MediaAssetID:    privateAsset.ID,
		PublishConsent:  false,
		ParticipationID: &privateParticipation,
	})
	require.NoError(t, err)
	assert.Equal(t, "private", privateMoment.PublicationStatus)
	assert.Zero(t, privateMoment.PointsAwarded)
	_, err = momentService.Moderate(adminCtx, privateMoment.ID, uuid.NewString(), &messages.ModerationRequestDTO{
		Action: "deny_points",
	})
	assertAPIErrorCode(t, err, "MODERATION_ACTION_INVALID")
	deleted, err := momentService.Moderate(adminCtx, privateMoment.ID, uuid.NewString(), &messages.ModerationRequestDTO{
		Action: "delete_photo",
	})
	require.NoError(t, err)
	assert.Equal(t, "deleted", deleted.PhotoStatus)
	replayedTerminal, err := momentService.Moderate(adminCtx, privateMoment.ID, uuid.NewString(), &messages.ModerationRequestDTO{
		Action: "delete_photo",
	})
	require.NoError(t, err)
	assert.Equal(t, deleted.PhotoStatus, replayedTerminal.PhotoStatus)

	for name, configure := range map[string]func(uint64) string{
		"sharing disabled":  func(userID uint64) string { return seedChallengeParticipation(t, userID, false, "active") },
		"activity archived": func(userID uint64) string { return seedChallengeParticipation(t, userID, true, "archived") },
		"participation cancelled": func(userID uint64) string {
			id := seedChallengeParticipation(t, userID, true, "active")
			require.NoError(t, TestSuite.DbConn.Model(&models.Participation{}).Where("id = ?", id).
				Update("status", string(gameEntities.ParticipationStatusCancelled)).Error)
			return id
		},
	} {
		t.Run(name, func(t *testing.T) {
			asset := createAvailableAsset(t, mediaService, storage, ownerCtx, "image/png")
			participationID := configure(owner.ID)
			_, _, createErr := momentService.Create(ownerCtx, uuid.NewString(), &messages.CreateMomentRequestDTO{
				MediaAssetID:    asset.ID,
				PublishConsent:  true,
				ParticipationID: &participationID,
			})
			assertAPIErrorCode(t, createErr, "MOMENT_NOT_ELIGIBLE")
		})
	}

	firstAsset := createAvailableAsset(t, mediaService, storage, ownerCtx, "image/jpeg")
	secondAsset := createAvailableAsset(t, mediaService, storage, ownerCtx, "image/jpeg")
	key := uuid.NewString()
	_, _, err = momentService.Create(ownerCtx, key, &messages.CreateMomentRequestDTO{
		MediaAssetID: firstAsset.ID, PublishConsent: false,
	})
	require.NoError(t, err)
	_, _, err = momentService.Create(ownerCtx, key, &messages.CreateMomentRequestDTO{
		MediaAssetID: secondAsset.ID, PublishConsent: false,
	})
	assertAPIErrorCode(t, err, "IDEMPOTENCY_KEY_REUSED")
	_, _, err = momentService.Create(ownerCtx, uuid.NewString(), &messages.CreateMomentRequestDTO{
		MediaAssetID: firstAsset.ID, PublishConsent: false,
	})
	assertAPIErrorCode(t, err, "MOMENT_ALREADY_CREATED")

	_, err = momentService.ListModeration(ownerCtx, "general", 0)
	assertAPIErrorCode(t, err, "FORBIDDEN")
	_, err = momentService.ListModeration(adminCtx, "invalid", 0)
	assertAPIErrorCode(t, err, "INVALID_REQUEST")
	_, err = momentService.Moderate(adminCtx, privateMoment.ID, uuid.NewString(), &messages.ModerationRequestDTO{Action: "reason"})
	assertAPIErrorCode(t, err, "INVALID_REQUEST")
}

func TestMediaMoments_CursorPaginationAndPreservedMineProjection(t *testing.T) {
	_, momentService, _ := setupMediaMomentServices(t)
	owner, ownerCtx := seedMediaMomentUser(t, "moment-cursor@example.com", userEntities.RoleDefault, true)
	versionID := "immutable-version"
	for index := 0; index < 21; index++ {
		assetID := uuid.NewString()
		capturedAt := mediaMomentNow.Add(-time.Duration(index) * time.Second)
		require.NoError(t, TestSuite.DbConn.Create(&models.MediaAsset{
			ID:               assetID,
			OwnerUserID:      owner.ID,
			Provider:         "s3",
			Bucket:           "dnj-private-test",
			StagingObjectKey: "staging/" + uuid.NewString(),
			FinalObjectKey:   "media/" + uuid.NewString() + ".jpg",
			FinalVersionID:   &versionID,
			ContentType:      "image/jpeg",
			Bytes:            1,
			ChecksumSHA256:   checksum([]byte("a")),
			State:            string(mediaEntities.AssetAvailable),
			UploadExpiresAt:  mediaMomentNow,
			RetentionDueAt:   mediaMomentNow.Add(90 * 24 * time.Hour),
			CreatedAt:        capturedAt,
			UpdatedAt:        capturedAt,
		}).Error)
		require.NoError(t, TestSuite.DbConn.Create(&models.Moment{
			ID:                uuid.NewString(),
			UserID:            owner.ID,
			MediaAssetID:      assetID,
			Origin:            "free",
			PublicationStatus: "public",
			ModerationStatus:  "approved",
			RewardStatus:      "not_applicable",
			CapturedAt:        capturedAt,
			CreatedAt:         capturedAt,
			UpdatedAt:         capturedAt,
		}).Error)
	}

	first, err := momentService.List(ownerCtx, "feed", "")
	require.NoError(t, err)
	require.Len(t, first.Items, 20)
	require.NotNil(t, first.NextCursor)
	second, err := momentService.List(ownerCtx, "feed", *first.NextCursor)
	require.NoError(t, err)
	require.Len(t, second.Items, 1)
	assert.Nil(t, second.NextCursor)
	_, err = momentService.List(ownerCtx, "feed", *first.NextCursor+"tampered")
	assertAPIErrorCode(t, err, "INVALID_REQUEST")

	deletedAssetID := uuid.NewString()
	now := mediaMomentNow.Add(time.Minute)
	require.NoError(t, TestSuite.DbConn.Create(&models.MediaAsset{
		ID:               deletedAssetID,
		OwnerUserID:      owner.ID,
		Provider:         "s3",
		Bucket:           "dnj-private-test",
		StagingObjectKey: "staging/" + uuid.NewString(),
		FinalObjectKey:   "media/" + uuid.NewString(),
		ContentType:      "image/jpeg",
		Bytes:            1,
		ChecksumSHA256:   checksum([]byte("a")),
		State:            string(mediaEntities.AssetDeleted),
		UploadExpiresAt:  now,
		RetentionDueAt:   now,
		DeletedAt:        &now,
		CreatedAt:        now,
		UpdatedAt:        now,
	}).Error)
	require.NoError(t, TestSuite.DbConn.Create(&models.Moment{
		ID:                uuid.NewString(),
		UserID:            owner.ID,
		MediaAssetID:      deletedAssetID,
		Origin:            "free",
		PublicationStatus: "private",
		ModerationStatus:  "rejected",
		RewardStatus:      "not_applicable",
		CapturedAt:        now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}).Error)
	mine, err := momentService.List(ownerCtx, "mine", "")
	require.NoError(t, err)
	require.NotEmpty(t, mine.Items)
	assert.Empty(t, mine.Items[0].ImageURL)
	assert.NotNil(t, mine.Items[0].ModerationMessage)
}

func assertAPIErrorCode(t *testing.T, err error, code string) {
	t.Helper()
	require.Error(t, err)
	var apiErr *appErrors.APIServiceError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, code, apiErr.Code)
}
