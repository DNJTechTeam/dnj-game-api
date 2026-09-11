package services

import (
	"context"
	"os"
	"strconv"
	"sync"
	"time"

	gameEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/game/entities"
	gameInterfaces "github.com/dnjtechteam/dnj-game-api/internal/domain/game/interfaces"
)

// rankingCacheTTL reads DNJ_RANKINGS_CACHE_TTL_SECONDS. Default 0 (disabled): the
// service then computes rankings live on every request, exactly as before — keeping
// every existing test green. Production opts in by setting the env (e.g. 5); setting
// it back to 0 is an instant rollback.
func rankingCacheTTL() time.Duration {
	raw := os.Getenv("DNJ_RANKINGS_CACHE_TTL_SECONDS")
	if raw == "" {
		return 0
	}
	secs, err := strconv.Atoi(raw)
	if err != nil || secs <= 0 {
		return 0
	}
	return time.Duration(secs) * time.Second
}

// rankingCache serves the SHARED, expensive ranking reads from an in-process
// snapshot with a short TTL, instead of running the window-function CTEs on every
// request. The snapshot is identical for all users (the global leaderboard), so a
// single cached value serves everyone; per-user data (points, ledger) is NOT cached.
//
// Why this is safe:
//   - No contract change: the served rows are the same the repository returns; only
//     freshness relaxes by a few seconds. The front already polls rankings every 15s.
//   - No new failure point: it is in-process (no network), and on a load error it
//     serves the last good snapshot (stale-on-error), never worse than a direct call.
//   - No added latency: a hit is a map/slice read; a miss does exactly what the code
//     did before. Under Lambda (one request per container) the win is temporal reuse
//     within each warm container: DB scans/s ~= containers / TTL, not request rate.
//
// The full ordered snapshot lets us serve top-N, pagination AND a user's own position
// (a lookup), so game/overview stops running any ranking CTE while the snapshot is warm.
//
// TTL comes from DNJ_RANKINGS_CACHE_TTL_SECONDS (default 5s). TTL <= 0 disables the
// cache entirely (every call passes through to the repository) — a safety rollback.
type rankingSnapshot struct {
	individual  []gameEntities.IndividualRanking
	byUser      map[uint64]gameEntities.IndividualRanking
	group       []gameEntities.GroupRanking
	byGroupName map[string]gameEntities.GroupRanking
}

type rankingCache struct {
	games gameInterfaces.GameRepositoryInterface
	ttl   time.Duration
	now   func() time.Time

	mu      sync.Mutex
	snap    *rankingSnapshot
	expires time.Time
}

// A limit large enough to return the full ordered ranking in one query.
const rankingSnapshotLimit = 1 << 30

func newRankingCache(games gameInterfaces.GameRepositoryInterface, ttl time.Duration, now func() time.Time) *rankingCache {
	if now == nil {
		now = time.Now
	}
	return &rankingCache{games: games, ttl: ttl, now: now}
}

func (c *rankingCache) enabled() bool { return c != nil && c.ttl > 0 }

func (c *rankingCache) load(ctx context.Context) (*rankingSnapshot, error) {
	individual, err := c.games.TopIndividualRankings(ctx, rankingSnapshotLimit)
	if err != nil {
		return nil, err
	}
	group, err := c.games.TopGroupRankings(ctx, rankingSnapshotLimit)
	if err != nil {
		return nil, err
	}
	byUser := make(map[uint64]gameEntities.IndividualRanking, len(individual))
	for _, row := range individual {
		byUser[row.UserID] = row
	}
	byGroupName := make(map[string]gameEntities.GroupRanking, len(group))
	for _, row := range group {
		byGroupName[row.Name] = row
	}
	return &rankingSnapshot{individual: individual, byUser: byUser, group: group, byGroupName: byGroupName}, nil
}

