# Database Migrations

Migrations are plain Go functions, registered in code and tracked in a
`schema_migrations` table. There is no external migration CLI.

## How it works

- `internal/infrastructure/db/migrations/migration_tracker.go` — the registry,
  the `schema_migrations` model, and `RunAll()`.
- `internal/infrastructure/db/migrations/model_migrations.go` — every migration,
  registered in order inside `RegisterModelMigrations`.
- `migrate.go` — `MigrateModels()`, called from `cmd/api/main.go` right after
  the DB connection is opened.

`RunAll()`:

1. Ensures the `schema_migrations` table exists.
2. For each registered migration, checks `schema_migrations` by `Name`. If
   already recorded, it is **skipped**.
3. Otherwise it runs `Up` **inside its own transaction**, inserts the tracking
   row, and commits. Any error rolls the whole migration back.

## Idempotency — required

The tracker guarantees each migration runs once per database. **Even so, every
`Up` must be written idempotently.** Reasons:

- A migration can fail partway (e.g. on statement 2 of 3); on the next boot it
  runs again from the top.
- The same SQL is applied across many databases/environments at different times.

### Rules

- **Creating a table**: use the `createModelMigration` helper — it checks
  `migrator.HasTable` and runs `AutoMigrate` (itself idempotent) or
  `CreateTable`.
- **Adding a column**: guard it.
  ```go
  if migrator.HasTable(&models.Task{}) && !migrator.HasColumn(&models.Task{}, "priority") {
      return db.Exec("ALTER TABLE tasks ADD COLUMN IF NOT EXISTS priority INTEGER NOT NULL DEFAULT 0").Error
  }
  ```
- **Indexes**: `CREATE INDEX IF NOT EXISTS ...`.
- **Dropping**: guard with `HasColumn` / `HasTable` before `DROP`.
- **Seeding data**: check for existence first (`SELECT ... ; if found return nil`),
  and no-op when required env vars are absent. See `seed_admin_user` in
  `model_migrations.go`.

Re-running `make run-api` twice must never error — that is the practical test
of idempotency.

## Adding a migration

Append to `RegisterModelMigrations` (order matters — new migrations go last):

```go
registry.Register(Migration{
    Name:        "add_priority_to_tasks",            // unique, immutable
    Description: "Add priority column to tasks",
    Version:     "1.1.0",
    Up: func(db *gorm.DB) error {
        migrator := db.Migrator()
        if migrator.HasTable(&models.Task{}) && !migrator.HasColumn(&models.Task{}, "priority") {
            return db.Exec("ALTER TABLE tasks ADD COLUMN IF NOT EXISTS priority INTEGER NOT NULL DEFAULT 0").Error
        }
        return nil
    },
    Down: func(db *gorm.DB) error {
        migrator := db.Migrator()
        if migrator.HasColumn(&models.Task{}, "priority") {
            return db.Exec("ALTER TABLE tasks DROP COLUMN priority").Error
        }
        return nil
    },
})
```

> Never rename or edit the `Name` of a migration that already shipped — the
> tracker keys off it. To change applied schema, add a *new* migration.

For a brand-new table, prefer the helper:

```go
registry.Register(createModelMigration("create_widgets_table", "1.1.0", &models.Widget{}))
```
