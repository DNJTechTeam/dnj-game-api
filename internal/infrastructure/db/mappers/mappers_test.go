package mappers

import (
	"testing"
	"time"

	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"

	"github.com/stretchr/testify/assert"
)

func TestMapTaskToEntity_NilInput(t *testing.T) {
	assert.Nil(t, MapTaskToEntity(nil))
}

func TestMapTaskEntityToModel_NilInput(t *testing.T) {
	assert.Nil(t, MapTaskEntityToModel(nil))
}

func TestMapUserToEntity_NilInput(t *testing.T) {
	assert.Nil(t, MapUserToEntity(nil))
}

func TestMapEntityToUser_NilInput(t *testing.T) {
	assert.Nil(t, MapEntityToUser(nil))
}

func TestMapUserToEntity_Fields(t *testing.T) {
	groupID := uint64(7)
	now := time.Now()
	model := &models.User{
		ID:          1,
		Email:       "user@example.com",
		Name:        "User",
		MobilePhone: "41999999999",
		Document:    "12345678900",
		Role:        "DEFAULT",
		GroupID:     &groupID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	entity := MapUserToEntity(model)
	assert.Equal(t, model.ID, entity.ID)
	assert.Equal(t, model.Email, entity.Email)
	assert.Equal(t, model.Document, entity.Document)
	assert.Equal(t, model.GroupID, entity.GroupID)

	back := MapEntityToUser(entity)
	assert.Equal(t, model.Email, back.Email)
	assert.Equal(t, model.GroupID, back.GroupID)
}

func TestMapGroupToEntity_NilInput(t *testing.T) {
	assert.Nil(t, MapGroupToEntity(nil))
}

func TestMapGroupEntityToModel_NilInput(t *testing.T) {
	assert.Nil(t, MapGroupEntityToModel(nil))
}

func TestMapGroupToEntity_Fields(t *testing.T) {
	model := &models.Group{ID: 1, Name: "Grupo Jovens A"}
	entity := MapGroupToEntity(model)
	assert.Equal(t, model.ID, entity.ID)
	assert.Equal(t, model.Name, entity.Name)

	back := MapGroupEntityToModel(entity)
	assert.Equal(t, model.Name, back.Name)
}

func TestMapSubscriptionWebhookToEntity_NilInput(t *testing.T) {
	assert.Nil(t, MapSubscriptionWebhookToEntity(nil))
}

func TestMapSubscriptionWebhookEntityToModel_NilInput(t *testing.T) {
	assert.Nil(t, MapSubscriptionWebhookEntityToModel(nil))
}

func TestMapSubscriptionWebhookToEntity_Fields(t *testing.T) {
	now := time.Now()
	model := &models.SubscriptionWebhook{ID: 1, Payload: `{"email":"a@a.com"}`, CreatedAt: now}
	entity := MapSubscriptionWebhookToEntity(model)
	assert.Equal(t, model.Payload, entity.Payload)

	back := MapSubscriptionWebhookEntityToModel(entity)
	assert.Equal(t, model.Payload, back.Payload)
}

func TestMapSubscriptionWebhookVerificationCodeToEntity_NilInput(t *testing.T) {
	assert.Nil(t, MapSubscriptionWebhookVerificationCodeToEntity(nil))
}

func TestMapSubscriptionWebhookVerificationCodeEntityToModel_NilInput(t *testing.T) {
	assert.Nil(t, MapSubscriptionWebhookVerificationCodeEntityToModel(nil))
}

func TestMapSubscriptionWebhookVerificationCodeToEntity_Fields(t *testing.T) {
	userID := uint64(5)
	now := time.Now()
	model := &models.SubscriptionWebhookVerificationCode{
		ID:                    1,
		SubscriptionWebhookID: 2,
		Email:                 "sub@example.com",
		Name:                  "Sub",
		MobilePhone:           "41999999999",
		Document:              "12345678900",
		VerificationCode:      "123456",
		Group:                 "Grupo A",
		UserID:                &userID,
		CreatedAt:             now,
	}

	entity := MapSubscriptionWebhookVerificationCodeToEntity(model)
	assert.Equal(t, model.Email, entity.Email)
	assert.Equal(t, model.VerificationCode, entity.VerificationCode)
	assert.Equal(t, model.UserID, entity.UserID)

	back := MapSubscriptionWebhookVerificationCodeEntityToModel(entity)
	assert.Equal(t, model.Email, back.Email)
	assert.Equal(t, model.UserID, back.UserID)
}
