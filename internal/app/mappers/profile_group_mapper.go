package mappers

import (
	"time"

	"github.com/dnjtechteam/dnj-game-api/internal/app/messages"
	groupEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/group/entities"
	inviteEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/groupinvite/entities"
	membershipEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/groupmembership/entities"
	userEntities "github.com/dnjtechteam/dnj-game-api/internal/domain/user/entities"
)

func MapUserToCurrentProfileDTO(user *userEntities.User, group *groupEntities.Group, rank int64) *messages.CurrentProfileResponseDTO {
	if user == nil {
		return nil
	}
	return &messages.CurrentProfileResponseDTO{
		ID: messages.Uint64StringFromUint64(user.ID), Email: user.Email, Name: user.Name,
		MobilePhone: user.MobilePhone, DocumentMasked: maskDocumentLast4(user.DocumentLast4),
		Role: string(user.Role), Group: MapGroupToSummaryDTO(group), Points: user.Points,
		RankPosition: rank, OnboardingComplete: user.OnboardingComplete,
		CreatedAt: user.CreatedAt, UpdatedAt: user.UpdatedAt,
	}
}

func MapMembershipToResponseDTO(entity *membershipEntities.GroupMembership) *messages.GroupMembershipResponseDTO {
	if entity == nil {
		return nil
	}
	return &messages.GroupMembershipResponseDTO{ID: messages.Uint64StringFromUint64(entity.ID), UserID: messages.Uint64StringFromUint64(entity.UserID), GroupID: messages.Uint64StringFromUint64(entity.GroupID), JoinedAt: entity.JoinedAt}
}

func MapMemberToResponseDTO(entity *membershipEntities.GroupMember) messages.GroupMemberResponseDTO {
	return messages.GroupMemberResponseDTO{ID: messages.Uint64StringFromUint64(entity.UserID), Name: entity.Name, Role: entity.Role, JoinedAt: entity.JoinedAt}
}

func MapInviteToResponseDTO(entity *inviteEntities.GroupInvite, now time.Time, code string) *messages.GroupInviteResponseDTO {
	if entity == nil {
		return nil
	}
	status := "ACTIVE"
	if entity.RevokedAt != nil {
		status = "REVOKED"
	} else if entity.ConsumedAt != nil {
		status = "CONSUMED"
	} else if !entity.ExpiresAt.After(now) {
		status = "EXPIRED"
	}
	var consumedBy, replaces *messages.Uint64String
	if entity.ConsumedByUserID != nil {
		value := messages.Uint64StringFromUint64(*entity.ConsumedByUserID)
		consumedBy = &value
	}
	if entity.ReplacesInviteID != nil {
		value := messages.Uint64StringFromUint64(*entity.ReplacesInviteID)
		replaces = &value
	}
	return &messages.GroupInviteResponseDTO{
		ID: messages.Uint64StringFromUint64(entity.ID), GroupID: messages.Uint64StringFromUint64(entity.GroupID), Status: status,
		ExpiresAt: entity.ExpiresAt, RevokedAt: entity.RevokedAt, ConsumedAt: entity.ConsumedAt,
		ConsumedByUserID: consumedBy, CreatedByUserID: messages.Uint64StringFromUint64(entity.CreatedByUserID),
		ReplacesInviteID: replaces, CreatedAt: entity.CreatedAt, Code: code,
	}
}
