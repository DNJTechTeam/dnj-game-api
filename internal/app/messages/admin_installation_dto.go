package messages

import (
	"bytes"
	"encoding/json"
	"time"
)

// Optional preserves the distinction between an omitted field and an explicit
// JSON null, which is required by the administrative PATCH contracts.
type Optional[T any] struct {
	Set   bool `json:"set"`
	Value *T   `json:"value"`
}

func (o *Optional[T]) UnmarshalJSON(data []byte) error {
	o.Set = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		o.Value = nil
		return nil
	}
	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	o.Value = &value
	return nil
}

type CreateAdminSpaceRequestDTO struct {
	Slug         *string          `json:"slug"`
	Name         *string          `json:"name"`
	MapReference Optional[string] `json:"mapReference"`
}

type UpdateAdminSpaceRequestDTO struct {
	Slug         Optional[string] `json:"slug"`
	Name         Optional[string] `json:"name"`
	MapReference Optional[string] `json:"mapReference"`
}

type ListAdminSpacesFilterDTO struct{ PaginationFilter }

type CreateAdminActivityRequestDTO struct {
	SpaceID         Optional[string]    `json:"spaceId"`
	Slug            *string             `json:"slug"`
	Name            *string             `json:"name"`
	Description     Optional[string]    `json:"description"`
	Kind            *string             `json:"kind"`
	StartsAt        Optional[time.Time] `json:"startsAt"`
	EndsAt          Optional[time.Time] `json:"endsAt"`
	CheckInPoints   *int                `json:"checkInPoints"`
	MomentPoints    *int                `json:"momentPoints"`
	CooldownSeconds *int                `json:"cooldownSeconds"`
	AllowsMoment    *bool               `json:"allowsMoment"`
}

type UpdateAdminActivityRequestDTO struct {
	SpaceID         Optional[string]    `json:"spaceId"`
	Slug            Optional[string]    `json:"slug"`
	Name            Optional[string]    `json:"name"`
	Description     Optional[string]    `json:"description"`
	Kind            Optional[string]    `json:"kind"`
	StartsAt        Optional[time.Time] `json:"startsAt"`
	EndsAt          Optional[time.Time] `json:"endsAt"`
	CheckInPoints   Optional[int]       `json:"checkInPoints"`
	MomentPoints    Optional[int]       `json:"momentPoints"`
	CooldownSeconds Optional[int]       `json:"cooldownSeconds"`
	AllowsMoment    Optional[bool]      `json:"allowsMoment"`
	Status          Optional[string]    `json:"status"`
}

type ListAdminActivitiesFilterDTO struct{ PaginationFilter }

type AdminActivityResponseDTO struct {
	ID              string     `json:"id"`
	SpaceID         *string    `json:"spaceId"`
	Slug            string     `json:"slug"`
	Name            string     `json:"name"`
	Description     *string    `json:"description"`
	Kind            string     `json:"kind"`
	Status          string     `json:"status"`
	StartsAt        *time.Time `json:"startsAt"`
	EndsAt          *time.Time `json:"endsAt"`
	CheckInPoints   int        `json:"checkInPoints"`
	MomentPoints    int        `json:"momentPoints"`
	CooldownSeconds int        `json:"cooldownSeconds"`
	AllowsMoment    bool       `json:"allowsMoment"`
}

type ListAdminStaffFilterDTO struct {
	PaginationFilter
	Role string
}

type AdminStaffResponseDTO struct {
	ID                 Uint64String `json:"id"`
	Name               string       `json:"name"`
	Role               string       `json:"role"`
	OnboardingComplete bool         `json:"onboardingComplete"`
}

type UpdateAdminUserRoleRequestDTO struct {
	Role *string `json:"role"`
}

type AdminUserRoleResponseDTO struct {
	ID   Uint64String `json:"id"`
	Role string       `json:"role"`
}

type ListAdminManagersFilterDTO struct{ PaginationFilter }

type AdminManagerAssignmentResponseDTO struct {
	ActivityID string       `json:"activityId"`
	UserID     Uint64String `json:"userId"`
}
