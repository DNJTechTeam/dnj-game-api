package db

import (
	"context"
	"sync/atomic"
	"time"

	"gorm.io/gorm"
)

// QueryStats accumulates, per request, how many statements hit the database
// and how long they took. It is attached to the request context by the HTTP
// observability middleware and read back when the request finishes so the
// access log can separate database time from everything else (Lambda CPU,
// serialization, network to the client).
type QueryStats struct {
	Queries    atomic.Int64
	TotalNanos atomic.Int64
}

// Duration returns the accumulated time spent in database statements.
func (s *QueryStats) Duration() time.Duration {
	if s == nil {
		return 0
	}
	return time.Duration(s.TotalNanos.Load())
}

// Count returns how many statements were executed.
func (s *QueryStats) Count() int64 {
	if s == nil {
		return 0
	}
	return s.Queries.Load()
}

type queryStatsKey struct{}
type queryStartedKey struct{}

// WithQueryStats returns a context carrying a fresh QueryStats collector.
func WithQueryStats(ctx context.Context) (context.Context, *QueryStats) {
	stats := &QueryStats{}
	return context.WithValue(ctx, queryStatsKey{}, stats), stats
}

// QueryStatsFromContext returns the collector attached to ctx, or nil.
func QueryStatsFromContext(ctx context.Context) *QueryStats {
	if ctx == nil {
		return nil
	}
	stats, _ := ctx.Value(queryStatsKey{}).(*QueryStats)
	return stats
}

// registerQueryStatsCallbacks hooks every GORM operation so statements are
// counted and timed against the QueryStats found in the statement context.
// Requests without a collector pay only a context lookup.
func registerQueryStatsCallbacks(db *gorm.DB) error {
	before := func(tx *gorm.DB) {
		if QueryStatsFromContext(tx.Statement.Context) == nil {
			return
		}
		tx.Statement.Context = context.WithValue(tx.Statement.Context, queryStartedKey{}, time.Now())
	}
	after := func(tx *gorm.DB) {
		stats := QueryStatsFromContext(tx.Statement.Context)
		if stats == nil {
			return
		}
		startedAt, ok := tx.Statement.Context.Value(queryStartedKey{}).(time.Time)
		if !ok {
			return
		}
		stats.Queries.Add(1)
		stats.TotalNanos.Add(int64(time.Since(startedAt)))
	}

	callbacks := db.Callback()
	if err := callbacks.Create().Before("gorm:create").Register("query_stats:before_create", before); err != nil {
		return err
	}
	if err := callbacks.Create().After("gorm:create").Register("query_stats:after_create", after); err != nil {
		return err
	}
	if err := callbacks.Query().Before("gorm:query").Register("query_stats:before_query", before); err != nil {
		return err
	}
	if err := callbacks.Query().After("gorm:query").Register("query_stats:after_query", after); err != nil {
		return err
	}
	if err := callbacks.Update().Before("gorm:update").Register("query_stats:before_update", before); err != nil {
		return err
	}
	if err := callbacks.Update().After("gorm:update").Register("query_stats:after_update", after); err != nil {
		return err
	}
	if err := callbacks.Delete().Before("gorm:delete").Register("query_stats:before_delete", before); err != nil {
		return err
	}
	if err := callbacks.Delete().After("gorm:delete").Register("query_stats:after_delete", after); err != nil {
		return err
	}
	if err := callbacks.Row().Before("gorm:row").Register("query_stats:before_row", before); err != nil {
		return err
	}
	if err := callbacks.Row().After("gorm:row").Register("query_stats:after_row", after); err != nil {
		return err
	}
	if err := callbacks.Raw().Before("gorm:raw").Register("query_stats:before_raw", before); err != nil {
		return err
	}
	if err := callbacks.Raw().After("gorm:raw").Register("query_stats:after_raw", after); err != nil {
		return err
	}
	return nil
}
