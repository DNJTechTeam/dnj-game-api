package services

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"

	commonInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/common/interfaces"
	groupInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/group/interfaces"
	identityInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/identity/interfaces"
	refreshInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/refreshsession/interfaces"
	swInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/subscriptionwebhook/interfaces"
	svcInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/subscriptionwebhookverificationcode/interfaces"
	taskInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/task/interfaces"
	uInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/user/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/common"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/repositories"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/di/test"
	"github.com/stretchr/testify/require"
)

// TestSuiteType bundles a real (testcontainers) Postgres connection with the
// repositories and BaseService used across service tests.
type TestSuiteType struct {
	*test.Containers
	UserRepository                uInterfaces.UserRepositoryInterface
	TaskRepository                taskInterfaces.TaskRepositoryInterface
	GroupRepository               groupInterfaces.GroupRepositoryInterface
	SubscriptionWebhookRepository swInterfaces.SubscriptionWebhookRepositoryInterface
	VerificationCodeRepository    svcInterfaces.SubscriptionWebhookVerificationCodeRepositoryInterface
	GoogleIdentityRepository      identityInterfaces.GoogleIdentityRepositoryInterface
	RefreshSessionRepository      refreshInterfaces.RefreshSessionRepositoryInterface
	BaseService                   *BaseService
}

var TestSuite *TestSuiteType

func initializeTestSuite() {
	TestSuite = &TestSuiteType{
		Containers: test.ProvideContainers(test.DbContainerName),
	}
	TestSuite.BaseService = NewBaseService(db.NewTransactionManager(TestSuite.DbConn))
	TestSuite.UserRepository = repositories.NewUserRepository(TestSuite.DbConn)
	TestSuite.TaskRepository = repositories.NewTaskRepository(TestSuite.DbConn)
	TestSuite.GroupRepository = repositories.NewGroupRepository(TestSuite.DbConn)
	TestSuite.SubscriptionWebhookRepository = repositories.NewSubscriptionWebhookRepository(TestSuite.DbConn)
	TestSuite.VerificationCodeRepository = repositories.NewSubscriptionWebhookVerificationCodeRepository(TestSuite.DbConn)
	TestSuite.GoogleIdentityRepository = repositories.NewGoogleIdentityRepository(TestSuite.DbConn)
	TestSuite.RefreshSessionRepository = repositories.NewRefreshSessionRepository(TestSuite.DbConn)
}

func TestMain(m *testing.M) {
	initializeTestSuite()

	code := m.Run()

	test.HandleShutdown(TestSuite.Containers)

	os.Exit(code)
}

// DefaultSetup sets the environment variables every test relies on. Call it at
// the top of each test function.
func (ts *TestSuiteType) DefaultSetup(t *testing.T) {
	t.Setenv("APP_NAME", "Nucleus API")
	t.Setenv("API_PREFIX", "/v1")
	t.Setenv("SERVER_RUNNER", "default")
	t.Setenv("SERVER_PORT", "8080")
	t.Setenv("SERVER_ENVIRONMENT", string(common.EnvironmentTest))
	t.Setenv("GIN_MODE", "debug")
	t.Setenv("JWT_IDENTITY_SECRET", "testIdentitySecret")
	t.Setenv("GOOGLE_CLIENT_ID", "test-google-client")
	t.Setenv("DOCUMENT_HMAC_SECRET", "test-document-hmac-secret")
	t.Setenv("SUBSCRIPTION_WEBHOOK_SECRET", "testWebhookSecret")
	t.Setenv("FRONTEND_URL", "https://app.example.com")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.example.com")
	t.Setenv("EMAIL_SENDER_EMAIL", "no-reply@example.com")
	t.Setenv("EMAIL_API_BASE_URL", "https://api.email.com")
	t.Setenv("EMAIL_API_KEY", "testApiKey")
}

// TruncateTable empties a table between test cases.
func (ts *TestSuiteType) TruncateTable(t *testing.T, model commonInterfaces.ModelInterface) {
	query := fmt.Sprintf("TRUNCATE TABLE %s CASCADE;", model.TableName())
	err := ts.DbConn.Exec(query).Error
	require.NoError(t, err)
}

// ContextWithUser returns a context carrying an authenticated user id, the same
// way AuthenticationMiddleware would in production.
func (ts *TestSuiteType) ContextWithUser(userID uint64) context.Context {
	return context.WithValue(ts.Ctx, common.UserIDContextKey, strconv.FormatUint(userID, 10))
}
