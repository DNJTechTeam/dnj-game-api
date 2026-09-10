package db

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type fakePool struct {
	maxIdle, maxOpen int
	idleTime         time.Duration
	lifetime         time.Duration
}

func (f *fakePool) SetMaxIdleConns(n int)                { f.maxIdle = n }
func (f *fakePool) SetMaxOpenConns(n int)                { f.maxOpen = n }
func (f *fakePool) SetConnMaxIdleTime(d time.Duration)   { f.idleTime = d }
func (f *fakePool) SetConnMaxLifetime(d time.Duration)   { f.lifetime = d }

func TestConfigurePool_DefaultsKeepIdleEqualToOpen(t *testing.T) {
	// given
	t.Setenv("DB_MAX_OPEN_CONNS", "")
	t.Setenv("DB_CONN_MAX_IDLE_SECONDS", "")
	t.Setenv("DB_CONN_MAX_LIFETIME_SECONDS", "")
	pool := &fakePool{}

	// when
	configurePool(pool)

	// then
	assert.Equal(t, defaultMaxOpenConns, pool.maxOpen)
	assert.Equal(t, pool.maxOpen, pool.maxIdle, "idle connections must not be dropped below the open limit")
	assert.Equal(t, defaultConnMaxIdleTime, pool.idleTime)
	assert.Equal(t, defaultConnMaxLifetime, pool.lifetime)
}

func TestConfigurePool_ReadsEnvAndClampsMinimum(t *testing.T) {
	// given
	t.Setenv("DB_MAX_OPEN_CONNS", "0")
	t.Setenv("DB_CONN_MAX_IDLE_SECONDS", "30")
	t.Setenv("DB_CONN_MAX_LIFETIME_SECONDS", "not-a-number")
	pool := &fakePool{}

	// when
	configurePool(pool)

	// then
	assert.Equal(t, 1, pool.maxOpen)
	assert.Equal(t, 1, pool.maxIdle)
	assert.Equal(t, 30*time.Second, pool.idleTime)
	assert.Equal(t, defaultConnMaxLifetime, pool.lifetime, "invalid values fall back to the default")
}

func TestPreferSimpleProtocol_FollowsTransactionPoolerPort(t *testing.T) {
	// given
	t.Setenv("DB_PREFER_SIMPLE_PROTOCOL", "")

	// when / then
	t.Setenv("DB_PORT", "5432")
	assert.False(t, PreferSimpleProtocol())

	t.Setenv("DB_PORT", supavisorTransactionPort)
	assert.True(t, PreferSimpleProtocol())

	t.Setenv("DB_PREFER_SIMPLE_PROTOCOL", "false")
	assert.False(t, PreferSimpleProtocol(), "explicit env overrides the port default")

	t.Setenv("DB_PORT", "5432")
	t.Setenv("DB_PREFER_SIMPLE_PROTOCOL", "true")
	assert.True(t, PreferSimpleProtocol())
}

type fakePinger struct{ err error }

func (f fakePinger) PingContext(context.Context) error { return f.err }

func TestWarmUpConnection_DoesNotPanicOnFailure(t *testing.T) {
	// given
	pinger := fakePinger{err: context.DeadlineExceeded}

	// when / then
	assert.NotPanics(t, func() { warmUpConnection(pinger) })
	assert.NotPanics(t, func() { warmUpConnection(fakePinger{}) })
}

func TestQueryStats_CountsStatementsOnlyWhenCollectorPresent(t *testing.T) {
	// given
	// DryRun runs the whole callback chain without touching PostgreSQL, which
	// is exactly what the collector hooks into.
	conn, err := gorm.Open(
		NewDialector("postgres://unreachable:unreachable@127.0.0.1:1/unreachable"),
		&gorm.Config{DryRun: true, DisableAutomaticPing: true},
	)
	require.NoError(t, err)
	require.NoError(t, registerQueryStatsCallbacks(conn))

	ctx, stats := WithQueryStats(context.Background())

	// when
	require.NoError(t, conn.WithContext(ctx).Exec("SELECT 1").Error)
	require.NoError(t, conn.WithContext(ctx).Exec("SELECT 2").Error)
	require.NoError(t, conn.WithContext(context.Background()).Exec("SELECT 3").Error)

	// then
	assert.Equal(t, int64(2), stats.Count(), "statements without a collector in context are ignored")
	assert.Greater(t, stats.Duration(), time.Duration(0))
	assert.Nil(t, QueryStatsFromContext(context.Background()))
	var nilStats *QueryStats
	assert.Equal(t, int64(0), nilStats.Count())
	assert.Equal(t, time.Duration(0), nilStats.Duration())
}
