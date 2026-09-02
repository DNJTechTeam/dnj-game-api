package repositories

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	mediaEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/media/entities"
	momentEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/moment/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedChallengeParticipationRepo(t *testing.T, ctx context.Context, userID uint64, momentPoints int) (activityID, participationID string) {
	t.Helper()
	activityID = uuid.NewString()
	runID := uuid.NewString()
	qrID := uuid.NewString()
	participationID = uuid.NewString()
	now := time.Now().UTC()
	require.NoError(t, TestSuite.DbConn.Create(&models.Activity{
		ID: activityID, Slug: "moment-repo-" + activityID, Name: "Desafio", Kind: "challenge",
		Status: "active", MomentPoints: momentPoints, AllowsMoment: true, CreatedAt: now, UpdatedAt: now,
	}).Error)
	require.NoError(t, TestSuite.DbConn.Create(&models.ActivityRun{
		ID: runID, ActivityID: activityID, StartedBy: userID, Status: "active",
		PointRules: json.RawMessage(`{"first":0,"second":0,"third":0,"participation":0}`),
		CreatedAt:  now, UpdatedAt: now,
	}).Error)
	require.NoError(t, TestSuite.DbConn.Create(&models.ActivityRunQRCode{
		ID: qrID, ActivityID: activityID, ActivityRunID: runID, TokenHash: testChecksum(),
		ExpiresAt: now.Add(time.Hour), Status: "active", CreatedAt: now, UpdatedAt: now,
	}).Error)
	require.NoError(t, TestSuite.DbConn.Create(&models.Participation{
		ID: participationID, UserID: userID, ActivityID: activityID, ActivityRunID: runID, QRCodeID: qrID,
		CheckedInAt: now, Status: "active", CanShareMoment: true, CreatedAt: now,
	}).Error)
	return activityID, participationID
}

func seedChallengeMoment(
	t *testing.T,
	ctx context.Context,
	mediaRepo *MediaRepository,
	momentRepo *MomentRepository,
	userID uint64,
	activityID, participationID string,
	publicationStatus string,
) *momentEntities.Moment {
	t.Helper()
	asset := seedMediaAsset(t, ctx, mediaRepo, userID, string(mediaEntities.AssetAvailable), time.Now().UTC().Add(time.Hour))
	now := time.Now().UTC()
	moment := &momentEntities.Moment{
		ID: uuid.NewString(), UserID: userID, ParticipationID: &participationID, ActivityID: &activityID,
		MediaAssetID: asset.ID, Origin: momentEntities.OriginChallenge,
		PublicationStatus: momentEntities.PublicationStatus(publicationStatus),
		ModerationStatus:  momentEntities.ModerationApproved, RewardStatus: momentEntities.RewardDenied,
		CapturedAt: now, CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, momentRepo.CreateMoment(ctx, moment))
	return moment
}

