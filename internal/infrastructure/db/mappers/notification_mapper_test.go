package mappers

import (
	"testing"

	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"
	"github.com/stretchr/testify/assert"
)

func TestNotificationMapper_NilInputs(t *testing.T) {
	assert.Nil(t, MapNotificationToEntity(nil))
	assert.Nil(t, MapNotificationEntityToModel(nil))
	assert.Nil(t, MapNotificationPreferenceToEntity(nil))
	assert.Nil(t, MapNotificationOperationToEntity(nil))
}

func TestNotificationMapper_RoundTrip(t *testing.T) {
	row := &models.Notification{ID: "n-1", UserID: 1, Category: "points", State: "unread", Title: "T", Body: "B", SourceType: "moment"}
	entity := MapNotificationToEntity(row)
	assert.Equal(t, row.ID, entity.ID)
	back := MapNotificationEntityToModel(entity)
	assert.Equal(t, row.ID, back.ID)
	assert.Equal(t, row.Category, back.Category)
}
