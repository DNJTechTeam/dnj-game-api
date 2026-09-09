package repositories

import (
	"context"
	"errors"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestEventSettingsRepository_Get(t *testing.T) {
	t.Run("returns defaults when no row exists", func(t *testing.T) {
		db, mock := newMockDB(t)
		repo := &EventSettingsRepository{db: db}
		mock.ExpectQuery("SELECT").WillReturnError(gorm.ErrRecordNotFound)

		settings, err := repo.Get(context.Background())

		require.NoError(t, err)
		assert.Equal(t, "global", settings.ID)
		assert.False(t, settings.ScoringClosed)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns existing row", func(t *testing.T) {
		db, mock := newMockDB(t)
		repo := &EventSettingsRepository{db: db}
		now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
		rows := sqlmock.NewRows([]string{"id", "scoring_closed", "closed_at", "closed_by", "audit_note", "created_at", "updated_at"}).
			AddRow("global", true, now, uint64(42), "test note", now, now)
		mock.ExpectQuery("SELECT").WillReturnRows(rows)

		settings, err := repo.Get(context.Background())

		require.NoError(t, err)
		assert.True(t, settings.ScoringClosed)
		assert.Equal(t, uint64(42), *settings.ClosedBy)
		assert.Equal(t, "test note", settings.AuditNote)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns error on db failure", func(t *testing.T) {
		db, mock := newMockDB(t)
		repo := &EventSettingsRepository{db: db}
		mock.ExpectQuery("SELECT").WillReturnError(errors.New("db failure"))

		_, err := repo.Get(context.Background())

		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestEventSettingsRepository_SetScoringClosed(t *testing.T) {
	t.Run("creates new row when none exists", func(t *testing.T) {
		db, mock := newMockDB(t)
		repo := &EventSettingsRepository{db: db}
		mock.ExpectQuery("SELECT").WillReturnError(gorm.ErrRecordNotFound)
		mock.ExpectBegin()
		mock.ExpectQuery("INSERT").WillReturnRows(sqlmock.NewRows([]string{"closed_at", "closed_by"}))
		mock.ExpectCommit()

		closedBy := uint64(42)
		err := repo.SetScoringClosed(context.Background(), true, &closedBy, "closing")

		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("updates existing row to closed", func(t *testing.T) {
		db, mock := newMockDB(t)
		repo := &EventSettingsRepository{db: db}
		now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
		rows := sqlmock.NewRows([]string{"id", "scoring_closed", "closed_at", "closed_by", "audit_note", "created_at", "updated_at"}).
			AddRow("global", false, nil, nil, "", now, now)
		mock.ExpectQuery("SELECT").WillReturnRows(rows)
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE").WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		closedBy := uint64(42)
		err := repo.SetScoringClosed(context.Background(), true, &closedBy, "closing")

		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("updates existing row to open", func(t *testing.T) {
		db, mock := newMockDB(t)
		repo := &EventSettingsRepository{db: db}
		now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
		rows := sqlmock.NewRows([]string{"id", "scoring_closed", "closed_at", "closed_by", "audit_note", "created_at", "updated_at"}).
			AddRow("global", true, now, uint64(42), "note", now, now)
		mock.ExpectQuery("SELECT").WillReturnRows(rows)
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE").WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		err := repo.SetScoringClosed(context.Background(), false, nil, "")

		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns error on select failure", func(t *testing.T) {
		db, mock := newMockDB(t)
		repo := &EventSettingsRepository{db: db}
		mock.ExpectQuery("SELECT").WillReturnError(errors.New("db failure"))

		err := repo.SetScoringClosed(context.Background(), true, nil, "")

		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns error on insert failure", func(t *testing.T) {
		db, mock := newMockDB(t)
		repo := &EventSettingsRepository{db: db}
		mock.ExpectQuery("SELECT").WillReturnError(gorm.ErrRecordNotFound)
		mock.ExpectBegin()
		mock.ExpectQuery("INSERT").WillReturnError(errors.New("insert failed"))
		mock.ExpectRollback()

		err := repo.SetScoringClosed(context.Background(), true, nil, "")

		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns error on update failure", func(t *testing.T) {
		db, mock := newMockDB(t)
		repo := &EventSettingsRepository{db: db}
		now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
		rows := sqlmock.NewRows([]string{"id", "scoring_closed", "closed_at", "closed_by", "audit_note", "created_at", "updated_at"}).
			AddRow("global", false, nil, nil, "", now, now)
		mock.ExpectQuery("SELECT").WillReturnRows(rows)
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE").WillReturnError(errors.New("update failed"))
		mock.ExpectRollback()

		err := repo.SetScoringClosed(context.Background(), true, nil, "")

		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
