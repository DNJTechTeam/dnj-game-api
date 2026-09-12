package db

import (
	"context"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/common"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var gormDbConnection *gorm.DB

func GetConnection() *gorm.DB {
	return gormDbConnection
}

func SetConnection(db *gorm.DB) {
	gormDbConnection = db
}

func Init() *gorm.DB {
	return initConnection(false)
}

// InitAPI builds the pool without an eager ping. This allows the HTTP process
// to expose liveness and a deterministic readiness=503 when PostgreSQL is
// temporarily unavailable. Migration commands keep using Init, which remains
// fail-fast and blocks unsafe deploys.
func InitAPI() *gorm.DB {
	return initConnection(true)
}

func initConnection(disableAutomaticPing bool) *gorm.DB {
	if gormDbConnection != nil {
		return gormDbConnection
	}

	databaseURL := buildDatabaseURL()

	config := &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		DisableAutomaticPing:                     disableAutomaticPing,
	}
	if !common.EnvironmentIs(common.EnvironmentLocalhost) {
		config.Logger = logger.Default.LogMode(logger.Silent)
	}

	db, err := gorm.Open(NewDialector(databaseURL), config)

	if err != nil {
		log.Fatalln(err)
	}

	if err := registerQueryStatsCallbacks(db); err != nil {
		log.Fatalln("Failed to register query stats callbacks:", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalln("Failed to get underlying sql.DB:", err)
	}

	configurePool(sqlDB)

	if disableAutomaticPing {
		warmUpConnection(sqlDB)
	}

	SetConnection(db)

	return db
}

// supavisorTransactionPort is the Supabase pooler port that runs in
// transaction mode, where server-side prepared statements are unavailable.
const supavisorTransactionPort = "6543"

// NewDialector builds the GORM PostgreSQL dialector shared by the API, the
// workers and the test suite, so protocol settings stay identical everywhere.
//
// Supavisor in transaction mode (port 6543) does not support server-side
// prepared statements, so pgx must use the simple protocol there. It is
// enabled automatically for that port and can be forced either way with
// DB_PREFER_SIMPLE_PROTOCOL=true|false.
func NewDialector(dsn string) gorm.Dialector {
	return postgres.New(postgres.Config{
		DSN:                  dsn,
		PreferSimpleProtocol: PreferSimpleProtocol(),
	})
}

// PreferSimpleProtocol reports whether pgx should skip prepared statements.
func PreferSimpleProtocol() bool {
	defaultValue := strings.TrimSpace(os.Getenv("DB_PORT")) == supavisorTransactionPort
	return envBool("DB_PREFER_SIMPLE_PROTOCOL", defaultValue)
}

// Pool defaults. A Lambda container serves one request at a time, so the pool
// only needs to cover the parallelism inside a single request (a transaction
// plus the reads issued alongside it). Idle connections must NOT be dropped
// below the open limit: every connection re-opened against Supabase costs a
// TLS handshake plus pooler authentication (pgbouncer.get_auth), which
// dominated response time before this tuning.
const (
	defaultMaxOpenConns    = 2
	defaultConnMaxIdleTime = 5 * time.Minute
	defaultConnMaxLifetime = time.Hour
	warmUpTimeout          = 3 * time.Second
)

func configurePool(sqlDB interface {
	SetMaxIdleConns(int)
	SetMaxOpenConns(int)
	SetConnMaxIdleTime(time.Duration)
	SetConnMaxLifetime(time.Duration)
}) {
	maxOpen := envInt("DB_MAX_OPEN_CONNS", defaultMaxOpenConns)
	if maxOpen < 1 {
		maxOpen = 1
	}
	sqlDB.SetMaxOpenConns(maxOpen)
	// Keep every connection we are allowed to open; closing idle ones only to
	// reopen them on the next request is the churn we are eliminating.
	sqlDB.SetMaxIdleConns(maxOpen)
	sqlDB.SetConnMaxIdleTime(time.Duration(envInt("DB_CONN_MAX_IDLE_SECONDS", int(defaultConnMaxIdleTime/time.Second))) * time.Second)
	sqlDB.SetConnMaxLifetime(time.Duration(envInt("DB_CONN_MAX_LIFETIME_SECONDS", int(defaultConnMaxLifetime/time.Second))) * time.Second)
}

// warmUpConnection establishes the first connection during process start-up
// (the Lambda init phase) instead of inside the first request. Failure is not
// fatal: readiness keeps reporting 503 until PostgreSQL is reachable.
func warmUpConnection(sqlDB interface {
	PingContext(context.Context) error
}) {
	ctx, cancel := context.WithTimeout(context.Background(), warmUpTimeout)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		log.Printf("database warm-up ping failed (readiness will report unavailable until it recovers): %v", err)
	}
}

func envInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		log.Printf("ignoring invalid %s=%q: %v", key, raw, err)
		return fallback
	}
	return value
}

func envBool(key string, fallback bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		log.Printf("ignoring invalid %s=%q: %v", key, raw, err)
		return fallback
	}
	return value
}

// buildDatabaseURL monta a URL de conexão a partir das envs DB_*, removendo
// espaços/quebras de linha acidentais e escapando credenciais com caracteres
// especiais (ex.: "?", "@", "$", "#").
func buildDatabaseURL() string {
	get := func(key string) string { return strings.TrimSpace(common.GetEnv(key)) }

	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(get("DB_USER"), get("DB_PASSWORD")),
		Host:   get("DB_HOST") + ":" + get("DB_PORT"),
		Path:   "/" + get("DB_NAME"),
	}
	return u.String()
}
