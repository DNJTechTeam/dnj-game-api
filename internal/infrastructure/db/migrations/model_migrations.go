package migrations

import (
	"fmt"

	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
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
		Definition:  "model-table-v1:" + name,
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
	// create_users_table was edited in place (pre-release, no production
	// data yet) to drop the legacy password-based columns and add the
	// passwordless-identity columns (document/role/group_id). AutoMigrate
	// never drops columns, so the old ones are dropped explicitly before it
	// runs. This Up is still idempotent: every step is guarded.
	registry.Register(Migration{
		Name:        "create_users_table",
		Description: "Create and migrate table for create_users_table",
		Version:     "1.0.0",
		Definition:  "users-passwordless-columns-v1",
		Up: func(db *gorm.DB) error {
			migrator := db.Migrator()

			if migrator.HasTable(&models.User{}) {
				for _, column := range []string{"password", "email_confirmed_at", "password_confirmed_at"} {
					if migrator.HasColumn(&models.User{}, column) {
						if err := migrator.DropColumn(&models.User{}, column); err != nil {
							return fmt.Errorf("failed to drop column %s from users: %w", column, err)
						}
					}
				}

				if err := db.AutoMigrate(&models.User{}); err != nil {
					return fmt.Errorf("failed to migrate table create_users_table: %w", err)
				}
			} else {
				if err := migrator.CreateTable(&models.User{}); err != nil {
					return fmt.Errorf("failed to create table create_users_table: %w", err)
				}
			}

			return nil
		},
		Down: func(db *gorm.DB) error {
			migrator := db.Migrator()
			if migrator.HasTable(&models.User{}) {
				if err := migrator.DropTable(&models.User{}); err != nil {
					return fmt.Errorf("failed to drop table create_users_table: %w", err)
				}
			}
			return nil
		},
	})

	registry.Register(createModelMigration(
		"create_tasks_table",
		"1.0.0",
		&models.Task{},
	))

	registry.Register(createModelMigration(
		"create_groups_table",
		"1.1.0",
		&models.Group{},
	))

	registry.Register(createModelMigration(
		"create_subscription_webhooks_table",
		"1.1.0",
		&models.SubscriptionWebhook{},
	))

	registry.Register(createModelMigration(
		"create_subscription_webhook_verification_codes_table",
		"1.1.0",
		&models.SubscriptionWebhookVerificationCode{},
	))

	registry.Register(Migration{
		Name:        "switch_subscription_webhook_verification_codes_dedup_key_to_document",
		Description: "Dedup subscription webhook verification codes by document (CPF) instead of email",
		Version:     "1.2.0",
		Definition:  "verification-code-document-dedup-v1",
		Up: func(db *gorm.DB) error {
			migrator := db.Migrator()
			if !migrator.HasTable(&models.SubscriptionWebhookVerificationCode{}) {
				return nil
			}

			if err := db.Exec(`DROP INDEX IF EXISTS idx_subscription_webhook_verification_codes_email`).Error; err != nil {
				return fmt.Errorf("failed to drop email unique index: %w", err)
			}

			if err := db.Exec(`UPDATE subscription_webhook_verification_codes SET document = '' WHERE document IS NULL`).Error; err != nil {
				return fmt.Errorf("failed to backfill null documents: %w", err)
			}

			if err := db.Exec(`ALTER TABLE subscription_webhook_verification_codes ALTER COLUMN document SET NOT NULL`).Error; err != nil {
				return fmt.Errorf("failed to set document not null: %w", err)
			}

			if err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_subscription_webhook_verification_codes_document ON subscription_webhook_verification_codes (document)`).Error; err != nil {
				return fmt.Errorf("failed to create document unique index: %w", err)
			}

			if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_subscription_webhook_verification_codes_email ON subscription_webhook_verification_codes (email)`).Error; err != nil {
				return fmt.Errorf("failed to create email index: %w", err)
			}

			return nil
		},
		Down: func(db *gorm.DB) error {
			migrator := db.Migrator()
			if !migrator.HasTable(&models.SubscriptionWebhookVerificationCode{}) {
				return nil
			}

			if err := db.Exec(`DROP INDEX IF EXISTS idx_subscription_webhook_verification_codes_document`).Error; err != nil {
				return err
			}

			if err := db.Exec(`DROP INDEX IF EXISTS idx_subscription_webhook_verification_codes_email`).Error; err != nil {
				return err
			}

			if err := db.Exec(`ALTER TABLE subscription_webhook_verification_codes ALTER COLUMN document DROP NOT NULL`).Error; err != nil {
				return err
			}

			return db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_subscription_webhook_verification_codes_email ON subscription_webhook_verification_codes (email)`).Error
		},
	})
}
