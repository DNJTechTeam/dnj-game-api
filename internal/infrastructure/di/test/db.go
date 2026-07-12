package test

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/migrations"

	"github.com/docker/go-connections/nat"
	"github.com/testcontainers/testcontainers-go"
	postgresContainer "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func provideDb(ctx context.Context, dockerNetwork *testcontainers.DockerNetwork) (*gorm.DB, testcontainers.Container) {
	dbContainer, err := postgresContainer.Run(
		ctx,
		"postgres:latest",
		testcontainers.WithEnv(
			map[string]string{
				"POSTGRES_USER":     "root",
				"POSTGRES_PASSWORD": "rootpassword",
				"POSTGRES_DB":       "dnjgame_db",
			},
		),
		network.WithNetwork([]string{DbContainerName}, dockerNetwork),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)

	if err != nil {
		log.Fatalf("failed to create db container: %s", err)
	}

	host, err := dbContainer.Host(ctx)
	if err != nil {
		log.Fatalf("failed to get db container host: %s", err)
	}

	port, err := dbContainer.MappedPort(ctx, nat.Port("5432"))
	if err != nil {
		log.Fatalf("failed to get db container port: %s", err)
	}

	dbConnection, err := gorm.Open(
		postgres.Open(fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", "root", "rootpassword", host, port.Port(), "dnjgame_db")),
		&gorm.Config{
			DisableForeignKeyConstraintWhenMigrating: true,
			Logger:                                   logger.Default.LogMode(logger.Silent),
		},
	)

	if err != nil {
		log.Fatalf("failed to connect to db: %s", err)
	}

	db.SetConnection(dbConnection)
	if err := migrations.MigrateModels(); err != nil {
		log.Fatalf("failed to run database migrations: %s", err)
	}

	return dbConnection, dbContainer
}
