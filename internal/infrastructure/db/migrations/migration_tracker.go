package migrations

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"gorm.io/gorm"
)

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

func (r *MigrationRegistry) RunAll() error {
	bootstrapTx := r.db.Begin()
	if bootstrapTx.Error != nil {
		return fmt.Errorf("failed to begin migration tracker transaction: %w", bootstrapTx.Error)
	}
	if err := bootstrapTx.Exec(`SELECT pg_advisory_xact_lock(?)`, int64(0x444e4a4d49475241)).Error; err != nil {
		bootstrapTx.Rollback()
		return fmt.Errorf("failed to acquire migration tracker lock: %w", err)
	}
	if err := NewMigrationRegistry(bootstrapTx).EnsureTrackerTable(); err != nil {
		bootstrapTx.Rollback()
		return err
	}
	if err := bootstrapTx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit migration tracker bootstrap: %w", err)
	}

	for _, migration := range r.migrations {
		tx := r.db.Begin()
		if tx.Error != nil {
			return fmt.Errorf("failed to begin transaction for migration %s: %w", migration.Name, tx.Error)
		}
		// A transaction-scoped advisory lock serializes concurrent migration
		// runners. The status check happens only after the lock is held, which
		// prevents two deploys from applying the same migration together.
		if err := tx.Exec(`SELECT pg_advisory_xact_lock(?)`, int64(0x444e4a4d49475241)).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to acquire migration lock for %s: %w", migration.Name, err)
		}

		hasRun, err := NewMigrationRegistry(tx).HasMigration(migration.Name)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("error checking migration %s: %w", migration.Name, err)
		}
		if hasRun {
			if err := r.validateAppliedMigration(tx, migration); err != nil {
				tx.Rollback()
				return err
			}
			if err := tx.Commit().Error; err != nil {
				return fmt.Errorf("failed to commit checksum validation for migration %s: %w", migration.Name, err)
			}
			continue
		}

		if err := migration.Up(tx); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %s failed: %w", migration.Name, err)
		}

		record := &MigrationRecord{
			Name:        migration.Name,
			Description: migration.Description,
			Version:     migration.Version,
			Checksum:    migration.Checksum(),
		}
		if err := tx.Create(record).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to record migration %s: %w", migration.Name, err)
		}

		if err := tx.Commit().Error; err != nil {
			return fmt.Errorf("failed to commit migration %s: %w", migration.Name, err)
		}
	}

	return nil
}
