package mappers

import (
	"testing"
	"time"

	inviteEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/groupinvite/entities"
	membershipEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/groupmembership/entities"
	identityEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/identity/entities"
	refreshEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/refreshsession/entities"
	"github.com/dnjtechteam/dnj-game-api/internal/infrastructure/db/models"

	"github.com/stretchr/testify/assert"
)

func TestMapTaskToEntity_NilInput(t *testing.T) {
	assert.Nil(t, MapTaskToEntity(nil))
}

func TestIteration3Mappers(t *testing.T) {
	now := time.Now().UTC()
	membershipModel := &models.GroupMembership{ID: 1, UserID: 2, GroupID: 3, JoinedAt: now, CreatedAt: now, UpdatedAt: now}
	membership := MapGroupMembershipToEntity(membershipModel)
	assert.Equal(t, membershipModel.GroupID, membership.GroupID)
	assert.Equal(t, membershipModel.UserID, MapGroupMembershipEntityToModel(membership).UserID)
	assert.Nil(t, MapGroupMembershipToEntity(nil))
	assert.Nil(t, MapGroupMembershipEntityToModel((*membershipEntities.GroupMembership)(nil)))

	consumerID := uint64(4)
	inviteModel := &models.GroupInvite{ID: 5, GroupID: 3, CodeHash: "hash", ExpiresAt: now.Add(time.Hour), ConsumedByUserID: &consumerID, CreatedByUserID: 1, CreatedAt: now, UpdatedAt: now}
	invite := MapGroupInviteToEntity(inviteModel)
	assert.Equal(t, inviteModel.CodeHash, invite.CodeHash)
	assert.Equal(t, inviteModel.ConsumedByUserID, MapGroupInviteEntityToModel(invite).ConsumedByUserID)
	assert.Nil(t, MapGroupInviteToEntity(nil))
	assert.Nil(t, MapGroupInviteEntityToModel((*inviteEntities.GroupInvite)(nil)))
}

func TestGoogleIdentityMappers(t *testing.T) {
	now := time.Now()
	model := &models.GoogleIdentity{ID: 1, UserID: 2, Provider: "google", Subject: "sub", Email: "a@example.com", CreatedAt: now, UpdatedAt: now}
	entity := MapGoogleIdentityToEntity(model)
	assert.Equal(t, model.Subject, entity.Subject)
	assert.Equal(t, model.Email, MapEntityToGoogleIdentity(entity).Email)
	assert.Nil(t, MapGoogleIdentityToEntity(nil))
	assert.Nil(t, MapEntityToGoogleIdentity((*identityEntities.GoogleIdentity)(nil)))
}

func TestRefreshSessionMappers(t *testing.T) {
	now := time.Now()
	model := &models.RefreshSession{ID: "id", UserID: 2, FamilyID: "family", TokenHash: "hash", ExpiresAt: now, CreatedAt: now, LastUsedAt: now}
	entity := MapRefreshSessionToEntity(model)
	assert.Equal(t, model.FamilyID, entity.FamilyID)
	assert.Equal(t, model.TokenHash, MapEntityToRefreshSession(entity).TokenHash)
	assert.Nil(t, MapRefreshSessionToEntity(nil))
	assert.Nil(t, MapEntityToRefreshSession((*refreshEntities.RefreshSession)(nil)))
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
