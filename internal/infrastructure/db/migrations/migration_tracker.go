package migrations

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"gorm.io/gorm"
)

const migrationLockID = int64(1)

type MigrationRecord struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement"`
	Name        string    `gorm:"uniqueIndex;not null"`
	Description string    `gorm:"default:null"`
	Version     string    `gorm:"not null"`
	Checksum    string    `gorm:"size:64;not null;default:''"`
	AppliedAt   time.Time `gorm:"autoCreateTime:nano"`
}

func (m *MigrationRecord) TableName() string {
	return "schema_migrations"
}

type Migration struct {
	Name        string
	Description string
	Version     string
	// Definition is an immutable, human-readable revision of the migration.
	// Change it only when changing an unapplied migration. Applied migrations
	// fail closed when their computed checksum no longer matches the database.
	Definition string
	Up         func(*gorm.DB) error
	Down       func(*gorm.DB) error
}

func (m Migration) Checksum() string {
	sum := sha256.Sum256([]byte(m.Name + "\x00" + m.Version + "\x00" + m.Description + "\x00" + m.Definition))
	return hex.EncodeToString(sum[:])
}

type MigrationRegistry struct {
	migrations []Migration
	db         *gorm.DB
}

func NewMigrationRegistry(db *gorm.DB) *MigrationRegistry {
	return &MigrationRegistry{
		migrations: make([]Migration, 0),
		db:         db,
	}
}

func (r *MigrationRegistry) Register(migration Migration) {
	if migration.Definition == "" {
		panic(fmt.Sprintf("migration %s must declare an immutable definition", migration.Name))
	}
	r.migrations = append(r.migrations, migration)
}

// Migrations returns a copy of the registered migration list. Tests use it to
// execute every Up function repeatedly instead of merely exercising the
// tracker skip path.
func (r *MigrationRegistry) Migrations() []Migration {
	result := make([]Migration, len(r.migrations))
	copy(result, r.migrations)
	return result
}

func (r *MigrationRegistry) EnsureTrackerTable() error {
	if err := r.db.AutoMigrate(&MigrationRecord{}); err != nil {
		return fmt.Errorf("failed to create or migrate migration tracker table: %w", err)
	}
	return nil
}

func (r *MigrationRegistry) HasMigration(name string) (bool, error) {
	var count int64
	err := r.db.Model(&MigrationRecord{}).
		Where("name = ?", name).
		Count(&count).Error

	if err != nil {
		return false, fmt.Errorf("failed to check migration status: %w", err)
	}

	return count > 0, nil
}

func (r *MigrationRegistry) RecordMigration(migration Migration) error {
	record := &MigrationRecord{
		Name:        migration.Name,
		Description: migration.Description,
		Version:     migration.Version,
		Checksum:    migration.Checksum(),
	}

	if err := r.db.Create(record).Error; err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}

	return nil
}

func (r *MigrationRegistry) validateAppliedMigration(tx *gorm.DB, migration Migration) error {
	var record MigrationRecord
	if err := tx.Where("name = ?", migration.Name).First(&record).Error; err != nil {
		return fmt.Errorf("failed to load migration %s: %w", migration.Name, err)
	}

	expected := migration.Checksum()
	if record.Checksum == "" {
		if err := tx.Model(&record).Update("checksum", expected).Error; err != nil {
			return fmt.Errorf("failed to backfill checksum for migration %s: %w", migration.Name, err)
		}
		return nil
	}

	if record.Checksum != expected {
		return fmt.Errorf("migration %s checksum mismatch: applied migration definitions are immutable", migration.Name)
	}
	return nil
}

func (r *MigrationRegistry) GetAppliedMigrations() ([]string, error) {
	var records []MigrationRecord
	err := r.db.Order("applied_at ASC").Find(&records).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get applied migrations: %w", err)
	}

	names := make([]string, len(records))
	for i, record := range records {
		names[i] = record.Name
	}

	return names, nil
}

// ensureMigrationLock creates the singleton row used to serialize migration
// runners. A row lock is supported by PostgreSQL and CockroachDB, unlike
// PostgreSQL advisory locks, and keeps the coordination state in the same
// database whose schema is being changed.
func (r *MigrationRegistry) ensureMigrationLock() error {
	if err := r.db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migration_lock (
			id BIGINT PRIMARY KEY
		)
	`).Error; err != nil {
		return fmt.Errorf("failed to create migration lock table: %w", err)
	}

	if err := r.db.Exec(`
		INSERT INTO schema_migration_lock (id)
		VALUES (?)
		ON CONFLICT (id) DO NOTHING
	`, migrationLockID).Error; err != nil {
		return fmt.Errorf("failed to initialize migration lock row: %w", err)
	}

	return nil
}

func acquireMigrationLock(tx *gorm.DB) error {
	var id int64
	if err := tx.Raw(`
		SELECT id
		FROM schema_migration_lock
		WHERE id = ?
		FOR UPDATE
	`, migrationLockID).Scan(&id).Error; err != nil {
		return err
	}
	if id != migrationLockID {
		return fmt.Errorf("migration lock row %d is missing", migrationLockID)
	}
	return nil
}

func (r *MigrationRegistry) runLockedTransaction(label string, run func(*gorm.DB) error) error {
	tx := r.db.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to begin %s transaction: %w", label, tx.Error)
	}
	if err := acquireMigrationLock(tx); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to acquire lock for %s: %w", label, err)
	}
	if err := run(tx); err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit %s: %w", label, err)
	}
	return nil
}

func (r *MigrationRegistry) RunAll() error {
	if err := r.ensureMigrationLock(); err != nil {
		return err
	}

	if err := r.runLockedTransaction("migration tracker bootstrap", func(tx *gorm.DB) error {
		return NewMigrationRegistry(tx).EnsureTrackerTable()
	}); err != nil {
		return err
	}

	for _, migration := range r.migrations {
		if err := r.runLockedTransaction("migration "+migration.Name, func(tx *gorm.DB) error {
			hasRun, err := NewMigrationRegistry(tx).HasMigration(migration.Name)
			if err != nil {
				return fmt.Errorf("error checking migration %s: %w", migration.Name, err)
			}
			if hasRun {
				return r.validateAppliedMigration(tx, migration)
			}

			if err := migration.Up(tx); err != nil {
				return fmt.Errorf("migration %s failed: %w", migration.Name, err)
			}

			record := &MigrationRecord{
				Name:        migration.Name,
				Description: migration.Description,
				Version:     migration.Version,
				Checksum:    migration.Checksum(),
			}
			if err := tx.Create(record).Error; err != nil {
				return fmt.Errorf("failed to record migration %s: %w", migration.Name, err)
			}

			return nil
		}); err != nil {
			return err
		}
	}

	return nil
}
