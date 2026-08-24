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
}
