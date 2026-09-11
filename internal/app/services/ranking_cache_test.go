package services

import (
	"context"
	"errors"
	"testing"
	"time"

	gameEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/game/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func groupName(s string) *string { return &s }

func seedRankingRepo(t *testing.T) *mocks.MockGameRepositoryInterface {
	t.Helper()
	repo := mocks.NewMockGameRepositoryInterface(t)
	individual := []gameEntities.IndividualRanking{
		{UserID: 1, Name: "Ana", GroupName: groupName("G1"), Points: 30, Position: 1},
		{UserID: 2, Name: "Bia", GroupName: groupName("G2"), Points: 20, Position: 2},
		{UserID: 3, Name: "Cadu", GroupName: nil, Points: 10, Position: 3},
	}
	group := []gameEntities.GroupRanking{
		{GroupID: 10, Name: "G1", Members: 1, Points: 30, Position: 1},
		{GroupID: 20, Name: "G2", Members: 1, Points: 20, Position: 2},
	}
	repo.EXPECT().TopIndividualRankings(context.Background(), rankingSnapshotLimit).Return(individual, nil)
	repo.EXPECT().TopGroupRankings(context.Background(), rankingSnapshotLimit).Return(group, nil)
	return repo
}

// Given a warm snapshot, When the same reads happen many times within the TTL,
// Then the repository is queried only once (temporal reuse).
func TestRankingCache_ServesFromSnapshotWithinTTL(t *testing.T) {
	// Given a repo that answers the full-ranking load exactly once.
	repo := seedRankingRepo(t)
	cache := newRankingCache(repo, 5*time.Second, func() time.Time { return time.Unix(1000, 0) })

	// When top-N, pagination-equivalent reads and a position lookup all run repeatedly.
	for i := 0; i < 10; i++ {
		top, err := cache.topIndividual(context.Background(), 2)
		require.NoError(t, err)
		require.Len(t, top, 2)
		assert.Equal(t, uint64(1), top[0].UserID)

		groups, err := cache.topGroup(context.Background(), 1)
		require.NoError(t, err)
		require.Len(t, groups, 1)

		ind, grp, err := cache.currentRanking(context.Background(), 2)
		require.NoError(t, err)
		require.NotNil(t, ind)
		assert.Equal(t, uint64(2), ind.Position)
		require.NotNil(t, grp)
		assert.Equal(t, uint64(2), grp.Position) // Bia's group G2 is 2nd
	}
	// Then the repo mock (EXPECT once) is satisfied by t.Cleanup.
}

// Given a user without a group, Then currentRanking returns the individual row and a nil group.
func TestRankingCache_CurrentRankingWithoutGroup(t *testing.T) {
	repo := seedRankingRepo(t)
	cache := newRankingCache(repo, 5*time.Second, func() time.Time { return time.Unix(1000, 0) })

	ind, grp, err := cache.currentRanking(context.Background(), 3)
	require.NoError(t, err)
	require.NotNil(t, ind)
	assert.Nil(t, grp)
}

// Given a user absent from the snapshot, Then currentRanking reports it (nil, nil) so
// the caller can fall back to a live lookup.
func TestRankingCache_CurrentRankingAbsentUser(t *testing.T) {
	repo := seedRankingRepo(t)
	cache := newRankingCache(repo, 5*time.Second, func() time.Time { return time.Unix(1000, 0) })

	ind, grp, err := cache.currentRanking(context.Background(), 999)
	require.NoError(t, err)
	assert.Nil(t, ind)
	assert.Nil(t, grp)
}

// Given the TTL expired, When a read happens, Then the snapshot is reloaded.
func TestRankingCache_RefreshesAfterTTL(t *testing.T) {
	repo := mocks.NewMockGameRepositoryInterface(t)
	first := []gameEntities.IndividualRanking{{UserID: 1, Name: "Ana", Points: 30, Position: 1}}
	second := []gameEntities.IndividualRanking{{UserID: 1, Name: "Ana", Points: 99, Position: 1}}
	repo.EXPECT().TopIndividualRankings(context.Background(), rankingSnapshotLimit).Return(first, nil).Once()
	repo.EXPECT().TopGroupRankings(context.Background(), rankingSnapshotLimit).Return(nil, nil).Once()
	repo.EXPECT().TopIndividualRankings(context.Background(), rankingSnapshotLimit).Return(second, nil).Once()
	repo.EXPECT().TopGroupRankings(context.Background(), rankingSnapshotLimit).Return(nil, nil).Once()

	now := time.Unix(1000, 0)
	cache := newRankingCache(repo, 5*time.Second, func() time.Time { return now })

	top, err := cache.topIndividual(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, 30, top[0].Points)

	now = now.Add(6 * time.Second) // past TTL
	top, err = cache.topIndividual(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, 99, top[0].Points)
}

// Given a refresh error but a prior good snapshot, Then the stale snapshot is served.
func TestRankingCache_StaleOnError(t *testing.T) {
	repo := mocks.NewMockGameRepositoryInterface(t)
	good := []gameEntities.IndividualRanking{{UserID: 1, Name: "Ana", Points: 30, Position: 1}}
	repo.EXPECT().TopIndividualRankings(context.Background(), rankingSnapshotLimit).Return(good, nil).Once()
	repo.EXPECT().TopGroupRankings(context.Background(), rankingSnapshotLimit).Return(nil, nil).Once()
	// Second refresh fails.
	repo.EXPECT().TopIndividualRankings(context.Background(), rankingSnapshotLimit).Return(nil, errors.New("db down")).Once()

	now := time.Unix(1000, 0)
	cache := newRankingCache(repo, 5*time.Second, func() time.Time { return now })

	_, err := cache.topIndividual(context.Background(), 1)
	require.NoError(t, err)

	now = now.Add(6 * time.Second)
	top, err := cache.topIndividual(context.Background(), 1)
	require.NoError(t, err) // stale-on-error: no error surfaced
	require.Len(t, top, 1)
	assert.Equal(t, 30, top[0].Points) // last good value
}

// Given the cache is disabled (ttl<=0), Then enabled() is false and prime is a no-op.
func TestRankingCache_DisabledWhenTTLZero(t *testing.T) {
	repo := mocks.NewMockGameRepositoryInterface(t)
	cache := newRankingCache(repo, 0, nil)
	assert.False(t, cache.enabled())
	cache.prime(context.Background()) // must not touch the repo (no EXPECT set)
}

// Given no env, Then rankingCacheTTL is 0; given a positive value, it parses.
func TestRankingCacheTTL_FromEnv(t *testing.T) {
	t.Setenv("DNJ_RANKINGS_CACHE_TTL_SECONDS", "")
	assert.Equal(t, time.Duration(0), rankingCacheTTL())
	t.Setenv("DNJ_RANKINGS_CACHE_TTL_SECONDS", "7")
	assert.Equal(t, 7*time.Second, rankingCacheTTL())
	t.Setenv("DNJ_RANKINGS_CACHE_TTL_SECONDS", "-3")
	assert.Equal(t, time.Duration(0), rankingCacheTTL())
	t.Setenv("DNJ_RANKINGS_CACHE_TTL_SECONDS", "abc")
	assert.Equal(t, time.Duration(0), rankingCacheTTL())
}

// Given the cache is enabled, When topRankings/currentUserRanking run for a ranked user,
// Then they serve top-N and the user's position from the snapshot (no live CTE).
func TestOverviewRankings_UsesCache(t *testing.T) {
	repo := seedRankingRepo(t) // full-ranking load expected once
	svc := &GameService{games: repo, rankings: newRankingCache(repo, 5*time.Second, func() time.Time { return time.Unix(1000, 0) })}

	ind, grp, err := svc.rankings.OverviewTop(context.Background())
	require.NoError(t, err)
	require.Len(t, ind, 3) // top 30 over 3 rows -> all 3
	require.Len(t, grp, 2)

	cur, curGrp, err := svc.rankings.OverviewCurrent(context.Background(), 2)
	require.NoError(t, err)
	require.NotNil(t, cur)
	assert.Equal(t, uint64(2), cur.Position)
	require.NotNil(t, curGrp)
	assert.Equal(t, uint64(2), curGrp.Position)
}

// Given a user absent from the snapshot, When currentUserRanking runs,
// Then it falls back to a live FindCurrentRanking so the user is never missing.
func TestOverviewRankings_FallsBackForAbsentUser(t *testing.T) {
	repo := seedRankingRepo(t)
	liveInd := &gameEntities.IndividualRanking{UserID: 999, Name: "Novo", Points: 5, Position: 4}
	repo.EXPECT().FindCurrentRanking(context.Background(), uint64(999)).Return(liveInd, nil, nil).Once()
	svc := &GameService{games: repo, rankings: newRankingCache(repo, 5*time.Second, func() time.Time { return time.Unix(1000, 0) })}

	cur, _, err := svc.rankings.OverviewCurrent(context.Background(), 999)
	require.NoError(t, err)
	require.NotNil(t, cur)
	assert.Equal(t, uint64(4), cur.Position)
}

// Given the cache is disabled, When the helpers run, Then they call the repo directly.
func TestOverviewRankings_DisabledUsesRepo(t *testing.T) {
	repo := mocks.NewMockGameRepositoryInterface(t)
	repo.EXPECT().TopIndividualRankings(context.Background(), 30).Return([]gameEntities.IndividualRanking{{UserID: 1, Position: 1}}, nil).Once()
	repo.EXPECT().TopGroupRankings(context.Background(), 10).Return(nil, nil).Once()
	repo.EXPECT().FindCurrentRanking(context.Background(), uint64(1)).Return(&gameEntities.IndividualRanking{UserID: 1, Position: 1}, nil, nil).Once()
	svc := &GameService{games: repo, rankings: newRankingCache(repo, 0, nil)}

	_, _, err := svc.rankings.OverviewTop(context.Background())
	require.NoError(t, err)
	cur, _, err := svc.rankings.OverviewCurrent(context.Background(), 1)
	require.NoError(t, err)
	require.NotNil(t, cur)
}
