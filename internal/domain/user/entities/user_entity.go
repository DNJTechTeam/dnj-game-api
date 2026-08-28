package entities

import (
	"time"
)

// UserRole enumerates the permission levels a User can have.
type UserRole string

const (
	RoleAdmin        UserRole = "ADMIN"
	RoleEventManager UserRole = "EVENT_MANAGER"
	RoleDefault      UserRole = "DEFAULT"
)

// AllowedUserRoles lists every valid UserRole, used for validation.
var AllowedUserRoles = []UserRole{
	RoleAdmin,
	RoleEventManager,
	RoleDefault,
}

// User is the identity aggregate. It carries no tenant/customer association —
// authorization beyond "is this a valid logged-in user" is intentionally out of
// scope for this template. Authentication is passwordless: a User is created
// the first time a subscription webhook verification code is confirmed.
type User struct {
	ID                 uint64
	Email              string
	Name               string
	MobilePhone        string
	Document           string
	DocumentHash       string
	DocumentLast4      string
	Role               UserRole
	ManagerScope       *string
	GroupID            *uint64
	Points             int
	OnboardingComplete bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
}