// TestMediaMoments_AwardReverseAndModerationRepositoryLifecycle exercises the moment
// award/reversal/moderation repository methods directly, including branches that the
// service layer never reaches through the public API: awarding zero points (an Activity
// configured with momentPoints=0), and reversing an award that is already reversed (a
// direct-repository idempotency guarantee that ApplyModeration's own pre-check makes
// unreachable through the HTTP-facing flow, since it never re-invokes the reversal once
// the moment's reward_status is already "reversed").
func TestMediaMoments_AwardReverseAndModerationRepositoryLifecycle(t *testing.T) {
	ctx := context.Background()
	TestSuite.TruncateTable(t, &models.Notification{})
	TestSuite.TruncateTable(t, &models.NotificationPreference{})
	TestSuite.TruncateTable(t, &models.MomentModerationDecision{})
	TestSuite.TruncateTable(t, &models.MomentLike{})
	TestSuite.TruncateTable(t, &models.Moment{})
	TestSuite.TruncateTable(t, &models.MediaAsset{})
	TestSuite.TruncateTable(t, &models.Participation{})
	TestSuite.TruncateTable(t, &models.ActivityRunQRCode{})
	TestSuite.TruncateTable(t, &models.ActivityRun{})
	TestSuite.TruncateTable(t, &models.Activity{})
	TestSuite.TruncateTable(t, &models.User{})

	mediaRepo := &MediaRepository{BaseRepository: NewBaseRepository[models.MediaAsset](TestSuite.DbConn)}
	momentRepo := &MomentRepository{BaseRepository: NewBaseRepository[models.Moment](TestSuite.DbConn)}
	owner := seedUser(t, ctx, "moment-award-owner@example.com")
	now := time.Now().UTC()

	// ── AwardMoment: zero-points activity denies the reward without a ledger entry ──
	zeroActivityID, zeroParticipationID := seedChallengeParticipationRepo(t, ctx, owner.ID, 0)
	zeroMoment := seedChallengeMoment(t, ctx, mediaRepo, momentRepo, owner.ID, zeroActivityID, zeroParticipationID, "public")
	require.NoError(t, momentRepo.AwardMoment(ctx, zeroMoment.ID, owner.ID, zeroActivityID, 0, now))
	refreshed, err := momentRepo.FindMoment(ctx, zeroMoment.ID, owner.ID, false)
	require.NoError(t, err)
	assert.Equal(t, momentEntities.RewardDenied, refreshed.RewardStatus)

	// ── AwardMoment: success path, then a duplicate award attempt is a conflict ──
	activityID, participationID := seedChallengeParticipationRepo(t, ctx, owner.ID, 30)
	moment := seedChallengeMoment(t, ctx, mediaRepo, momentRepo, owner.ID, activityID, participationID, "public")
	require.NoError(t, momentRepo.AwardMoment(ctx, moment.ID, owner.ID, activityID, 30, now))
	afterAward, err := momentRepo.FindMoment(ctx, moment.ID, owner.ID, false)
	require.NoError(t, err)
	assert.Equal(t, momentEntities.RewardAwarded, afterAward.RewardStatus)
	assert.Equal(t, 30, afterAward.PointsAwarded)
	var participation models.Participation
	require.NoError(t, TestSuite.DbConn.Where("id = ?", participationID).Take(&participation).Error)
	assert.False(t, participation.CanShareMoment)

	var awardNotificationCount int64
	require.NoError(t, TestSuite.DbConn.
		Model(&models.Notification{}).
		Where("user_id = ? AND category = ? AND source_id = ?", owner.ID, "points", moment.ID).
		Count(&awardNotificationCount).Error)
	assert.Zero(t, awardNotificationCount)

	err = momentRepo.AwardMoment(ctx, moment.ID, owner.ID, activityID, 30, now)
	assert.ErrorIs(t, err, appErrors.ErrConflict)

	// ── ReverseMomentAward: success, reflected atomically on the user's balance ──
	userBeforeReverse, err := TestSuite.UserRepository.FindByID(ctx, owner.ID)
	require.NoError(t, err)
	require.GreaterOrEqual(t, userBeforeReverse.Points, 30)

	reversed, err := momentRepo.ReverseMomentAward(ctx, moment.ID, owner.ID, now)
	require.NoError(t, err)
	assert.True(t, reversed)
	userAfterReverse, err := TestSuite.UserRepository.FindByID(ctx, owner.ID)
	require.NoError(t, err)
	assert.Equal(t, userBeforeReverse.Points-30, userAfterReverse.Points)

	var reversalNotificationCount int64
	require.NoError(t, TestSuite.DbConn.
		Model(&models.Notification{}).
		Where("user_id = ? AND category = ? AND source_id = ?", owner.ID, "points", moment.ID).
		Count(&reversalNotificationCount).Error)
	assert.Zero(t, reversalNotificationCount)

	// Reversing an already-reversed award is a safe, durable no-op — not a double-decrement.
	reversedAgain, err := momentRepo.ReverseMomentAward(ctx, moment.ID, owner.ID, now)
	require.NoError(t, err)
	assert.False(t, reversedAgain)
	userAfterSecondReverse, err := TestSuite.UserRepository.FindByID(ctx, owner.ID)
	require.NoError(t, err)
	assert.Equal(t, userAfterReverse.Points, userAfterSecondReverse.Points)

	// ── ApplyModeration: deny_points reverses the award and locks the linked Participation ──
	activityID2, participationID2 := seedChallengeParticipationRepo(t, ctx, owner.ID, 40)
	moderationMoment := seedChallengeMoment(t, ctx, mediaRepo, momentRepo, owner.ID, activityID2, participationID2, "public")
	require.NoError(t, momentRepo.AwardMoment(ctx, moderationMoment.ID, owner.ID, activityID2, 40, now))

	denied, deniedAsset, changed, err := momentRepo.ApplyModeration(ctx, moderationMoment.ID, "deny_points", 0, uuid.NewString(), now)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, momentEntities.PublicationPrivate, denied.PublicationStatus)
	assert.Equal(t, momentEntities.ModerationRejected, denied.ModerationStatus)
	assert.Equal(t, mediaEntities.AssetAvailable, deniedAsset.State)

	var moderationNotification models.Notification
	require.NoError(t, TestSuite.DbConn.
		Where("user_id = ? AND category = ? AND source_id = ?", owner.ID, "moment_moderation", moderationMoment.ID).
		Take(&moderationNotification).Error)
	assert.Equal(t, "unread", moderationNotification.State)

	// deny_points on a moment with no award in effect is a conflict.
	_, _, _, err = momentRepo.ApplyModeration(ctx, zeroMoment.ID, "deny_points", 0, uuid.NewString(), now)
	assert.ErrorIs(t, err, appErrors.ErrConflict)

	// Applying the same decision again is a durable, idempotent no-op (changed=false).
	_, _, changedAgain, err := momentRepo.ApplyModeration(ctx, moderationMoment.ID, "deny_points", 0, uuid.NewString(), now)
	require.NoError(t, err)
	assert.False(t, changedAgain)

	// A second, different decision (delete_photo) on an already-rejected Moment still
	// changes state (the asset is actually deleted) and must still notify the owner —
	// the moderation-status transition alone no longer gates the notification.
	_, secondDecisionAsset, secondDecisionChanged, err := momentRepo.ApplyModeration(ctx, moderationMoment.ID, "delete_photo", 0, uuid.NewString(), now)
	require.NoError(t, err)
	assert.True(t, secondDecisionChanged)
	assert.Equal(t, mediaEntities.AssetDeleted, secondDecisionAsset.State)
	var deletePhotoNotification models.Notification
	require.NoError(t, TestSuite.DbConn.
		Where("user_id = ? AND category = ? AND source_id = ? AND body = ?",
			owner.ID, "moment_moderation", moderationMoment.ID, "Sua foto foi removida da galeria.").
		Take(&deletePhotoNotification).Error)
	var moderationNotificationCount int64
	require.NoError(t, TestSuite.DbConn.Model(&models.Notification{}).
		Where("user_id = ? AND category = ? AND source_id = ?", owner.ID, "moment_moderation", moderationMoment.ID).
		Count(&moderationNotificationCount).Error)
	assert.Equal(t, int64(2), moderationNotificationCount)

	// ── ApplyModeration: delete_photo marks the asset deleted (idempotent on retry) ──
	freeAsset := seedMediaAsset(t, ctx, mediaRepo, owner.ID, string(mediaEntities.AssetAvailable), now.Add(time.Hour))
	freeMoment := &momentEntities.Moment{
		ID: uuid.NewString(), UserID: owner.ID, MediaAssetID: freeAsset.ID, Origin: momentEntities.OriginFree,
		PublicationStatus: momentEntities.PublicationPublic, ModerationStatus: momentEntities.ModerationApproved,
		RewardStatus: momentEntities.RewardNotApplicable, CapturedAt: now, CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, momentRepo.CreateMoment(ctx, freeMoment))
	deletedMoment, deletedAsset, deletedChanged, err := momentRepo.ApplyModeration(ctx, freeMoment.ID, "delete_photo", 0, uuid.NewString(), now)
	require.NoError(t, err)
	assert.True(t, deletedChanged)
	assert.Equal(t, mediaEntities.AssetDeleted, deletedAsset.State)
	assert.Equal(t, momentEntities.ModerationRejected, deletedMoment.ModerationStatus)

	_, _, deletedAgainChanged, err := momentRepo.ApplyModeration(ctx, freeMoment.ID, "delete_photo", 0, uuid.NewString(), now)
	require.NoError(t, err)
	assert.False(t, deletedAgainChanged)

	// ── CreateModerationDecision: duplicate id is a durable no-op, not an error ──
	decisionID := uuid.NewString()
	created, err := momentRepo.CreateModerationDecision(ctx, &momentEntities.ModerationDecision{
		ID: decisionID, MomentID: moderationMoment.ID, ActorUserID: owner.ID, Action: "deny_points",
		IdempotencyKey: uuid.NewString(), CreatedAt: now,
	})
	require.NoError(t, err)
	assert.True(t, created)
	createdAgain, err := momentRepo.CreateModerationDecision(ctx, &momentEntities.ModerationDecision{
		ID: decisionID, MomentID: moderationMoment.ID, ActorUserID: owner.ID, Action: "deny_points",
		IdempotencyKey: uuid.NewString(), CreatedAt: now,
	})
	require.NoError(t, err)
	assert.False(t, createdAgain)
}

