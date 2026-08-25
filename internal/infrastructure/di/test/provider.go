package test

import (
	"context"
	"log"
	"slices"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"gorm.io/gorm"
)

type connectionConfig struct {
	User     string
	Password string
	Database string
}

const (
	DbContainerName = "dnjgame_db"
)

var AllContainers = []string{DbContainerName}

type Containers struct {
	Ctx    context.Context
	DbConn *gorm.DB

	dbContainer testcontainers.Container
	network     *testcontainers.DockerNetwork
}

type contextKey struct{}

func (c *contextKey) String() string {
	return "Host"
}

func ProvideContainers(containersToStart ...string) *Containers {
	return provideContainers(true, containersToStart...)
}

// ProvideUnmigratedContainers creates the same isolated infrastructure but
// leaves PostgreSQL empty. Migration integration tests use it to prove clean
// installs, legacy upgrades and concurrent runners independently of TestMain.
func ProvideUnmigratedContainers(containersToStart ...string) *Containers {
	return provideContainers(false, containersToStart...)
}

func provideContainers(runMigrations bool, containersToStart ...string) *Containers {
	testContainers := &Containers{}
	testContainers.Ctx = context.WithValue(context.Background(), &contextKey{}, "test-host")
	testContainers.network = provideNetwork(testContainers.Ctx)

	for _, container := range containersToStart {
		if !slices.Contains(AllContainers, container) {
			log.Fatalf("invalid container: %s", container)
		}
	}

	if len(containersToStart) == 0 {
		containersToStart = AllContainers
	}

	if slices.Contains(containersToStart, DbContainerName) {
		dbConnection, dbContainer := provideDb(testContainers.Ctx, testContainers.network, runMigrations)
		testContainers.DbConn = dbConnection
		testContainers.dbContainer = dbContainer
	}

	return testContainers
}

func provideNetwork(ctx context.Context) *testcontainers.DockerNetwork {
	dockerNetwork, err := network.New(ctx,
		network.WithAttachable(),
	)
	if err != nil {
		log.Fatalf("failed to create network: %s", err)
	}

	return dockerNetwork
}

func HandleShutdown(containers *Containers) {
	if containers.dbContainer != nil {
		if err := containers.dbContainer.Terminate(containers.Ctx); err != nil {
			log.Fatalf("failed to stop db container: %s", err)
		}
	}
	if containers.network != nil {
		if err := containers.network.Remove(containers.Ctx); err != nil {
			log.Fatalf("failed to remove network: %s", err)
		}
	}
}