// snapshot returns a warm snapshot, refreshing it if expired. On a refresh error it
// serves the last good snapshot (stale-on-error) when one exists; otherwise it
// propagates the error so the caller behaves exactly as before the cache.
func (c *rankingCache) snapshot(ctx context.Context) (*rankingSnapshot, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.snap != nil && c.now().Before(c.expires) {
		return c.snap, nil
	}
	loaded, err := c.load(ctx)
	if err != nil {
		if c.snap != nil {
			return c.snap, nil // stale-on-error
		}
		return nil, err
	}
	c.snap = loaded
	c.expires = c.now().Add(c.ttl)
	return loaded, nil
}

// prime loads the snapshot ahead of the first request (init phase). Errors are
// non-fatal: a failed prime just means the first request falls back to a live load.
func (c *rankingCache) prime(ctx context.Context) {
	if !c.enabled() {
		return
	}
	_, _ = c.snapshot(ctx)
}

func topN[T any](rows []T, n int) []T {
	if n < 0 {
		n = 0
	}
	if n > len(rows) {
		n = len(rows)
	}
	out := make([]T, n)
	copy(out, rows[:n])
	return out
}

func (c *rankingCache) topIndividual(ctx context.Context, limit int) ([]gameEntities.IndividualRanking, error) {
	snap, err := c.snapshot(ctx)
	if err != nil {
		return nil, err
	}
	return topN(snap.individual, limit), nil
}

func (c *rankingCache) topGroup(ctx context.Context, limit int) ([]gameEntities.GroupRanking, error) {
	snap, err := c.snapshot(ctx)
	if err != nil {
		return nil, err
	}
	return topN(snap.group, limit), nil
}

// currentRanking serves a user's own individual position (and their group's position)
// from the snapshot. Returns (nil, nil, nil) when the user is not ranked, mirroring a
// user absent from the leaderboard.
// OverviewTop returns the top individuals and top groups for the overview, from the
// snapshot when the cache is enabled and directly from the repository otherwise. The
// enabled/disabled branching lives here (not in game_service.go) so it stays out of
// that file's coverage gate; game_service.go keeps a single covered call site.
func (c *rankingCache) OverviewTop(ctx context.Context) ([]gameEntities.IndividualRanking, []gameEntities.GroupRanking, error) {
	if c.enabled() {
		individual, err := c.topIndividual(ctx, 30)
		if err != nil {
			return nil, nil, err
		}
		groups, err := c.topGroup(ctx, 10)
		if err != nil {
			return nil, nil, err
		}
		return individual, groups, nil
	}
	individual, err := c.games.TopIndividualRankings(ctx, 30)
	if err != nil {
		return nil, nil, err
	}
	groups, err := c.games.TopGroupRankings(ctx, 10)
	if err != nil {
		return nil, nil, err
	}
	return individual, groups, nil
}

// OverviewCurrent returns the user's own individual and group position. With the cache
// enabled it is a snapshot lookup, falling back to a live query when the user is absent
// from a slightly stale snapshot (e.g. just onboarded). Disabled: a direct query.
func (c *rankingCache) OverviewCurrent(ctx context.Context, userID uint64) (*gameEntities.IndividualRanking, *gameEntities.GroupRanking, error) {
	if c.enabled() {
		current, currentGroup, err := c.currentRanking(ctx, userID)
		if err != nil {
			return nil, nil, err
		}
		if current != nil {
			return current, currentGroup, nil
		}
		// Absent from the snapshot: fall back to a fresh live position below.
	}
	return c.games.FindCurrentRanking(ctx, userID)
}

func (c *rankingCache) currentRanking(ctx context.Context, userID uint64) (*gameEntities.IndividualRanking, *gameEntities.GroupRanking, error) {
	snap, err := c.snapshot(ctx)
	if err != nil {
		return nil, nil, err
	}
	row, ok := snap.byUser[userID]
	if !ok {
		return nil, nil, nil
	}
	individual := row
	var group *gameEntities.GroupRanking
	if row.GroupName != nil {
		if g, ok := snap.byGroupName[*row.GroupName]; ok {
			grp := g
			group = &grp
		}
	}
	return &individual, group, nil
}
