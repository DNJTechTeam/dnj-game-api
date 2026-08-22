package google

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/idtoken"
)

func TestValidatedPayload_EnforcesGoogleIssuerAndVerifiedEmail(t *testing.T) {
	t.Run("accepts required Google claims", func(t *testing.T) {
		// given
		payload := &idtoken.Payload{Issuer: "https://accounts.google.com", Audience: "client", Subject: "sub", Expires: 123, Claims: map[string]any{
			"email": "USER@example.com", "email_verified": true, "name": "User",
		}}

		// when
		result, err := validatedPayload(payload)

		// then
		require.NoError(t, err)
		assert.Equal(t, "user@example.com", result.Email)
		assert.Equal(t, "sub", result.Subject)
	})

	for name, payload := range map[string]*idtoken.Payload{
		"wrong issuer":     {Issuer: "https://attacker.example", Subject: "sub", Claims: map[string]any{"email": "u@example.com", "email_verified": true}},
		"unverified email": {Issuer: "accounts.google.com", Subject: "sub", Claims: map[string]any{"email": "u@example.com", "email_verified": false}},
		"missing subject":  {Issuer: "accounts.google.com", Claims: map[string]any{"email": "u@example.com", "email_verified": true}},
	} {
		t.Run(name, func(t *testing.T) {
			// given / when
			_, err := validatedPayload(payload)

			// then
			require.Error(t, err)
		})
	}
}
