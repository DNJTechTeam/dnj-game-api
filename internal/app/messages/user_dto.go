package messages

import "time"

type UserResponseDTO struct {
	ID                 Uint64String     `json:"id"`
	Email              string           `json:"email"`
	Name               string           `json:"name"`
	AvatarURL          *string          `json:"avatarUrl,omitempty"`
	MobilePhone        string           `json:"mobilePhone"`
	Document           string           `json:"document"`
	DocumentMasked     string           `json:"documentMasked,omitempty"`
	Role               string           `json:"role"`
	OnboardingComplete bool             `json:"onboardingComplete"`
	CreatedAt          time.Time        `json:"createdAt"`
	UpdatedAt          time.Time        `json:"updatedAt"`
	Group              *GroupSummaryDTO `json:"group"`
}
