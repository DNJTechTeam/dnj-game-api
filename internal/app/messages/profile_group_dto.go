package messages

import (
	"bytes"
	"encoding/json"
	"time"
)

type UpdateCurrentProfileRequestDTO struct {
	Name        *string `json:"name"`
	MobilePhone *string `json:"mobilePhone"`
	AvatarURL   *string `json:"avatarUrl"`
}

type CurrentProfileResponseDTO struct {
	ID                 Uint64String     `json:"id"`
	Email              string           `json:"email"`
	Name               string           `json:"name"`
	AvatarURL          *string          `json:"avatarUrl,omitempty"`
	MobilePhone        string           `json:"mobilePhone"`
	DocumentMasked     string           `json:"documentMasked"`
	Role               string           `json:"role"`
	Group              *GroupSummaryDTO `json:"group"`
	Points             int              `json:"points"`
	RankPosition       int64            `json:"rankPosition"`
	OnboardingComplete bool             `json:"onboardingComplete"`
	CreatedAt          time.Time        `json:"createdAt"`
	UpdatedAt          time.Time        `json:"updatedAt"`
}

type NullableUint64String struct {
	Set   bool
	Valid bool
	Value uint64
}

func (value *NullableUint64String) UnmarshalJSON(data []byte) error {
	value.Set = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		value.Valid = false
		value.Value = 0
		return nil
	}
	var parsed Uint64String
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}
	value.Valid = true
	value.Value = parsed.Uint64()
	return nil
}

type UpdateCurrentGroupRequestDTO struct {
	GroupID NullableUint64String `json:"groupId"`
}

type GroupMembershipResponseDTO struct {
	ID       Uint64String `json:"id"`
	UserID   Uint64String `json:"userId"`
	GroupID  Uint64String `json:"groupId"`
	JoinedAt time.Time    `json:"joinedAt"`
}

type CurrentGroupResponseDTO struct {
	Group      *GroupSummaryDTO            `json:"group"`
	Membership *GroupMembershipResponseDTO `json:"membership"`
}

type GroupMemberResponseDTO struct {
	ID       Uint64String `json:"id"`
	Name     string       `json:"name"`
	Role     string       `json:"role"`
	JoinedAt time.Time    `json:"joinedAt"`
}

type GroupInviteResponseDTO struct {
	ID               Uint64String  `json:"id"`
	GroupID          Uint64String  `json:"groupId"`
	Status           string        `json:"status"`
	ExpiresAt        time.Time     `json:"expiresAt"`
	RevokedAt        *time.Time    `json:"revokedAt"`
	ConsumedAt       *time.Time    `json:"consumedAt"`
	ConsumedByUserID *Uint64String `json:"consumedByUserId"`
	CreatedByUserID  Uint64String  `json:"createdByUserId"`
	ReplacesInviteID *Uint64String `json:"replacesInviteId"`
	CreatedAt        time.Time     `json:"createdAt"`
	Code             string        `json:"code,omitempty"`
}

type ConsumeGroupInviteRequestDTO struct {
	Code string `json:"code" binding:"required"`
}

type ListGroupsFilterDTO struct{ PaginationFilter }
type ListGroupMembersFilterDTO struct{ PaginationFilter }
type ListGroupInvitesFilterDTO struct{ PaginationFilter }
