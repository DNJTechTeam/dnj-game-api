package messages

import "time"

type UserResponseDTO struct {
	ID          Uint64String     `json:"id"`
	Email       string           `json:"email"`
	Name        string           `json:"name"`
	MobilePhone string           `json:"mobilePhone"`
	Document    string           `json:"document"`
	Role        string           `json:"role"`
	CreatedAt   time.Time        `json:"createdAt"`
	UpdatedAt   time.Time        `json:"updatedAt"`
	Group       *GroupSummaryDTO `json:"group"`
}
