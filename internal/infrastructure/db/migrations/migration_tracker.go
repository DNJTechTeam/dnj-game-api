package migrations

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

type MigrationRecord struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement"`
	Name        string    `gorm:"uniqueIndex;not null"`
	Description string    `gorm:"default:null"`
	Version     string    `gorm:"not null"`
	AppliedAt   time.Time `gorm:"autoCreateTime:nano"`
}

func (m *MigrationRecord) TableName() string {
	return "schema_migrations"
}

type Migration struct {
	Name        string
	Description string
	Version     string
	Up          func(*gorm.DB) error
	Down        func(*gorm.DB) error
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
	r.migrations = append(r.migrations, migration)
}

func (r *MigrationRegistry) EnsureTrackerTable() error {
	migrator := r.db.Migrator()

	if !migrator.HasTable(&MigrationRecord{}) {
		if err := migrator.CreateTable(&MigrationRecord{}); err != nil {
			return fmt.Errorf("failed to create migration tracker table: %w", err)
		}
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
	}

	if err := r.db.Create(record).Error; err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
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
	if err := r.EnsureTrackerTable(); err != nil {
		return err
	}

	for _, migration := range r.migrations {
		hasRun, err := r.HasMigration(migration.Name)
		if err != nil {
			return fmt.Errorf("error checking migration %s: %w", migration.Name, err)
		}

		if hasRun {
			continue
		}

		tx := r.db.Begin()
		if tx.Error != nil {
			return fmt.Errorf("failed to begin transaction for migration %s: %w", migration.Name, tx.Error)
		}

		if err := migration.Up(tx); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %s failed: %w", migration.Name, err)
		}

		record := &MigrationRecord{
			Name:        migration.Name,
			Description: migration.Description,
			Version:     migration.Version,
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
