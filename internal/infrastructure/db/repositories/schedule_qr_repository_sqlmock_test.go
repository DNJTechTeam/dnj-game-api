package repositories

import (
	"context"
	"errors"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	activityEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/activity/entities"
	gameEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/game/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func scheduleQRCheckInRows(now time.Time) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "user_id", "activity_id", "space_id", "point_entry_id", "checked_in_at", "blocked_until", "created_at"}).AddRow("check-in", 42, "activity", "space", "entry", now, now.Add(10*time.Minute), now)
}

func TestIteration4AdminRepositories_ScheduleQRQueriesCoverage(t *testing.T) {
	testScheduleQRRepositoryQueries(t)
}

func TestIteration6ScheduleQRRepositoryQueries(t *testing.T) {
	testScheduleQRRepositoryQueries(t)
}

func testScheduleQRRepositoryQueries(t *testing.T) {
	now := time.Date(2026, 10, 18, 15, 0, 0, 0, time.UTC)

	t.Run("schedule check-in lookups map persisted rows", func(t *testing.T) {
		for _, find := range []func(*GameRepository) (*gameEntities.ScheduleQRCheckIn, error){
			func(repo *GameRepository) (*gameEntities.ScheduleQRCheckIn, error) {
				return repo.FindScheduleQRCheckIn(context.Background(), 42, "activity")
			},
			func(repo *GameRepository) (*gameEntities.ScheduleQRCheckIn, error) {
				return repo.FindLatestScheduleQRCheckIn(context.Background(), 42)
			},
			func(repo *GameRepository) (*gameEntities.ScheduleQRCheckIn, error) {
				return repo.FindScheduleQRCheckInByID(context.Background(), "check-in")
			},
		} {
			db, mock := newMockDB(t)
			mock.ExpectQuery("SELECT").WillReturnRows(scheduleQRCheckInRows(now))

			checkIn, err := find(&GameRepository{BaseRepository: NewBaseRepository[models.ActivityRun](db)})

			require.NoError(t, err)
			assert.Equal(t, "check-in", checkIn.ID)
			assert.Equal(t, now.Add(10*time.Minute), checkIn.BlockedUntil)
			require.NoError(t, mock.ExpectationsWereMet())
		}
	})

	t.Run("lookup failures are redacted", func(t *testing.T) {
		for _, find := range []func(*GameRepository) error{
			func(repo *GameRepository) error {
				_, err := repo.FindScheduleQRCheckIn(context.Background(), 42, "activity")
				return err
			},
			func(repo *GameRepository) error {
				_, err := repo.FindLatestScheduleQRCheckIn(context.Background(), 42)
				return err
			},
			func(repo *GameRepository) error {
				_, err := repo.FindScheduleQRCheckInByID(context.Background(), "check-in")
				return err
			},
			func(repo *GameRepository) error { _, err := repo.FindQRScanBlock(context.Background(), 42); return err },
			func(repo *GameRepository) error {
				_, err := repo.IsActiveSpecialEventRun(context.Background(), "run", now)
				return err
			},
		} {
			db, mock := newMockDB(t)
			mock.ExpectQuery("SELECT").WillReturnError(errors.New("database unavailable"))

			assert.Error(t, find(&GameRepository{BaseRepository: NewBaseRepository[models.ActivityRun](db)}))
			require.NoError(t, mock.ExpectationsWereMet())
		}
	})

	t.Run("scan block and active special event queries", func(t *testing.T) {
		db, mock := newMockDB(t)
		mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"user_id", "blocked_until", "updated_at"}).AddRow(42, now.Add(10*time.Minute), now))
		mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
		repo := &GameRepository{BaseRepository: NewBaseRepository[models.ActivityRun](db)}

		blockedUntil, blockErr := repo.FindQRScanBlock(context.Background(), 42)
		active, eventErr := repo.IsActiveSpecialEventRun(context.Background(), "run", now)

		require.NoError(t, blockErr)
		require.NoError(t, eventErr)
		assert.Equal(t, now.Add(10*time.Minute), *blockedUntil)
		assert.True(t, active)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("writes return safe database errors", func(t *testing.T) {
		db, mock := newMockDB(t)
		mock.ExpectBegin().WillReturnError(errors.New("database unavailable"))
		mock.ExpectBegin().WillReturnError(errors.New("database unavailable"))
		repo := &GameRepository{BaseRepository: NewBaseRepository[models.ActivityRun](db)}
		checkIn := &gameEntities.ScheduleQRCheckIn{ID: "check-in", UserID: 42, ActivityID: "activity", SpaceID: "space", PointEntryID: "entry", CheckedInAt: now, BlockedUntil: now.Add(10 * time.Minute)}
		entry := &gameEntities.PointEntry{ID: "entry", UserID: 42, ActivityID: "activity", Origin: "schedule_qr", Reason: "schedule_checkin", Delta: 10, CreatedAt: now}

		createErr := repo.CreateScheduleQRCheckInAndAward(context.Background(), checkIn, entry)
		saveErr := repo.SaveQRScanBlock(context.Background(), 42, now.Add(10*time.Minute))

		require.Error(t, createErr)
		require.Error(t, saveErr)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("schedule overlap and current schedule are resolved", func(t *testing.T) {
		db, mock := newMockDB(t)
		mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
		mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"id", "space_id", "slug", "name", "kind", "status", "starts_at", "ends_at", "created_at", "updated_at"}).AddRow("activity", "space", "activity", "Activity", string(activityEntities.KindSchedule), string(activityEntities.StatusActive), now.Add(-time.Hour), now.Add(time.Hour), now, now))
		repo := &ActivityRepository{BaseRepository: NewBaseRepository[models.Activity](db)}

		excluded := "other-activity"
		overlaps, overlapErr := repo.HasScheduleOverlap(context.Background(), "space", now, now.Add(time.Hour), &excluded)
		activity, scheduleErr := repo.FindScheduleForSpaceAt(context.Background(), "space", now)

		require.NoError(t, overlapErr)
		require.NoError(t, scheduleErr)
		assert.True(t, overlaps)
		assert.Equal(t, "activity", activity.ID)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("schedule queries redact database failures", func(t *testing.T) {
		for _, call := range []func(*ActivityRepository) error{
			func(repo *ActivityRepository) error {
				_, err := repo.HasScheduleOverlap(context.Background(), "space", now, now.Add(time.Hour), nil)
				return err
			},
			func(repo *ActivityRepository) error {
				_, err := repo.FindScheduleForSpaceAt(context.Background(), "space", now)
				return err
			},
		} {
			db, mock := newMockDB(t)
			mock.ExpectQuery("SELECT").WillReturnError(errors.New("database unavailable"))

			assert.Error(t, call(&ActivityRepository{BaseRepository: NewBaseRepository[models.Activity](db)}))
			require.NoError(t, mock.ExpectationsWereMet())
		}
	})
}
