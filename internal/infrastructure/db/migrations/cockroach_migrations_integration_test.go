package migrations_test

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/migrations"
	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestMigrations_CockroachCleanInstallAndReplay(t *testing.T) {
	ctx := context.Background()
	// An in-memory store (this is a throwaway container that only needs to exist for the
	// duration of the test, so durability is irrelevant) skips the disk-fsync-per-write cost
	// CockroachDB otherwise pays on every DDL statement's Raft log append, which dominates the
	// wall-clock time of replaying the full migration history twice.
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "cockroachdb/cockroach:v25.2.19",
			ExposedPorts: []string{"26257/tcp"},
			Cmd: []string{
				"start",
				"--insecure",
				"--store=type=mem,size=0.25",
				"--listen-addr=0.0.0.0:26257",
				"--advertise-addr=localhost:26257",
				"--join=localhost:26257",
				"--http-addr=0.0.0.0:8080",
			},
			WaitingFor: wait.ForListeningPort("26257/tcp").WithStartupTimeout(60 * time.Second),
		},
		Started: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, container.Terminate(context.Background())) })
	exitCode, output, err := container.Exec(ctx, []string{
		"cockroach", "init", "--insecure", "--host=localhost:26257",
	})
	require.NoError(t, err)
	outputBytes, err := io.ReadAll(output)
	require.NoError(t, err)
	require.Zero(t, exitCode, string(outputBytes))
	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, nat.Port("26257/tcp"))
	require.NoError(t, err)
	dsn := fmt.Sprintf("postgresql://root@%s:%s/defaultdb?sslmode=disable", host, port.Port())
	database, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		Logger:                                   logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	require.NoError(t, migrations.MigrateModelsWithDB(database))
	require.NoError(t, migrations.MigrateModelsWithDB(database))
	for _, table := range []string{
		"media_assets",
		"moments",
		"moment_likes",
		"idempotency_operations",
		"media_processing_claims",
		"media_cleanup_jobs",
		"notifications",
		"notification_preferences",
	} {
		assert.True(t, database.Migrator().HasTable(table), table)
	}
	assert.False(t, database.Migrator().HasTable("events"))
}
