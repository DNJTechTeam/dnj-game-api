package services

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"

	commonInterfaces "github.com/scienceandcode/nucleus-api/internal/domain/common/interfaces"
	taskInterfaces "github.com/scienceandcode/nucleus-api/internal/domain/task/interfaces"
	uInterfaces "github.com/scienceandcode/nucleus-api/internal/domain/user/interfaces"
	"github.com/scienceandcode/nucleus-api/internal/infrastructure/common"
	"github.com/scienceandcode/nucleus-api/internal/infrastructure/db"
	"github.com/scienceandcode/nucleus-api/internal/infrastructure/db/repositories"
	"github.com/scienceandcode/nucleus-api/internal/infrastructure/di/test"
	"github.com/stretchr/testify/require"
)

// TestSuiteType bundles a real (testcontainers) Postgres connection with the
// repositories and BaseService used across service tests.
type TestSuiteType struct {
	*test.Containers
	UserRepository uInterfaces.UserRepositoryInterface
	TaskRepository taskInterfaces.TaskRepositoryInterface
	BaseService    *BaseService
}

var TestSuite *TestSuiteType

func initializeTestSuite() {
	TestSuite = &TestSuiteType{
		Containers: test.ProvideContainers(test.DbContainerName),
	}
	TestSuite.BaseService = NewBaseService(db.NewTransactionManager(TestSuite.DbConn))
	TestSuite.UserRepository = repositories.NewUserRepository(TestSuite.DbConn)
	TestSuite.TaskRepository = repositories.NewTaskRepository(TestSuite.DbConn)
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
	t.Setenv("EMAIL_CONFIRMATION_SECRET", "testSecret")
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
