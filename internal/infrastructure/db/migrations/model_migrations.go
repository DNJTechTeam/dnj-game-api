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

	registry.Register(Migration{
		Name:        "expand_users_for_v2_identity",
		Description: "Add secure onboarding fields used by V2 identity",
		Version:     "2.0.0",
		Definition:  "users-v2-identity-expand-v1",
		Up: func(db *gorm.DB) error {
			return db.Exec(`
				ALTER TABLE users
					ADD COLUMN IF NOT EXISTS document_hash VARCHAR(64),
					ADD COLUMN IF NOT EXISTS document_last4 VARCHAR(4),
					ADD COLUMN IF NOT EXISTS onboarding_complete BOOLEAN NOT NULL DEFAULT FALSE
			`).Error
		},
		Down: func(db *gorm.DB) error { return nil },
	})

	registry.Register(Migration{
		Name:        "create_user_identities_table",
		Description: "Create external identity links for Google OIDC",
		Version:     "2.0.0",
		Definition:  "user-identities-v1",
		Up: func(db *gorm.DB) error {
			return db.Exec(`
				CREATE TABLE IF NOT EXISTS user_identities (
					id BIGSERIAL PRIMARY KEY,
					user_id BIGINT NOT NULL,
					provider VARCHAR(32) NOT NULL,
					subject VARCHAR(255) NOT NULL,
					email TEXT NOT NULL,
					created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
					CONSTRAINT user_identities_provider_subject_key UNIQUE (provider, subject)
				)
			`).Error
		},
		Down: func(db *gorm.DB) error { return nil },
	})

	registry.Register(Migration{
		Name:        "create_refresh_sessions_table",
		Description: "Create hashed rotating refresh sessions",
		Version:     "2.0.0",
		Definition:  "refresh-sessions-v1",
		Up: func(db *gorm.DB) error {
			return db.Exec(`
				CREATE TABLE IF NOT EXISTS refresh_sessions (
					id VARCHAR(36) PRIMARY KEY,
					user_id BIGINT NOT NULL,
					family_id VARCHAR(36) NOT NULL,
					token_hash VARCHAR(64) NOT NULL UNIQUE,
					replaced_by_hash VARCHAR(64) NOT NULL DEFAULT '',
					expires_at TIMESTAMPTZ NOT NULL,
					revoked_at TIMESTAMPTZ,
					reuse_detected_at TIMESTAMPTZ,
					created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
					last_used_at TIMESTAMPTZ NOT NULL
				)
			`).Error
		},
		Down: func(db *gorm.DB) error { return nil },
	})

	registry.Register(Migration{
		Name:        "create_v2_identity_indexes",
		Description: "Create lookup indexes for V2 identity and refresh sessions",
		Version:     "2.0.0",
		Definition:  "v2-identity-indexes-v1",
		Up: func(db *gorm.DB) error {
			statements := []string{
				`CREATE UNIQUE INDEX IF NOT EXISTS users_document_hash_unique ON users (document_hash) WHERE document_hash IS NOT NULL AND document_hash <> ''`,
				`CREATE INDEX IF NOT EXISTS user_identities_user_id_idx ON user_identities (user_id)`,
				`CREATE INDEX IF NOT EXISTS refresh_sessions_user_id_idx ON refresh_sessions (user_id)`,
				`CREATE INDEX IF NOT EXISTS refresh_sessions_family_id_idx ON refresh_sessions (family_id)`,
			}
			for _, statement := range statements {
				if err := db.Exec(statement).Error; err != nil {
					return err
				}
			}
			return nil
		},
		Down: func(db *gorm.DB) error { return nil },
	})

	registry.Register(Migration{
		Name:        "expand_iteration3_profiles_groups",
		Description: "Expand profiles with points and create current memberships and hashed group invites",
		Version:     "2.2.0",
		Definition:  "iteration3-profiles-groups-expand-v1",
		Up: func(db *gorm.DB) error {
			statements := []string{
				`ALTER TABLE users ADD COLUMN IF NOT EXISTS points INTEGER NOT NULL DEFAULT 0`,
				`CREATE TABLE IF NOT EXISTS group_memberships (
					id BIGSERIAL PRIMARY KEY,
					user_id BIGINT NOT NULL UNIQUE,
					group_id BIGINT NOT NULL,
					joined_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
					created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
				)`,
				`CREATE TABLE IF NOT EXISTS group_invites (
					id BIGSERIAL PRIMARY KEY,
					group_id BIGINT NOT NULL,
					code_hash VARCHAR(64) NOT NULL UNIQUE,
					expires_at TIMESTAMPTZ NOT NULL,
					revoked_at TIMESTAMPTZ,
					consumed_at TIMESTAMPTZ,
					consumed_by_user_id BIGINT,
					created_by_user_id BIGINT NOT NULL,
					replaces_invite_id BIGINT,
					created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
				)`,
			}
			for _, statement := range statements {
				if err := db.Exec(statement).Error; err != nil {
					return err
				}
			}
			return nil
		},
		Down: func(db *gorm.DB) error { return nil },
	})

	registry.Register(Migration{
		Name:        "backfill_iteration3_group_memberships",
		Description: "Backfill current memberships without changing legacy users.group_id",
		Version:     "2.2.0",
		Definition:  "iteration3-group-memberships-backfill-v1",
		Up: func(db *gorm.DB) error {
			return db.Exec(`
				INSERT INTO group_memberships (user_id, group_id, joined_at, created_at, updated_at)
				SELECT users.id, users.group_id, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
				FROM users
				WHERE users.group_id IS NOT NULL
				  AND NOT EXISTS (SELECT 1 FROM group_memberships WHERE group_memberships.user_id = users.id)
			`).Error
		},
		Down: func(db *gorm.DB) error { return nil },
	})

	registry.Register(Migration{
		Name:        "contract_iteration3_group_indexes",
		Description: "Enforce deterministic and scoped membership and invite lookups",
		Version:     "2.2.0",
		Definition:  "iteration3-groups-contract-indexes-v1",
		Up: func(db *gorm.DB) error {
			statements := []string{
				`CREATE INDEX IF NOT EXISTS group_memberships_group_user_idx ON group_memberships (group_id, user_id)`,
				`CREATE INDEX IF NOT EXISTS group_invites_group_created_idx ON group_invites (group_id, created_at DESC, id DESC)`,
				`CREATE INDEX IF NOT EXISTS group_invites_availability_idx ON group_invites (expires_at, id)`,
				`CREATE INDEX IF NOT EXISTS users_points_rank_idx ON users (points DESC, id ASC)`,
			}
			for _, statement := range statements {
				if err := db.Exec(statement).Error; err != nil {
					return err
				}
			}
			return nil
		},
		Down: func(db *gorm.DB) error { return nil },
	})
}
