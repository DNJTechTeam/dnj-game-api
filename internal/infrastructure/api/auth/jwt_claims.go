package auth

import (
	"github.com/golang-jwt/jwt/v5"
)

// IdentityClaims is the only JWT shape in this template: it identifies a user
// and nothing more. A richer permission model (roles, tenants, scopes) is
// intentionally left out — add it here when you need it.
type IdentityClaims struct {
	UserID             string `json:"sub"`
	HasUpdatedPassword bool   `json:"hasUpdatedPassword"`
	EmailConfirmed     bool   `json:"emailConfirmed"`
	jwt.RegisteredClaims
}
