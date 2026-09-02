package repositories

import (
	"context"
	"errors"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	appErrors "github.com/dnjtechteam/dnj-game-api/internal/app/errors"
	activityEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/activity/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func iteration5StringPointer(value string) *string { return &value }

func TestIteration5_PublicActivityRowMappingOptionalSpaceAndTimestamps(t *testing.T) {
	createdAt := time.Date(2026, 10, 18, 12, 0, 0, 0, time.FixedZone("offset", -3*60*60))
	updatedAt := createdAt.Add(time.Hour)

	withoutSpace := mapPublicActivityRow(&publicActivityRow{
		ID:     "activity-without-space",
		Kind:   string(activityEntities.KindLive),
		Status: string(activityEntities.StatusActive),
	})
	withSpace := mapPublicActivityRow(&publicActivityRow{
		ID:             "activity-with-space",
		SpaceID:        iteration5StringPointer("space-id"),
		JoinedSpaceID:  iteration5StringPointer("space-id"),
		SpaceSlug:      iteration5StringPointer("palco"),
		SpaceName:      iteration5StringPointer("Palco"),
		MapReference:   iteration5StringPointer("map-ref"),
		SpaceCreatedAt: &createdAt,
		SpaceUpdatedAt: &updatedAt,
	})

	require.Nil(t, withoutSpace.Space)
	require.NotNil(t, withSpace.Space)
	assert.Equal(t, "palco", withSpace.Space.Slug)
	assert.Equal(t, createdAt, withSpace.Space.CreatedAt)
	assert.Equal(t, updatedAt, withSpace.Space.UpdatedAt)
}

func iteration5PublicActivityRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "space_id", "slug", "name", "description", "kind", "status",
		"starts_at", "ends_at", "actual_started_at", "flex_minutes", "check_in_points",
		"moment_points", "cooldown_seconds", "allows_moment", "created_at", "updated_at",
		"joined_space_id", "space_slug", "space_name", "space_map_reference", "space_created_at", "space_updated_at",
	}).AddRow(
		"activity-id", nil, "activity-slug", "Activity", nil, "schedule", "draft",
		nil, nil, nil, 0, 0, 0, 0, false, time.Now(), time.Now(), nil, nil, nil, nil, nil, nil,
	)
}

func TestIteration5_ActivityRepositoryManagerScheduleQueriesAndErrors(t *testing.T) {
	t.Run("global and assigned schedules", func(t *testing.T) {
		db, mock := newMockDB(t)
		repo := &ActivityRepository{BaseRepository: NewBaseRepository[models.Activity](db)}
		mock.ExpectQuery(`SELECT`).WillReturnRows(iteration5PublicActivityRows())
		global, err := repo.ListManagerSchedule(context.Background(), 42, true)
		require.NoError(t, err)
		require.Len(t, global, 1)

		mock.ExpectQuery(`SELECT`).WillReturnRows(iteration5PublicActivityRows())
		assigned, err := repo.ListManagerSchedule(context.Background(), 42, false)
		require.NoError(t, err)
		require.Len(t, assigned, 1)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("database failure is redacted", func(t *testing.T) {
		db, mock := newMockDB(t)
		repo := &ActivityRepository{BaseRepository: NewBaseRepository[models.Activity](db)}
		mock.ExpectQuery(`SELECT`).WillReturnError(errors.New("connection reset"))
		_, err := repo.ListManagerSchedule(context.Background(), 42, true)
		assert.ErrorIs(t, err, appErrors.InternalError)
	})
}
