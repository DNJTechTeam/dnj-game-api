package services

import (
	"time"

	"github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/repositories"
	"github.com/dnjtechteam/dnj-game-api/internal/presentation/api/handlers"
	"github.com/dnjtechteam/dnj-game-api/internal/presentation/api/routers"
	"github.com/gin-gonic/gin"

	"testing"
)

// e2eRig wires every business-domain service to a real Gin engine backed by
// the shared TestSuite (real Postgres via testcontainers), mirroring the
// per-domain *_http_integration_test.go pattern but registering every route
// at once so a single test can walk a journey across domains (auth -> groups
// -> games -> media/moments -> notifications -> admin).
type e2eRig struct {
	Engine   *gin.Engine
	JWT      interfaces.JwtServiceInterface
	Identity *IdentityService
	Storage  *memoryMediaStorage
}

// setupE2ERig truncates every table touched by a journey and builds a fresh
// rig. Call once per journey test.
func setupE2ERig(t *testing.T) *e2eRig {
	t.Helper()
	TestSuite.DefaultSetup(t)
	t.Setenv("S3_BUCKET", "dnj-e2e-test")
	t.Setenv("DNJ_CURSOR_HMAC_SECRET", "e2e-cursor-secret")
	t.Setenv("DNJ_MEDIA_RETENTION_ANCHOR_AT", time.Now().UTC().Format(time.RFC3339))

	for _, model := range []interface{ TableName() string }{
		&models.Notification{}, &models.NotificationPreference{},
		&models.MomentModerationDecision{}, &models.MomentLike{}, &models.Moment{},
		&models.MediaCleanupJob{}, &models.MediaProcessingClaim{}, &models.MediaAsset{},
		&models.IdempotencyOperation{},
		&models.GroupInvite{}, &models.GroupMembership{},
		&models.UserFavorite{},
		&models.ManagerOperation{}, &models.ParticipantOperation{}, &models.PointEntry{},
		&models.ActivityRunParticipant{}, &models.Participation{}, &models.ActivityRunQRCode{}, &models.ActivityRun{},
		&models.ActivityManagerAssignment{}, &models.AdminOperation{}, &models.OperationAudit{},
		&models.Activity{}, &models.Space{},
		&models.RefreshSession{}, &models.GoogleIdentity{}, &models.EmailSignupCode{},
		&models.Group{}, &models.User{},
	} {
		TestSuite.TruncateTable(t, model)
	}

	storage := newMemoryMediaStorage()
	mediaRepo := repositories.NewMediaRepository(TestSuite.DbConn)
	momentRepo := repositories.NewMomentRepository(TestSuite.DbConn)
	notificationRepo := repositories.NewNotificationRepository(TestSuite.DbConn)

	jwt := NewJwtService(TestSuite.BaseService)
	identityService := NewIdentityService(
		TestSuite.BaseService, TestSuite.UserRepository, TestSuite.GroupRepository, TestSuite.GroupMembershipRepository,
		TestSuite.GoogleIdentityRepository, TestSuite.RefreshSessionRepository, TestSuite.EmailSignupCodeRepository,
		jwt, &fakeGoogleVerifier{}, newTestEmailService(),
	).(*IdentityService)

	profileService := NewProfileService(TestSuite.BaseService, TestSuite.UserRepository, TestSuite.GroupRepository)
	groupService := NewGroupService(TestSuite.BaseService, TestSuite.GroupRepository, TestSuite.UserRepository, TestSuite.GroupMembershipRepository)
	groupInviteService := NewGroupInviteService(TestSuite.BaseService, TestSuite.UserRepository, TestSuite.GroupRepository, TestSuite.GroupMembershipRepository, TestSuite.GroupInviteRepository)
	userService := NewUserService(TestSuite.BaseService, TestSuite.UserRepository, TestSuite.GroupRepository, TestSuite.GroupMembershipRepository)
	contentService := NewContentService(TestSuite.ActivityRepository, TestSuite.SpaceRepository)
	favoriteService := NewFavoriteService(TestSuite.BaseService, TestSuite.FavoriteRepository, TestSuite.ActivityRepository, TestSuite.UserRepository)
	gameService := NewGameService(TestSuite.BaseService, TestSuite.GameRepository, TestSuite.ActivityRepository, TestSuite.UserRepository, TestSuite.OperationAuditRepository)
	installationActivityService := NewActivityService(TestSuite.BaseService, TestSuite.ActivityRepository, TestSuite.OperationAuditRepository, TestSuite.UserRepository)
	spaceService := NewSpaceService(TestSuite.SpaceRepository)
	adminService := NewAdminInstallationService(TestSuite.BaseService, TestSuite.SpaceRepository, TestSuite.ActivityRepository, TestSuite.OperationAuditRepository, TestSuite.AdminOperationRepository, TestSuite.UserRepository)
	mediaService := NewMediaService(TestSuite.BaseService, mediaRepo, storage, TestSuite.UserRepository)
	momentService := NewMomentService(TestSuite.BaseService, momentRepo, mediaRepo, storage, TestSuite.UserRepository, TestSuite.OperationAuditRepository)
	notificationService := NewNotificationService(TestSuite.BaseService, notificationRepo, TestSuite.UserRepository)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	router := routers.NewRouter(engine, &handlers.Handlers{
		HealthcheckHandler:       &handlers.HealthcheckHandler{DB: TestSuite.DbConn, MediaStorage: storage},
		IdentityHandler:          &handlers.IdentityHandler{IdentityService: identityService},
		ProfileHandler:           &handlers.ProfileHandler{ProfileService: profileService},
		GroupHandler:             &handlers.GroupHandler{GroupService: groupService},
		GroupInviteHandler:       &handlers.GroupInviteHandler{GroupInviteService: groupInviteService},
		UserHandler:              &handlers.UserHandler{UserService: userService},
		InstallationHandler:      &handlers.InstallationHandler{SpaceService: spaceService, ActivityService: installationActivityService},
		AdminInstallationHandler: &handlers.AdminInstallationHandler{AdminInstallationService: adminService},
		ContentHandler:           &handlers.ContentHandler{ContentService: contentService},
		FavoriteHandler:          &handlers.FavoriteHandler{FavoriteService: favoriteService},
		GameHandler:              &handlers.GameHandler{GameService: gameService},
		MediaHandler:             &handlers.MediaHandler{MediaService: mediaService},
		MomentHandler:            &handlers.MomentHandler{MomentService: momentService},
		NotificationHandler:      &handlers.NotificationHandler{NotificationService: notificationService},
	})
	router.RegisterRoutes()

	return &e2eRig{Engine: engine, JWT: jwt, Identity: identityService, Storage: storage}
}
