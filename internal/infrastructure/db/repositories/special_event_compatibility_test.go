package repositories

import (
	"context"
	"fmt"
	"testing"
	"time"

	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	specialEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/specialevent/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
	"github.com/docker/go-connections/nat"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestSpecialEventRepository_FindVisibleDatabaseCompatibility(t *testing.T) {
	ctx := context.Background()
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "cockroachdb/cockroach:v25.2.19",
			ExposedPorts: []string{"26257/tcp"},
			Cmd:          []string{"start-single-node", "--insecure", "--store=type=mem,size=0.25"},
			WaitingFor:   wait.ForLog("node startup completed").WithStartupTimeout(60 * time.Second),
		},
		Started: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, container.Terminate(context.Background())) })
	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, nat.Port("26257/tcp"))
	require.NoError(t, err)
	cockroach, err := gorm.Open(postgres.Open(fmt.Sprintf("postgresql://root@%s:%s/defaultdb?sslmode=disable", host, port.Port())), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := cockroach.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, cockroach.AutoMigrate(&models.SpecialEvent{}))

	TestSuite.TruncateTable(t, &models.SpecialEvent{})
	now := time.Now().UTC().Truncate(time.Microsecond)
	fixture := func(status specialEntities.Status, targets []string, endsAt, updatedAt time.Time) specialEntities.Event {
		return specialEntities.Event{
			ID: uuid.NewString(), ActivityID: uuid.NewString(), Title: "Compatibility test",
			DurationMinutes: 30, CreatedBy: 1, CreatedAt: now, UpdatedAt: updatedAt,
			Status: status, Targets: targets, EndsAt: endsAt,
		}
	}
	active := fixture(specialEntities.StatusActive, []string{"app", "tv", "screen"}, now.Add(time.Hour), now)
	teaser := fixture(specialEntities.StatusTeaser, []string{"app"}, now.Add(time.Hour), now.Add(-time.Minute))
	escapedTarget := `app"\' ? OR TRUE --`
	escapedEvent := fixture(specialEntities.StatusActive, []string{escapedTarget}, active.EndsAt, now)
	tests := []struct {
		name   string
		target string
		events []specialEntities.Event
		wantID string
	}{
		{name: "empty", target: "app"},
		{name: "teaser", target: "app", events: []specialEntities.Event{teaser}, wantID: teaser.ID},
		{name: "app", target: "app", events: []specialEntities.Event{active}, wantID: active.ID},
		{name: "tv", target: "tv", events: []specialEntities.Event{active}, wantID: active.ID},
		{name: "screen", target: "screen", events: []specialEntities.Event{active}, wantID: active.ID},
		{name: "other target", target: "screen", events: []specialEntities.Event{teaser}},
		{name: "no substring match", target: "ap", events: []specialEntities.Event{active}},
		{name: "empty targets", target: "app", events: []specialEntities.Event{fixture(specialEntities.StatusActive, []string{}, now.Add(time.Hour), now)}},
		{name: "expired", target: "app", events: []specialEntities.Event{fixture(specialEntities.StatusActive, active.Targets, now.Add(-time.Second), now)}},
		{name: "expires now", target: "app", events: []specialEntities.Event{fixture(specialEntities.StatusActive, active.Targets, now, now)}},
		{name: "draft", target: "app", events: []specialEntities.Event{fixture(specialEntities.StatusDraft, active.Targets, active.EndsAt, now)}},
		{name: "closed", target: "app", events: []specialEntities.Event{fixture(specialEntities.StatusClosed, active.Targets, active.EndsAt, now)}},
		{name: "newest visible", target: "app", events: []specialEntities.Event{active, teaser}, wantID: active.ID},
		{name: "parameter cannot change SQL", target: escapedTarget, events: []specialEntities.Event{active}},
		{name: "escaped JSON target", target: escapedTarget, events: []specialEntities.Event{escapedEvent}, wantID: escapedEvent.ID},
	}

	for _, database := range []struct {
		name string
		db   *gorm.DB
	}{{"PostgreSQL", TestSuite.DbConn}, {"CockroachDB", cockroach}} {
		t.Run(database.name, func(t *testing.T) {
			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					// given
					tx := database.db.Begin()
					require.NoError(t, tx.Error)
					t.Cleanup(func() { require.NoError(t, tx.Rollback().Error) })
					repo := NewSpecialEventRepository(tx)
					for _, event := range tc.events {
						require.NoError(t, repo.Create(ctx, &event))
					}

					// when
					found, err := repo.FindVisible(ctx, tc.target, now)

					// then
					if tc.wantID == "" {
						require.ErrorIs(t, err, appErrors.ErrNotFound)
						require.Nil(t, found)
						return
					}
					require.NoError(t, err)
					require.Equal(t, tc.wantID, found.ID)
				})
			}
		})
	}
}