// TestNotifications_PointsNotificationRespectsPreferenceOptOut confirms a user who
// disabled the "points" category never receives an AwardMoment-derived notification,
// while moment_moderation (which cannot be disabled) still reaches the same account.
func TestNotifications_PointsNotificationRespectsPreferenceOptOut(t *testing.T) {
	ctx := context.Background()
	TestSuite.TruncateTable(t, &models.Notification{})
	TestSuite.TruncateTable(t, &models.NotificationPreference{})
	TestSuite.TruncateTable(t, &models.MomentModerationDecision{})
	TestSuite.TruncateTable(t, &models.MomentLike{})
	TestSuite.TruncateTable(t, &models.Moment{})
	TestSuite.TruncateTable(t, &models.MediaAsset{})
	TestSuite.TruncateTable(t, &models.Participation{})
	TestSuite.TruncateTable(t, &models.ActivityRunQRCode{})
	TestSuite.TruncateTable(t, &models.ActivityRun{})
	TestSuite.TruncateTable(t, &models.Activity{})
	TestSuite.TruncateTable(t, &models.User{})

	mediaRepo := &MediaRepository{BaseRepository: NewBaseRepository[models.MediaAsset](TestSuite.DbConn)}
	momentRepo := &MomentRepository{BaseRepository: NewBaseRepository[models.Moment](TestSuite.DbConn)}
	owner := seedUser(t, ctx, "moment-award-optout@example.com")
	now := time.Now().UTC()
	require.NoError(t, TestSuite.DbConn.Create(&models.NotificationPreference{
		UserID: owner.ID, PointsEnabled: false, AnnouncementEnabled: true, UpdatedAt: now,
	}).Error)

	activityID, participationID := seedChallengeParticipationRepo(t, ctx, owner.ID, 25)
	moment := seedChallengeMoment(t, ctx, mediaRepo, momentRepo, owner.ID, activityID, participationID, "public")
	require.NoError(t, momentRepo.AwardMoment(ctx, moment.ID, owner.ID, activityID, 25, now))

	var pointsCount int64
	require.NoError(t, TestSuite.DbConn.Model(&models.Notification{}).
		Where("user_id = ? AND category = ?", owner.ID, "points").Count(&pointsCount).Error)
	assert.Zero(t, pointsCount)

	_, _, changed, err := momentRepo.ApplyModeration(ctx, moment.ID, "deny_points", 0, uuid.NewString(), now)
	require.NoError(t, err)
	assert.True(t, changed)

	var moderationCount int64
	require.NoError(t, TestSuite.DbConn.Model(&models.Notification{}).
		Where("user_id = ? AND category = ?", owner.ID, "moment_moderation").Count(&moderationCount).Error)
	assert.Equal(t, int64(1), moderationCount)

	var reversalCount int64
	require.NoError(t, TestSuite.DbConn.Model(&models.Notification{}).
		Where("user_id = ? AND category = ?", owner.ID, "points").Count(&reversalCount).Error)
	assert.Zero(t, reversalCount)
}
