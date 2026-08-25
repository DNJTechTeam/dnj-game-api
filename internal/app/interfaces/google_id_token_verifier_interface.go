package interfaces

import "context"

type GooglePayload struct {
	Issuer        string
	Audience      string
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
	ExpiresAt     int64
}

type GoogleIDTokenVerifierInterface interface {
	Verify(ctx context.Context, idToken, audience string) (*GooglePayload, error)
}
