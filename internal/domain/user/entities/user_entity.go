package entities

import (
	"time"
)

// User is the identity aggregate. It carries no tenant/customer association —
// authorization beyond "is this a valid logged-in user" is intentionally out of
// scope for this template.
type User struct {
	ID                  uint64
	Email               string
	Name                string
	MobilePhone         string
	EmailConfirmedAt    *time.Time
	PasswordConfirmedAt *time.Time
	Password            string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}
