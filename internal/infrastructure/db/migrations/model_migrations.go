package migrations

import (
	"fmt"
	"os"
	"strings"
	"time"

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

func addConstraintIfMissing(db *gorm.DB, table, name, definition string) error {
	if db.Migrator().HasConstraint(table, name) {
		return nil
	}
	return db.Exec("ALTER TABLE " + table + " ADD CONSTRAINT " + name + " " + definition).Error
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

	registry.Register(Migration{
		Name:        "expand_iteration4_installation_activities",
		Description: "Create single-installation spaces, activities, assignments and privileged-operation audit storage",
		Version:     "2.3.0",
		Definition:  "iteration4-installation-activities-expand-v1",
		Up: func(db *gorm.DB) error {
			statements := []string{
				`CREATE TABLE IF NOT EXISTS spaces (
					id UUID PRIMARY KEY,
					slug VARCHAR(120),
					name VARCHAR(200),
					map_reference TEXT,
					created_at TIMESTAMPTZ,
					updated_at TIMESTAMPTZ
				)`,
				`ALTER TABLE spaces ADD COLUMN IF NOT EXISTS slug VARCHAR(120)`,
				`ALTER TABLE spaces ADD COLUMN IF NOT EXISTS name VARCHAR(200)`,
				`ALTER TABLE spaces ADD COLUMN IF NOT EXISTS map_reference TEXT`,
				`ALTER TABLE spaces ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ`,
				`ALTER TABLE spaces ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ`,
				`CREATE TABLE IF NOT EXISTS activities (
					id UUID PRIMARY KEY,
					space_id UUID,
					slug VARCHAR(120),
					name VARCHAR(200),
					description TEXT,
					kind VARCHAR(32),
					status VARCHAR(32),
					starts_at TIMESTAMPTZ,
					ends_at TIMESTAMPTZ,
					check_in_points INTEGER,
					moment_points INTEGER,
					cooldown_seconds INTEGER,
					allows_moment BOOLEAN,
					created_at TIMESTAMPTZ,
					updated_at TIMESTAMPTZ
				)`,
				`ALTER TABLE activities ADD COLUMN IF NOT EXISTS space_id UUID`,
				`ALTER TABLE activities ADD COLUMN IF NOT EXISTS slug VARCHAR(120)`,
				`ALTER TABLE activities ADD COLUMN IF NOT EXISTS name VARCHAR(200)`,
				`ALTER TABLE activities ADD COLUMN IF NOT EXISTS description TEXT`,
				`ALTER TABLE activities ADD COLUMN IF NOT EXISTS kind VARCHAR(32)`,
				`ALTER TABLE activities ADD COLUMN IF NOT EXISTS status VARCHAR(32)`,
				`ALTER TABLE activities ADD COLUMN IF NOT EXISTS starts_at TIMESTAMPTZ`,
				`ALTER TABLE activities ADD COLUMN IF NOT EXISTS ends_at TIMESTAMPTZ`,
				`ALTER TABLE activities ADD COLUMN IF NOT EXISTS check_in_points INTEGER`,
				`ALTER TABLE activities ADD COLUMN IF NOT EXISTS moment_points INTEGER`,
				`ALTER TABLE activities ADD COLUMN IF NOT EXISTS cooldown_seconds INTEGER`,
				`ALTER TABLE activities ADD COLUMN IF NOT EXISTS allows_moment BOOLEAN`,
				`ALTER TABLE activities ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ`,
				`ALTER TABLE activities ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ`,
				`CREATE TABLE IF NOT EXISTS activity_manager_assignments (
					activity_id UUID NOT NULL,
					user_id BIGINT NOT NULL,
					created_at TIMESTAMPTZ,
					PRIMARY KEY (activity_id, user_id)
				)`,
				`ALTER TABLE activity_manager_assignments ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ`,
				`CREATE TABLE IF NOT EXISTS operation_audit (
					id UUID PRIMARY KEY,
					actor_user_id BIGINT,
					action VARCHAR(120),
					entity_type VARCHAR(80),
					entity_id UUID,
					metadata JSONB,
					idempotency_key UUID,
					created_at TIMESTAMPTZ
				)`,
				`ALTER TABLE operation_audit ADD COLUMN IF NOT EXISTS actor_user_id BIGINT`,
				`ALTER TABLE operation_audit ADD COLUMN IF NOT EXISTS action VARCHAR(120)`,
				`ALTER TABLE operation_audit ADD COLUMN IF NOT EXISTS entity_type VARCHAR(80)`,
				`ALTER TABLE operation_audit ADD COLUMN IF NOT EXISTS entity_id UUID`,
				`ALTER TABLE operation_audit ADD COLUMN IF NOT EXISTS metadata JSONB`,
				`ALTER TABLE operation_audit ADD COLUMN IF NOT EXISTS idempotency_key UUID`,
				`ALTER TABLE operation_audit ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ`,
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
		Name:        "backfill_iteration4_installation_activities",
		Description: "Backfill safe defaults for single-installation activity configuration without deleting legacy data",
		Version:     "2.3.0",
		Definition:  "iteration4-installation-activities-backfill-v1",
		Up: func(db *gorm.DB) error {
			statements := []string{
				`UPDATE spaces SET slug = 'space-' || REPLACE(CAST(id AS VARCHAR), '-', '') WHERE slug IS NULL OR TRIM(slug) = ''`,
				`UPDATE spaces SET name = slug WHERE name IS NULL OR TRIM(name) = ''`,
				`UPDATE spaces SET created_at = CURRENT_TIMESTAMP WHERE created_at IS NULL`,
				`UPDATE spaces SET updated_at = created_at WHERE updated_at IS NULL`,
				`UPDATE activities SET slug = 'activity-' || REPLACE(CAST(id AS VARCHAR), '-', '') WHERE slug IS NULL OR TRIM(slug) = ''`,
				`UPDATE activities SET name = slug WHERE name IS NULL OR TRIM(name) = ''`,
				`UPDATE activities SET kind = 'live' WHERE kind IS NULL OR TRIM(kind) = ''`,
				`UPDATE activities SET status = 'draft' WHERE status IS NULL OR TRIM(status) = ''`,
				`UPDATE activities SET check_in_points = 0 WHERE check_in_points IS NULL`,
				`UPDATE activities SET moment_points = 0 WHERE moment_points IS NULL`,
				`UPDATE activities SET cooldown_seconds = 0 WHERE cooldown_seconds IS NULL`,
				`UPDATE activities SET allows_moment = FALSE WHERE allows_moment IS NULL`,
				`UPDATE activities SET created_at = CURRENT_TIMESTAMP WHERE created_at IS NULL`,
				`UPDATE activities SET updated_at = created_at WHERE updated_at IS NULL`,
				`UPDATE activity_manager_assignments SET created_at = CURRENT_TIMESTAMP WHERE created_at IS NULL`,
				`UPDATE operation_audit SET action = 'legacy.unknown' WHERE action IS NULL OR TRIM(action) = ''`,
				`UPDATE operation_audit SET entity_type = 'unknown' WHERE entity_type IS NULL OR TRIM(entity_type) = ''`,
				`UPDATE operation_audit SET metadata = CAST('{}' AS JSONB) WHERE metadata IS NULL`,
				`UPDATE operation_audit SET idempotency_key = id WHERE idempotency_key IS NULL`,
				`UPDATE operation_audit SET created_at = CURRENT_TIMESTAMP WHERE created_at IS NULL`,
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
		Name:        "contract_iteration4_installation_activities",
		Description: "Enforce activity types, permissions, deterministic lookups and operation idempotency",
		Version:     "2.3.0",
		Definition:  "iteration4-installation-activities-contract-v1",
		Up: func(db *gorm.DB) error {
			statements := []string{
				`ALTER TABLE spaces ALTER COLUMN slug SET NOT NULL`,
				`ALTER TABLE spaces ALTER COLUMN name SET NOT NULL`,
				`ALTER TABLE spaces ALTER COLUMN created_at SET NOT NULL`,
				`ALTER TABLE spaces ALTER COLUMN updated_at SET NOT NULL`,
				`ALTER TABLE activities ALTER COLUMN slug SET NOT NULL`,
				`ALTER TABLE activities ALTER COLUMN name SET NOT NULL`,
				`ALTER TABLE activities ALTER COLUMN kind SET NOT NULL`,
				`ALTER TABLE activities ALTER COLUMN status SET NOT NULL`,
				`ALTER TABLE activities ALTER COLUMN check_in_points SET NOT NULL`,
				`ALTER TABLE activities ALTER COLUMN moment_points SET NOT NULL`,
				`ALTER TABLE activities ALTER COLUMN cooldown_seconds SET NOT NULL`,
				`ALTER TABLE activities ALTER COLUMN allows_moment SET NOT NULL`,
				`ALTER TABLE activities ALTER COLUMN created_at SET NOT NULL`,
				`ALTER TABLE activities ALTER COLUMN updated_at SET NOT NULL`,
				`ALTER TABLE activity_manager_assignments ALTER COLUMN created_at SET NOT NULL`,
				`ALTER TABLE operation_audit ALTER COLUMN action SET NOT NULL`,
				`ALTER TABLE operation_audit ALTER COLUMN entity_type SET NOT NULL`,
				`ALTER TABLE operation_audit ALTER COLUMN metadata SET NOT NULL`,
				`ALTER TABLE operation_audit ALTER COLUMN idempotency_key SET NOT NULL`,
				`ALTER TABLE operation_audit ALTER COLUMN created_at SET NOT NULL`,
				`CREATE UNIQUE INDEX IF NOT EXISTS spaces_slug_unique ON spaces (slug)`,
				`CREATE UNIQUE INDEX IF NOT EXISTS activities_slug_unique ON activities (slug)`,
				`CREATE UNIQUE INDEX IF NOT EXISTS activity_manager_assignments_unique ON activity_manager_assignments (activity_id, user_id)`,
				`CREATE INDEX IF NOT EXISTS activities_schedule_idx ON activities (status, starts_at, id) WHERE kind = 'schedule'`,
				`CREATE INDEX IF NOT EXISTS activities_space_idx ON activities (space_id, id)`,
				`CREATE INDEX IF NOT EXISTS activity_manager_assignments_user_idx ON activity_manager_assignments (user_id, activity_id)`,
				`CREATE UNIQUE INDEX IF NOT EXISTS operation_audit_actor_idempotency_unique ON operation_audit (actor_user_id, idempotency_key)`,
				`CREATE INDEX IF NOT EXISTS operation_audit_entity_created_idx ON operation_audit (entity_type, entity_id, created_at, id)`,
			}
			for _, statement := range statements {
				if err := db.Exec(statement).Error; err != nil {
					return err
				}
			}

			constraints := []struct{ table, name, definition string }{
				{"spaces", "spaces_slug_format_check", `CHECK (slug ~ '^[a-z0-9]+(?:-[a-z0-9]+)*$')`},
				{"spaces", "spaces_name_not_blank_check", `CHECK (CHAR_LENGTH(TRIM(name)) > 0)`},
				{"activities", "activities_space_fk", `FOREIGN KEY (space_id) REFERENCES spaces(id) ON DELETE SET NULL`},
				{"activities", "activities_slug_format_check", `CHECK (slug ~ '^[a-z0-9]+(?:-[a-z0-9]+)*$')`},
				{"activities", "activities_name_not_blank_check", `CHECK (CHAR_LENGTH(TRIM(name)) > 0)`},
				{"activities", "activities_kind_check", `CHECK (kind IN ('schedule','checkpoint','challenge','competitive','live'))`},
				{"activities", "activities_status_check", `CHECK (status IN ('draft','active','paused','completed','archived'))`},
				{"activities", "activities_time_window_check", `CHECK (ends_at IS NULL OR starts_at IS NULL OR starts_at < ends_at)`},
				{"activities", "activities_points_check", `CHECK (check_in_points >= 0 AND moment_points >= 0 AND cooldown_seconds >= 0)`},
				{"activities", "activities_moment_eligibility_check", `CHECK (NOT allows_moment OR kind IN ('checkpoint','challenge','competitive','live'))`},
				{"activity_manager_assignments", "activity_manager_assignments_activity_fk", `FOREIGN KEY (activity_id) REFERENCES activities(id) ON DELETE CASCADE`},
				{"activity_manager_assignments", "activity_manager_assignments_user_fk", `FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE`},
				{"operation_audit", "operation_audit_actor_fk", `FOREIGN KEY (actor_user_id) REFERENCES users(id) ON DELETE SET NULL`},
			}
			for _, constraint := range constraints {
				if err := addConstraintIfMissing(db, constraint.table, constraint.name, constraint.definition); err != nil {
					return err
				}
			}
			return nil
		},
		Down: func(db *gorm.DB) error { return nil },
	})

	registry.Register(Migration{
		Name:        "expand_iteration4_admin_enabler",
		Description: "Add safe administrative idempotency storage and text entity references for audits",
		Version:     "2.3.1",
		Definition:  "iteration4-admin-enabler-expand-v1",
		Up: func(db *gorm.DB) error {
			statements := []string{
				`ALTER TABLE operation_audit ADD COLUMN IF NOT EXISTS entity_reference VARCHAR(160)`,
				`CREATE TABLE IF NOT EXISTS admin_operations (
					id UUID PRIMARY KEY,
					actor_user_id BIGINT NOT NULL,
					idempotency_key UUID NOT NULL,
					operation VARCHAR(120) NOT NULL,
					entity_type VARCHAR(80) NOT NULL,
					entity_ref VARCHAR(160) NOT NULL,
					request_hash VARCHAR(64) NOT NULL,
					http_status INTEGER NOT NULL,
					response JSONB NOT NULL,
					created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
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
		Name:        "backfill_iteration4_admin_enabler",
		Description: "Backfill audit entity references without copying request bodies or personal data",
		Version:     "2.3.1",
		Definition:  "iteration4-admin-enabler-backfill-v1",
		Up: func(db *gorm.DB) error {
			return db.Exec(`
				UPDATE operation_audit
				SET entity_reference = CAST(entity_id AS VARCHAR)
				WHERE entity_reference IS NULL AND entity_id IS NOT NULL
			`).Error
		},
		Down: func(db *gorm.DB) error { return nil },
	})

	registry.Register(Migration{
		Name:        "contract_iteration4_admin_enabler",
		Description: "Enforce administrative idempotency, deterministic listings and actor relationships",
		Version:     "2.3.1",
		Definition:  "iteration4-admin-enabler-contract-v1",
		Up: func(db *gorm.DB) error {
			statements := []string{
				`CREATE UNIQUE INDEX IF NOT EXISTS admin_operations_actor_idempotency_unique ON admin_operations (actor_user_id, idempotency_key)`,
				`CREATE INDEX IF NOT EXISTS admin_operations_entity_created_idx ON admin_operations (entity_type, entity_ref, created_at, id)`,
				`CREATE INDEX IF NOT EXISTS operation_audit_entity_reference_created_idx ON operation_audit (entity_type, entity_reference, created_at, id)`,
				`CREATE INDEX IF NOT EXISTS spaces_admin_list_idx ON spaces (name, id)`,
				`CREATE INDEX IF NOT EXISTS activities_admin_list_idx ON activities (name, id)`,
				`CREATE INDEX IF NOT EXISTS users_admin_role_list_idx ON users (role, name, id)`,
			}
			for _, statement := range statements {
				if err := db.Exec(statement).Error; err != nil {
					return err
				}
			}
			return addConstraintIfMissing(db, "admin_operations", "admin_operations_actor_fk", `FOREIGN KEY (actor_user_id) REFERENCES users(id) ON DELETE RESTRICT`)
		},
		Down: func(db *gorm.DB) error { return nil },
	})

	registry.Register(Migration{
		Name:        "expand_iteration5_agenda_content",
		Description: "Create participant favorites and body-free idempotency storage",
		Version:     "2.4.0",
		Definition:  "iteration5-agenda-content-expand-v1",
		Up: func(db *gorm.DB) error {
			statements := []string{
				`CREATE TABLE IF NOT EXISTS user_favorites (
					user_id BIGINT,
					activity_id UUID,
					created_at TIMESTAMPTZ
				)`,
				`ALTER TABLE user_favorites ADD COLUMN IF NOT EXISTS user_id BIGINT`,
				`ALTER TABLE user_favorites ADD COLUMN IF NOT EXISTS activity_id UUID`,
				`ALTER TABLE user_favorites ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ`,
				`CREATE TABLE IF NOT EXISTS participant_operations (
					id UUID PRIMARY KEY,
					actor_user_id BIGINT,
					idempotency_key UUID,
					operation VARCHAR(80),
					activity_id UUID,
					intent_hash VARCHAR(64),
					http_status INTEGER,
					created_at TIMESTAMPTZ
				)`,
				`ALTER TABLE participant_operations ADD COLUMN IF NOT EXISTS actor_user_id BIGINT`,
				`ALTER TABLE participant_operations ADD COLUMN IF NOT EXISTS idempotency_key UUID`,
				`ALTER TABLE participant_operations ADD COLUMN IF NOT EXISTS operation VARCHAR(80)`,
				`ALTER TABLE participant_operations ADD COLUMN IF NOT EXISTS activity_id UUID`,
				`ALTER TABLE participant_operations ADD COLUMN IF NOT EXISTS intent_hash VARCHAR(64)`,
				`ALTER TABLE participant_operations ADD COLUMN IF NOT EXISTS http_status INTEGER`,
				`ALTER TABLE participant_operations ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ`,
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
		Name:        "backfill_iteration5_agenda_content",
		Description: "Backfill safe timestamps for pre-existing participant rows",
		Version:     "2.4.0",
		Definition:  "iteration5-agenda-content-backfill-v1",
		Up: func(db *gorm.DB) error {
			statements := []string{
				`UPDATE user_favorites SET created_at = CURRENT_TIMESTAMP WHERE created_at IS NULL`,
				`UPDATE participant_operations SET created_at = CURRENT_TIMESTAMP WHERE created_at IS NULL`,
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
		Name:        "contract_iteration5_agenda_content",
		Description: "Enforce participant idempotency, visibility lookup and favorite uniqueness",
		Version:     "2.4.0",
		Definition:  "iteration5-agenda-content-contract-v1",
		Up: func(db *gorm.DB) error {
			statements := []string{
				`ALTER TABLE user_favorites ALTER COLUMN user_id SET NOT NULL`,
				`ALTER TABLE user_favorites ALTER COLUMN activity_id SET NOT NULL`,
				`ALTER TABLE user_favorites ALTER COLUMN created_at SET NOT NULL`,
				`ALTER TABLE participant_operations ALTER COLUMN actor_user_id SET NOT NULL`,
				`ALTER TABLE participant_operations ALTER COLUMN idempotency_key SET NOT NULL`,
				`ALTER TABLE participant_operations ALTER COLUMN operation SET NOT NULL`,
				`ALTER TABLE participant_operations ALTER COLUMN activity_id SET NOT NULL`,
				`ALTER TABLE participant_operations ALTER COLUMN intent_hash SET NOT NULL`,
				`ALTER TABLE participant_operations ALTER COLUMN http_status SET NOT NULL`,
				`ALTER TABLE participant_operations ALTER COLUMN created_at SET NOT NULL`,
				`CREATE UNIQUE INDEX IF NOT EXISTS user_favorites_user_activity_unique ON user_favorites (user_id, activity_id)`,
				`CREATE INDEX IF NOT EXISTS user_favorites_activity_user_idx ON user_favorites (activity_id, user_id)`,
				`CREATE UNIQUE INDEX IF NOT EXISTS participant_operations_actor_key_unique ON participant_operations (actor_user_id, idempotency_key)`,
				`CREATE INDEX IF NOT EXISTS participant_operations_activity_created_idx ON participant_operations (activity_id, created_at, id)`,
				`CREATE INDEX IF NOT EXISTS activities_public_list_idx ON activities (starts_at, name, id)`,
				`CREATE INDEX IF NOT EXISTS activities_public_kind_space_idx ON activities (kind, space_id, starts_at, name, id)`,
			}
			for _, statement := range statements {
				if err := db.Exec(statement).Error; err != nil {
					return err
				}
			}
			constraints := []struct{ table, name, definition string }{
				{"user_favorites", "user_favorites_user_fk", `FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE RESTRICT`},
				{"user_favorites", "user_favorites_activity_fk", `FOREIGN KEY (activity_id) REFERENCES activities(id) ON DELETE RESTRICT`},
				{"participant_operations", "participant_operations_actor_fk", `FOREIGN KEY (actor_user_id) REFERENCES users(id) ON DELETE RESTRICT`},
				{"participant_operations", "participant_operations_status_check", `CHECK (http_status = 204)`},
			}
			for _, constraint := range constraints {
				if err := addConstraintIfMissing(db, constraint.table, constraint.name, constraint.definition); err != nil {
					return err
				}
			}
			return nil
		},
		Down: func(db *gorm.DB) error { return nil },
	})

	registry.Register(Migration{
		Name:        "expand_iteration6_games_runs_scoring",
		Description: "Create competitive activity runs, participants, QR, participation, point ledger and manager idempotency storage",
		Version:     "2.5.0",
		Definition:  "iteration6-games-runs-scoring-expand-v2",
		Up: func(db *gorm.DB) error {
			statements := []string{
				`CREATE TABLE IF NOT EXISTS activity_runs (
					id UUID PRIMARY KEY,
					activity_id UUID NOT NULL,
					started_by BIGINT NOT NULL,
					status VARCHAR(24) NOT NULL,
					point_rules JSONB NOT NULL,
					started_at TIMESTAMPTZ,
					ended_at TIMESTAMPTZ,
					created_at TIMESTAMPTZ NOT NULL,
					updated_at TIMESTAMPTZ NOT NULL
				)`,
				`CREATE TABLE IF NOT EXISTS activity_run_qr_codes (
					id UUID PRIMARY KEY,
					activity_id UUID NOT NULL,
					activity_run_id UUID NOT NULL,
					token_hash VARCHAR(64) NOT NULL,
					expires_at TIMESTAMPTZ NOT NULL,
					status VARCHAR(24) NOT NULL,
					created_at TIMESTAMPTZ NOT NULL,
					updated_at TIMESTAMPTZ NOT NULL
				)`,
				`CREATE TABLE IF NOT EXISTS participations (
					id UUID PRIMARY KEY,
					user_id BIGINT NOT NULL,
					activity_id UUID NOT NULL,
					activity_run_id UUID NOT NULL,
					qr_code_id UUID NOT NULL,
					checked_in_at TIMESTAMPTZ NOT NULL,
					status VARCHAR(24) NOT NULL,
					can_share_moment BOOLEAN NOT NULL DEFAULT FALSE,
					check_in_points INTEGER NOT NULL DEFAULT 0,
					created_at TIMESTAMPTZ NOT NULL
				)`,
				`CREATE TABLE IF NOT EXISTS activity_run_participants (
					id UUID PRIMARY KEY,
					activity_run_id UUID NOT NULL,
					user_id BIGINT NOT NULL,
					participation_id UUID NOT NULL,
					checked_in_at TIMESTAMPTZ NOT NULL,
					result VARCHAR(24),
					points_awarded INTEGER NOT NULL DEFAULT 0,
					created_at TIMESTAMPTZ NOT NULL
				)`,
				`CREATE TABLE IF NOT EXISTS point_entries (
					id UUID PRIMARY KEY,
					user_id BIGINT NOT NULL,
					activity_id UUID,
					activity_run_id UUID,
					participation_id UUID,
					origin VARCHAR(64) NOT NULL,
					reason VARCHAR(80) NOT NULL,
					delta INTEGER NOT NULL,
					created_at TIMESTAMPTZ NOT NULL
				)`,
				`CREATE TABLE IF NOT EXISTS manager_operations (
					id UUID PRIMARY KEY,
					actor_user_id BIGINT NOT NULL,
					idempotency_key UUID NOT NULL,
					operation VARCHAR(120) NOT NULL,
					activity_id UUID NOT NULL,
					activity_run_id UUID,
					intent_hash VARCHAR(64) NOT NULL,
					result_ref UUID,
					result_status VARCHAR(24),
					result_started_at TIMESTAMPTZ,
					result_ended_at TIMESTAMPTZ,
					result_expires_at TIMESTAMPTZ,
					http_status INTEGER NOT NULL,
					created_at TIMESTAMPTZ NOT NULL
				)`,
				`ALTER TABLE participant_operations ADD COLUMN IF NOT EXISTS result_ref UUID`,
				`ALTER TABLE participant_operations ADD COLUMN IF NOT EXISTS result_points INTEGER`,
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
		Name:        "backfill_iteration6_games_runs_scoring",
		Description: "Normalize safe defaults for any pre-contract iteration 6 rows without inventing runs or points",
		Version:     "2.5.0",
		Definition:  "iteration6-games-runs-scoring-backfill-v1",
		Up: func(db *gorm.DB) error {
			statements := []string{
				`UPDATE activity_runs SET updated_at = created_at WHERE updated_at IS NULL`,
				`UPDATE activity_run_qr_codes SET updated_at = created_at WHERE updated_at IS NULL`,
				`UPDATE activity_run_participants SET points_awarded = 0 WHERE points_awarded IS NULL`,
				`UPDATE participations SET can_share_moment = FALSE WHERE can_share_moment IS NULL`,
				`UPDATE participations SET check_in_points = 0 WHERE check_in_points IS NULL`,
				`INSERT INTO point_entries (id, user_id, activity_id, activity_run_id, participation_id, origin, reason, delta, created_at)
				 SELECT gen_random_uuid(), users.id, NULL, NULL, NULL, 'legacy_balance', 'legacy_balance', users.points, users.created_at
				 FROM users
				 WHERE users.points > 0
				   AND NOT EXISTS (
				     SELECT 1 FROM point_entries
				     WHERE point_entries.user_id = users.id AND point_entries.origin = 'legacy_balance'
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
		Name:        "contract_iteration6_games_runs_scoring",
		Description: "Enforce run state, immutable-ledger relationships, deterministic ranking and idempotent competitive operations",
		Version:     "2.5.0",
		Definition:  "iteration6-games-runs-scoring-contract-v1",
		Up: func(db *gorm.DB) error {
			if db.Migrator().HasConstraint("participant_operations", "participant_operations_status_check") {
				if err := db.Migrator().DropConstraint("participant_operations", "participant_operations_status_check"); err != nil {
					return err
				}
			}
			statements := []string{
				`CREATE UNIQUE INDEX IF NOT EXISTS activity_runs_one_open_per_activity ON activity_runs (activity_id) WHERE status IN ('draft','active','paused','results')`,
				`CREATE INDEX IF NOT EXISTS activity_runs_manager_open_idx ON activity_runs (started_by, status, created_at DESC, id DESC)`,
				`CREATE INDEX IF NOT EXISTS activity_runs_activity_created_idx ON activity_runs (activity_id, created_at DESC, id DESC)`,
				`CREATE UNIQUE INDEX IF NOT EXISTS activity_run_qr_token_hash_unique ON activity_run_qr_codes (token_hash)`,
				`CREATE UNIQUE INDEX IF NOT EXISTS activity_run_qr_one_active_per_run ON activity_run_qr_codes (activity_run_id) WHERE status = 'active'`,
				`CREATE INDEX IF NOT EXISTS activity_run_qr_expiry_idx ON activity_run_qr_codes (expires_at, id)`,
				`CREATE UNIQUE INDEX IF NOT EXISTS participations_run_user_unique ON participations (activity_run_id, user_id)`,
				`CREATE INDEX IF NOT EXISTS participations_user_current_idx ON participations (user_id, checked_in_at DESC, id DESC)`,
				`CREATE UNIQUE INDEX IF NOT EXISTS activity_run_participants_run_user_unique ON activity_run_participants (activity_run_id, user_id)`,
				`CREATE UNIQUE INDEX IF NOT EXISTS activity_run_participants_participation_unique ON activity_run_participants (participation_id)`,
				`CREATE INDEX IF NOT EXISTS activity_run_participants_order_idx ON activity_run_participants (activity_run_id, checked_in_at, user_id)`,
				`CREATE UNIQUE INDEX IF NOT EXISTS point_entries_run_user_reason_unique ON point_entries (activity_run_id, user_id, reason) WHERE activity_run_id IS NOT NULL`,
				`CREATE UNIQUE INDEX IF NOT EXISTS point_entries_legacy_user_unique ON point_entries (user_id) WHERE origin = 'legacy_balance'`,
				`CREATE INDEX IF NOT EXISTS point_entries_user_history_idx ON point_entries (user_id, created_at DESC, id DESC)`,
				`CREATE UNIQUE INDEX IF NOT EXISTS manager_operations_actor_idempotency_unique ON manager_operations (actor_user_id, idempotency_key)`,
				`CREATE INDEX IF NOT EXISTS manager_operations_run_created_idx ON manager_operations (activity_run_id, created_at, id)`,
				`CREATE INDEX IF NOT EXISTS users_iteration6_ranking_idx ON users (points DESC, name ASC, id ASC) WHERE onboarding_complete = TRUE AND role = 'DEFAULT'`,
			}
			for _, statement := range statements {
				if err := db.Exec(statement).Error; err != nil {
					return err
				}
			}
			triggerStatements := []string{
				`CREATE OR REPLACE FUNCTION prevent_point_entries_mutation()
				 RETURNS TRIGGER AS $$
				 BEGIN
				   RAISE EXCEPTION 'point_entries is append-only';
				 END;
				 $$ LANGUAGE PLpgSQL`,
				`DROP TRIGGER IF EXISTS point_entries_append_only ON point_entries`,
				`CREATE TRIGGER point_entries_append_only
				 BEFORE UPDATE OR DELETE ON point_entries
				 FOR EACH ROW EXECUTE FUNCTION prevent_point_entries_mutation()`,
			}
			for _, statement := range triggerStatements {
				if err := db.Exec(statement).Error; err != nil {
					return err
				}
			}
			constraints := []struct{ table, name, definition string }{
				{"users", "users_points_nonnegative_check", `CHECK (points >= 0)`},
				{"activity_runs", "activity_runs_activity_fk", `FOREIGN KEY (activity_id) REFERENCES activities(id) ON DELETE RESTRICT`},
				{"activity_runs", "activity_runs_started_by_fk", `FOREIGN KEY (started_by) REFERENCES users(id) ON DELETE RESTRICT`},
				{"activity_runs", "activity_runs_status_check", `CHECK (status IN ('draft','active','paused','results','completed','cancelled'))`},
				{"activity_runs", "activity_runs_terminal_time_check", `CHECK ((status IN ('completed','cancelled') AND ended_at IS NOT NULL) OR (status NOT IN ('completed','cancelled') AND ended_at IS NULL))`},
				{"activity_run_qr_codes", "activity_run_qr_activity_fk", `FOREIGN KEY (activity_id) REFERENCES activities(id) ON DELETE RESTRICT`},
				{"activity_run_qr_codes", "activity_run_qr_run_fk", `FOREIGN KEY (activity_run_id) REFERENCES activity_runs(id) ON DELETE RESTRICT`},
				{"activity_run_qr_codes", "activity_run_qr_status_check", `CHECK (status IN ('active','disabled'))`},
				{"participations", "participations_user_fk", `FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE RESTRICT`},
				{"participations", "participations_activity_fk", `FOREIGN KEY (activity_id) REFERENCES activities(id) ON DELETE RESTRICT`},
				{"participations", "participations_run_fk", `FOREIGN KEY (activity_run_id) REFERENCES activity_runs(id) ON DELETE RESTRICT`},
				{"participations", "participations_qr_fk", `FOREIGN KEY (qr_code_id) REFERENCES activity_run_qr_codes(id) ON DELETE RESTRICT`},
				{"participations", "participations_status_check", `CHECK (status IN ('active','completed','cancelled'))`},
				{"participations", "participations_no_scan_points_check", `CHECK (check_in_points = 0)`},
				{"activity_run_participants", "activity_run_participants_run_fk", `FOREIGN KEY (activity_run_id) REFERENCES activity_runs(id) ON DELETE RESTRICT`},
				{"activity_run_participants", "activity_run_participants_user_fk", `FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE RESTRICT`},
				{"activity_run_participants", "activity_run_participants_participation_fk", `FOREIGN KEY (participation_id) REFERENCES participations(id) ON DELETE RESTRICT`},
				{"activity_run_participants", "activity_run_participants_result_check", `CHECK (result IS NULL OR result IN ('first','second','third','participation'))`},
				{"activity_run_participants", "activity_run_participants_points_check", `CHECK (points_awarded >= 0)`},
				{"point_entries", "point_entries_user_fk", `FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE RESTRICT`},
				{"point_entries", "point_entries_activity_fk", `FOREIGN KEY (activity_id) REFERENCES activities(id) ON DELETE RESTRICT`},
				{"point_entries", "point_entries_run_fk", `FOREIGN KEY (activity_run_id) REFERENCES activity_runs(id) ON DELETE RESTRICT`},
				{"point_entries", "point_entries_participation_fk", `FOREIGN KEY (participation_id) REFERENCES participations(id) ON DELETE RESTRICT`},
				{"point_entries", "point_entries_origin_check", `CHECK (
					(origin = 'activity_run_results' AND activity_id IS NOT NULL AND activity_run_id IS NOT NULL AND participation_id IS NOT NULL)
					OR (origin = 'legacy_balance' AND activity_id IS NULL AND activity_run_id IS NULL AND participation_id IS NULL)
				)`},
				{"point_entries", "point_entries_delta_check", `CHECK (delta <> 0)`},
				{"manager_operations", "manager_operations_actor_fk", `FOREIGN KEY (actor_user_id) REFERENCES users(id) ON DELETE RESTRICT`},
				{"manager_operations", "manager_operations_activity_fk", `FOREIGN KEY (activity_id) REFERENCES activities(id) ON DELETE RESTRICT`},
				{"manager_operations", "manager_operations_run_fk", `FOREIGN KEY (activity_run_id) REFERENCES activity_runs(id) ON DELETE RESTRICT`},
				{"manager_operations", "manager_operations_http_status_check", `CHECK (http_status IN (200,201))`},
				{"participant_operations", "participant_operations_status_check", `CHECK (http_status IN (200,201,204))`},
			}
			for _, constraint := range constraints {
				if err := addConstraintIfMissing(db, constraint.table, constraint.name, constraint.definition); err != nil {
					return err
				}
			}
			return nil
		},
		Down: func(db *gorm.DB) error { return nil },
	})

	registry.Register(Migration{
		Name:        "expand_media_moments_v2",
		Description: "Create private media lifecycle, Moments, durable idempotency, processing claims and cleanup jobs",
		Version:     "2.6.0",
		Definition:  "media-moments-v2-expand-v2",
		Up: func(db *gorm.DB) error {
			statements := []string{
				`ALTER TABLE users ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ`,
				`CREATE TABLE IF NOT EXISTS media_assets (
					id UUID PRIMARY KEY, owner_user_id BIGINT NOT NULL, provider VARCHAR(24) NOT NULL,
					bucket TEXT NOT NULL, staging_object_key TEXT NOT NULL, staging_version_id TEXT,
					final_object_key TEXT NOT NULL, final_version_id TEXT, content_type VARCHAR(64) NOT NULL,
					bytes BIGINT NOT NULL, checksum_sha256 VARCHAR(44) NOT NULL, state VARCHAR(32) NOT NULL,
					upload_expires_at TIMESTAMPTZ NOT NULL, retention_due_at TIMESTAMPTZ NOT NULL,
					available_at TIMESTAMPTZ, failed_at TIMESTAMPTZ, deleted_at TIMESTAMPTZ,
					created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL
				)`,
				`CREATE TABLE IF NOT EXISTS media_processing_claims (
					media_asset_id UUID PRIMARY KEY, claim_token UUID NOT NULL, operation_key UUID NOT NULL,
					stage VARCHAR(32) NOT NULL, staging_version_id TEXT, final_version_id TEXT,
					lease_expires_at TIMESTAMPTZ NOT NULL, attempt_count INTEGER NOT NULL DEFAULT 0,
					last_error_category VARCHAR(64), completed_at TIMESTAMPTZ,
					created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL
				)`,
				`CREATE TABLE IF NOT EXISTS idempotency_operations (
					id UUID PRIMARY KEY, actor_user_id BIGINT NOT NULL, idempotency_key UUID NOT NULL,
					operation VARCHAR(120) NOT NULL, resource_ref VARCHAR(120), intent_hash VARCHAR(64) NOT NULL,
					state VARCHAR(24) NOT NULL, result_ref VARCHAR(120), result_boolean BOOLEAN, result_count INTEGER,
					response_snapshot JSONB NOT NULL DEFAULT '{}'::JSONB, http_status INTEGER NOT NULL DEFAULT 0,
					created_at TIMESTAMPTZ NOT NULL, completed_at TIMESTAMPTZ
				)`,
				`CREATE TABLE IF NOT EXISTS media_cleanup_jobs (
					id UUID PRIMARY KEY, media_asset_id UUID NOT NULL, kind VARCHAR(40) NOT NULL,
					state VARCHAR(24) NOT NULL, due_at TIMESTAMPTZ NOT NULL, attempt_count INTEGER NOT NULL DEFAULT 0,
					max_attempts INTEGER NOT NULL, next_attempt_at TIMESTAMPTZ NOT NULL, claim_token UUID,
					lease_expires_at TIMESTAMPTZ, last_error_code VARCHAR(64), completed_at TIMESTAMPTZ,
					created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL
				)`,
				`CREATE TABLE IF NOT EXISTS moments (
					id UUID PRIMARY KEY, user_id BIGINT NOT NULL, participation_id UUID, activity_id UUID,
					media_asset_id UUID NOT NULL, origin VARCHAR(24) NOT NULL, publication_status VARCHAR(24) NOT NULL,
					moderation_status VARCHAR(24) NOT NULL, reward_status VARCHAR(24) NOT NULL,
					points_awarded INTEGER NOT NULL DEFAULT 0, captured_at TIMESTAMPTZ NOT NULL,
					created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL
				)`,
				`CREATE TABLE IF NOT EXISTS moment_likes (
					moment_id UUID NOT NULL, user_id BIGINT NOT NULL, created_at TIMESTAMPTZ NOT NULL,
					PRIMARY KEY (moment_id, user_id)
				)`,
				`CREATE TABLE IF NOT EXISTS moment_moderation_decisions (
					id UUID PRIMARY KEY, moment_id UUID NOT NULL, actor_user_id BIGINT NOT NULL,
					action VARCHAR(32) NOT NULL, idempotency_key UUID NOT NULL, created_at TIMESTAMPTZ NOT NULL
				)`,
				`ALTER TABLE point_entries ADD COLUMN IF NOT EXISTS moment_id UUID`,
				`CREATE UNIQUE INDEX IF NOT EXISTS idempotency_operations_actor_key_unique ON idempotency_operations (actor_user_id, idempotency_key)`,
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
		Name:        "backfill_global_idempotency_registry",
		Description: "Register prior participant, manager and admin keys without copying bodies or personal data",
		Version:     "2.6.0",
		Definition:  "global-idempotency-registry-backfill-v1",
		Up: func(db *gorm.DB) error {
			statements := []string{
				`INSERT INTO idempotency_operations (id, actor_user_id, idempotency_key, operation, resource_ref, intent_hash, state, result_ref, response_snapshot, http_status, created_at, completed_at)
				 SELECT id, actor_user_id, idempotency_key, operation, activity_id, intent_hash, 'completed', result_ref, '{}'::JSONB, http_status, created_at, created_at FROM participant_operations ON CONFLICT (actor_user_id, idempotency_key) DO NOTHING`,
				`INSERT INTO idempotency_operations (id, actor_user_id, idempotency_key, operation, resource_ref, intent_hash, state, result_ref, response_snapshot, http_status, created_at, completed_at)
				 SELECT id, actor_user_id, idempotency_key, operation, activity_id, intent_hash, 'completed', result_ref, '{}'::JSONB, http_status, created_at, created_at FROM manager_operations ON CONFLICT (actor_user_id, idempotency_key) DO NOTHING`,
				`INSERT INTO idempotency_operations (id, actor_user_id, idempotency_key, operation, resource_ref, intent_hash, state, result_ref, response_snapshot, http_status, created_at, completed_at)
				 SELECT id, actor_user_id, idempotency_key, operation, NULL, request_hash, 'completed', NULL, '{}'::JSONB, http_status, created_at, created_at FROM admin_operations ON CONFLICT (actor_user_id, idempotency_key) DO NOTHING`,
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
		Name:        "contract_media_moments_v2",
		Description: "Enforce ownership, immutable history, one asset/participation per Moment, media leases and Moment ledger rules",
		Version:     "2.6.0",
		Definition:  "media-moments-v2-contract-v2",
		Up: func(db *gorm.DB) error {
			if db.Migrator().HasConstraint("point_entries", "point_entries_origin_check") {
				if err := db.Migrator().DropConstraint("point_entries", "point_entries_origin_check"); err != nil {
					return err
				}
			}
			statements := []string{
				`CREATE INDEX IF NOT EXISTS users_deleted_at_idx ON users (deleted_at)`,
				`CREATE UNIQUE INDEX IF NOT EXISTS media_assets_staging_key_unique ON media_assets (staging_object_key)`,
				`CREATE UNIQUE INDEX IF NOT EXISTS media_assets_final_key_unique ON media_assets (final_object_key)`,
				`CREATE INDEX IF NOT EXISTS media_assets_expiration_idx ON media_assets (state, upload_expires_at, id)`,
				`CREATE INDEX IF NOT EXISTS media_assets_retention_idx ON media_assets (state, retention_due_at, id)`,
				`CREATE UNIQUE INDEX IF NOT EXISTS idempotency_operations_actor_key_unique ON idempotency_operations (actor_user_id, idempotency_key)`,
				`CREATE INDEX IF NOT EXISTS idempotency_operations_processing_idx ON idempotency_operations (state, created_at, id)`,
				`CREATE UNIQUE INDEX IF NOT EXISTS media_cleanup_jobs_asset_kind_unique ON media_cleanup_jobs (media_asset_id, kind)`,
				`CREATE INDEX IF NOT EXISTS media_cleanup_jobs_claim_idx ON media_cleanup_jobs (state, next_attempt_at, due_at, id)`,
				`CREATE UNIQUE INDEX IF NOT EXISTS moments_media_asset_unique ON moments (media_asset_id)`,
				`CREATE UNIQUE INDEX IF NOT EXISTS moments_participation_unique ON moments (participation_id) WHERE participation_id IS NOT NULL`,
				`CREATE INDEX IF NOT EXISTS moments_feed_idx ON moments (captured_at DESC, id DESC) WHERE publication_status = 'public' AND moderation_status = 'approved'`,
				`CREATE INDEX IF NOT EXISTS moments_user_idx ON moments (user_id, captured_at DESC, id DESC)`,
				`CREATE INDEX IF NOT EXISTS moment_likes_user_idx ON moment_likes (user_id, moment_id)`,
				`CREATE UNIQUE INDEX IF NOT EXISTS moment_moderation_decisions_actor_key_unique ON moment_moderation_decisions (actor_user_id, idempotency_key)`,
				`CREATE UNIQUE INDEX IF NOT EXISTS point_entries_moment_user_reason_unique ON point_entries (moment_id, user_id, reason) WHERE moment_id IS NOT NULL`,
				`CREATE INDEX IF NOT EXISTS point_entries_moment_idx ON point_entries (moment_id, created_at, id) WHERE moment_id IS NOT NULL`,
			}
			for _, statement := range statements {
				if err := db.Exec(statement).Error; err != nil {
					return err
				}
			}
			constraints := []struct{ table, name, definition string }{
				{"media_assets", "media_assets_owner_fk", `FOREIGN KEY (owner_user_id) REFERENCES users(id) ON DELETE RESTRICT`},
				{"media_assets", "media_assets_provider_check", `CHECK (provider = 's3')`},
				{"media_assets", "media_assets_content_check", `CHECK (content_type IN ('image/jpeg','image/png') AND bytes > 0 AND bytes <= 10485760)`},
				{"media_assets", "media_assets_state_check", `CHECK (state IN ('pending_upload','processing','available','failed','deleted'))`},
				{"media_processing_claims", "media_processing_claim_asset_fk", `FOREIGN KEY (media_asset_id) REFERENCES media_assets(id) ON DELETE RESTRICT`},
				{"idempotency_operations", "idempotency_operations_actor_fk", `FOREIGN KEY (actor_user_id) REFERENCES users(id) ON DELETE RESTRICT`},
				{"idempotency_operations", "idempotency_operations_state_check", `CHECK (state IN ('processing','completed'))`},
				{"media_cleanup_jobs", "media_cleanup_jobs_asset_fk", `FOREIGN KEY (media_asset_id) REFERENCES media_assets(id) ON DELETE RESTRICT`},
				{"media_cleanup_jobs", "media_cleanup_jobs_state_check", `CHECK (state IN ('pending','processing','retry','failed','completed'))`},
				{"moments", "moments_user_fk", `FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE RESTRICT`},
				{"moments", "moments_participation_fk", `FOREIGN KEY (participation_id) REFERENCES participations(id) ON DELETE RESTRICT`},
				{"moments", "moments_activity_fk", `FOREIGN KEY (activity_id) REFERENCES activities(id) ON DELETE RESTRICT`},
				{"moments", "moments_media_asset_fk", `FOREIGN KEY (media_asset_id) REFERENCES media_assets(id) ON DELETE RESTRICT`},
				{"moments", "moments_origin_status_check", `CHECK ((origin = 'free' AND participation_id IS NULL AND activity_id IS NULL AND points_awarded = 0 AND reward_status = 'not_applicable') OR (origin = 'challenge' AND participation_id IS NOT NULL AND activity_id IS NOT NULL AND points_awarded >= 0 AND reward_status IN ('awarded','denied','reversed')))`},
				{"moments", "moments_publication_check", `CHECK (publication_status IN ('private','public'))`},
				{"moments", "moments_moderation_check", `CHECK (moderation_status IN ('approved','rejected'))`},
				{"moment_likes", "moment_likes_moment_fk", `FOREIGN KEY (moment_id) REFERENCES moments(id) ON DELETE RESTRICT`},
				{"moment_likes", "moment_likes_user_fk", `FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE RESTRICT`},
				{"moment_moderation_decisions", "moment_moderation_moment_fk", `FOREIGN KEY (moment_id) REFERENCES moments(id) ON DELETE RESTRICT`},
				{"moment_moderation_decisions", "moment_moderation_actor_fk", `FOREIGN KEY (actor_user_id) REFERENCES users(id) ON DELETE RESTRICT`},
				{"moment_moderation_decisions", "moment_moderation_action_check", `CHECK (action IN ('deny_points','delete_photo'))`},
				{"point_entries", "point_entries_moment_fk", `FOREIGN KEY (moment_id) REFERENCES moments(id) ON DELETE RESTRICT`},
				{"point_entries", "point_entries_origin_check", `CHECK ((origin = 'activity_run_results' AND activity_id IS NOT NULL AND activity_run_id IS NOT NULL AND participation_id IS NOT NULL AND moment_id IS NULL) OR (origin = 'legacy_balance' AND activity_id IS NULL AND activity_run_id IS NULL AND participation_id IS NULL AND moment_id IS NULL) OR (origin = 'moment' AND activity_id IS NOT NULL AND activity_run_id IS NULL AND participation_id IS NOT NULL AND moment_id IS NOT NULL))`},
			}
			for _, constraint := range constraints {
				if err := addConstraintIfMissing(db, constraint.table, constraint.name, constraint.definition); err != nil {
					return err
				}
			}
			return nil
		},
		Down: func(db *gorm.DB) error { return nil },
	})

	registry.Register(Migration{
		Name:        "create_notifications_v1",
		Description: "Persist notification preferences and server-derived notifications for Iteration 8",
		Version:     "2.7.0",
		Definition:  "notifications-v1",
		Up: func(db *gorm.DB) error {
			statements := []string{
				`CREATE TABLE IF NOT EXISTS notification_preferences (
					user_id BIGINT PRIMARY KEY, points_enabled BOOLEAN NOT NULL DEFAULT true,
					announcement_enabled BOOLEAN NOT NULL DEFAULT true, updated_at TIMESTAMPTZ NOT NULL
				)`,
				`CREATE TABLE IF NOT EXISTS notifications (
					id UUID PRIMARY KEY, user_id BIGINT NOT NULL, category VARCHAR(32) NOT NULL,
					state VARCHAR(16) NOT NULL DEFAULT 'unread', title VARCHAR(200) NOT NULL, body TEXT NOT NULL,
					source_type VARCHAR(32) NOT NULL, source_id VARCHAR(64), metadata JSONB,
					created_at TIMESTAMPTZ NOT NULL, read_at TIMESTAMPTZ
				)`,
				`CREATE INDEX IF NOT EXISTS notifications_user_feed_idx ON notifications (user_id, created_at DESC, id DESC)`,
				`CREATE INDEX IF NOT EXISTS notifications_user_unread_idx ON notifications (user_id, state)`,
			}
			for _, statement := range statements {
				if err := db.Exec(statement).Error; err != nil {
					return err
				}
			}
			constraints := []struct{ table, name, definition string }{
				{"notification_preferences", "notification_preferences_user_fk", `FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE RESTRICT`},
				{"notifications", "notifications_user_fk", `FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE RESTRICT`},
				{"notifications", "notifications_category_check", `CHECK (category IN ('moment_moderation','points','announcement'))`},
				{"notifications", "notifications_state_check", `CHECK (state IN ('unread','read'))`},
			}
			for _, constraint := range constraints {
				if err := addConstraintIfMissing(db, constraint.table, constraint.name, constraint.definition); err != nil {
					return err
				}
			}
			return nil
		},
		Down: func(db *gorm.DB) error { return nil },
	})

	registry.Register(Migration{
		Name: "seed_initial_admin_users",
		Description: "One-time bootstrap: promote an operator-provided list of emails " +
			"(ADMIN_SEED_EMAILS) to ADMIN, creating the user row if it doesn't exist yet. Like " +
			"every migration, the tracker (schema_migrations) runs this exactly once per " +
			"database — ADMIN_SEED_EMAILS must be set on the deploy that first ships this " +
			"migration, or it applies as a no-op and can never retry. After bootstrap, grant " +
			"further ADMIN/EVENT_MANAGER roles through PATCH /v2/admin/users/:userId/role, not " +
			"by editing this list — later runs never touch it again.",
		Version:    "2.8.0",
		Definition: "seed-initial-admin-users-v1",
		Up: func(db *gorm.DB) error {
			raw := strings.TrimSpace(os.Getenv("ADMIN_SEED_EMAILS"))
			if raw == "" {
				return nil
			}
			now := time.Now().UTC()
			for entry := range strings.SplitSeq(raw, ",") {
				email := strings.ToLower(strings.TrimSpace(entry))
				if email == "" {
					continue
				}
				name := email
				if at := strings.Index(email, "@"); at > 0 {
					name = email[:at]
				}
				err := db.Exec(`
					INSERT INTO users (email, name, role, onboarding_complete, points, created_at, updated_at)
					VALUES (?, ?, 'ADMIN', true, 0, ?, ?)
					ON CONFLICT (email) DO UPDATE SET role = 'ADMIN'
					WHERE users.role <> 'ADMIN'
				`, email, name, now, now).Error
				if err != nil {
					return fmt.Errorf("failed to seed admin user %s: %w", email, err)
				}
			}
			return nil
		},
		Down: func(db *gorm.DB) error { return nil },
	})
}
