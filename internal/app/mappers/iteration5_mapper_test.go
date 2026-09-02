package mappers

import (
	"testing"
	"time"

	activityEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/activity/entities"
	spaceEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/space/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIteration5PublicContentMapper_ExactProjectionAndNilSafety(t *testing.T) {
	// given
	startsAt := time.Date(2026, 10, 18, 12, 0, 0, 0, time.FixedZone("offset", -3*60*60))
	endsAt := startsAt.Add(time.Hour)
	state := "live"
	item := &activityEntities.PublicActivity{Activity: activityEntities.Activity{ID: "11111111-1111-4111-8111-111111111111", Name: "Activity", Kind: activityEntities.KindLive, StartsAt: &startsAt, EndsAt: &endsAt}, Space: &spaceEntities.Space{ID: "22222222-2222-4222-8222-222222222222", Name: "Palco", Slug: "palco"}}

	// when
	activity := MapPublicActivityToResponseDTO(item, &state)
	schedule := MapScheduleItemToResponseDTO(item, state)

	// then
	assert.Nil(t, MapPublicSpaceToResponseDTO(nil))
	assert.Nil(t, MapPublicActivityToResponseDTO(nil, nil))
	require.NotNil(t, activity)
	assert.Equal(t, time.UTC, activity.StartsAt.Location())
	assert.Equal(t, "palco", activity.Space.Slug)
	assert.Equal(t, time.UTC, schedule.StartsAt.Location())
	assert.Equal(t, "live", schedule.State)
}

func TestIteration5PublicContentMapper_NilTimeAndSpaceFields(t *testing.T) {
	item := &activityEntities.PublicActivity{
		Activity: activityEntities.Activity{ID: "activity-without-window", Name: "Activity"},
	}

	result := MapPublicActivityToResponseDTO(item, nil)

	require.NotNil(t, result)
	assert.Nil(t, result.StartsAt)
	assert.Nil(t, result.EndsAt)
	assert.Nil(t, result.Space)
}
