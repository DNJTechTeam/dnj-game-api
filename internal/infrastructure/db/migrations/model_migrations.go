package migrations

import (
	"fmt"
	"os"

	"github.com/scienceandcode/nucleus-api/internal/infrastructure/db/models"
	"gorm.io/gorm"
)

// createModelMigration builds an idempotent "create table" migration. If the
// table exists it runs AutoMigrate (which only adds missing columns/indexes);
// otherwise it creates the table. AutoMigrate is itself idempotent, so this
// migration is safe to run any number of times.
func createModelMigration(name, version string, model interface{}) Migration {
	return Migration{
		Name:        name,
		Description: fmt.Sprintf("Create and migrate table for %s", name),
		Version:     version,
		Up: func(db *gorm.DB) error {
			migrator := db.Migrator()

			if migrator.HasTable(model) {
				if err := db.AutoMigrate(model); err != nil {
					return fmt.Errorf("failed to migrate table %s: %w", name, err)
				}
			} else {
				if err := migrator.CreateTable(model); err != nil {
					return fmt.Errorf("failed to create table %s: %w", name, err)
				}
			}

			return nil
		},
		Down: func(db *gorm.DB) error {
			migrator := db.Migrator()
			if migrator.HasTable(model) {
				if err := migrator.DropTable(model); err != nil {
					return fmt.Errorf("failed to drop table %s: %w", name, err)
				}
			}
			return nil
		},
	}
}

// RegisterModelMigrations declares every migration, in order. Each migration
// runs exactly once (tracked in schema_migrations), but every Up MUST still be
// written idempotently — see docs/migrations.md.
func RegisterModelMigrations(registry *MigrationRegistry) {
	registry.Register(createModelMigration(
		"create_users_table",
		"1.0.0",
		&models.User{},
	))

	registry.Register(createModelMigration(
		"create_tasks_table",
		"1.0.0",
		&models.Task{},
	))

	// Example seed migration. It is fully idempotent: it no-ops when the env
	// vars are absent and when the user already exists. Use this pattern for
	// bootstrapping reference data.
	registry.Register(Migration{
		Name:        "seed_admin_user",
		Description: "Seed an initial admin user from SEED_ADMIN_EMAIL / SEED_ADMIN_PASSWORD",
		Version:     "1.0.0",
		Up: func(db *gorm.DB) error {
			email := os.Getenv("SEED_ADMIN_EMAIL")
			password := os.Getenv("SEED_ADMIN_PASSWORD")
			if email == "" || password == "" {
				return nil // nothing to seed
			}

			var existing models.User
			if db.Where("email = ?", email).First(&existing).Error == nil {
				return nil // already seeded
			}

			now := db.NowFunc()
			user := models.User{
				Email:     email,
				Name:      "Admin",
				Password:  password, // hashed by the User BeforeCreate hook
				CreatedAt: now,
				UpdatedAt: now,
			}
			if err := db.Create(&user).Error; err != nil {
				return fmt.Errorf("failed to seed admin user: %w", err)
			}

			// Mark email + password as confirmed so the admin can log in directly.
			return db.Model(&user).Updates(map[string]interface{}{
				"email_confirmed_at":    now,
				"password_confirmed_at": now,
			}).Error
		},
		Down: func(db *gorm.DB) error {
			email := os.Getenv("SEED_ADMIN_EMAIL")
			if email == "" {
				return nil
			}
			return db.Where("email = ?", email).Delete(&models.User{}).Error
		},
	})
}
