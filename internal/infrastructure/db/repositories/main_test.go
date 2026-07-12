package repositories

import (
	"fmt"
	"os"
	"testing"

	commonInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/common/interfaces"
	groupInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/group/interfaces"
	swInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/subscriptionwebhook/interfaces"
	svcInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/subscriptionwebhookverificationcode/interfaces"
	uInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/user/interfaces"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/di/test"
	"github.com/stretchr/testify/require"
)

type TestSuiteType struct {
	*test.Containers
	UserRepository                uInterfaces.UserRepositoryInterface
	GroupRepository               groupInterfaces.GroupRepositoryInterface
	SubscriptionWebhookRepository swInterfaces.SubscriptionWebhookRepositoryInterface
	VerificationCodeRepository    svcInterfaces.SubscriptionWebhookVerificationCodeRepositoryInterface
}

var TestSuite *TestSuiteType

func initializeTestSuite() {
	TestSuite = &TestSuiteType{
		Containers: test.ProvideContainers(test.DbContainerName),
	}
	TestSuite.UserRepository = NewUserRepository(TestSuite.DbConn)
	TestSuite.GroupRepository = NewGroupRepository(TestSuite.DbConn)
	TestSuite.SubscriptionWebhookRepository = NewSubscriptionWebhookRepository(TestSuite.DbConn)
	TestSuite.VerificationCodeRepository = NewSubscriptionWebhookVerificationCodeRepository(TestSuite.DbConn)
}

func TestMain(m *testing.M) {
	initializeTestSuite()

	code := m.Run()

	test.HandleShutdown(TestSuite.Containers)

	os.Exit(code)
}

func (ts *TestSuiteType) TruncateTable(t *testing.T, model commonInterfaces.ModelInterface) {
	query := fmt.Sprintf("TRUNCATE TABLE %s CASCADE;", model.TableName())
	err := ts.DbConn.Exec(query).Error
	require.NoError(t, err)
}
