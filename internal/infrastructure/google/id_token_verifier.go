package google

import (
	"context"
	"fmt"
	"strings"

	"github.com/dnjtechteam/dnj-game-api/internal/app/interfaces"
	"google.golang.org/api/idtoken"
)

type IDTokenVerifier struct{}

func NewIDTokenVerifier() interfaces.GoogleIDTokenVerifierInterface { return &IDTokenVerifier{} }

func (*IDTokenVerifier) Verify(ctx context.Context, rawToken, audience string) (*interfaces.GooglePayload, error) {
	if strings.TrimSpace(rawToken) == "" || strings.TrimSpace(audience) == "" {
		return nil, fmt.Errorf("google token and audience are required")
	}
	payload, err := idtoken.Validate(ctx, rawToken, audience)
	if err != nil {
		return nil, err
	}
	return validatedPayload(payload)
}

func validatedPayload(payload *idtoken.Payload) (*interfaces.GooglePayload, error) {
	if payload == nil {
		return nil, fmt.Errorf("empty google payload")
	}
	if payload.Issuer != "accounts.google.com" && payload.Issuer != "https://accounts.google.com" {
		return nil, fmt.Errorf("invalid google issuer")
	}
	email, _ := payload.Claims["email"].(string)
	emailVerified, _ := payload.Claims["email_verified"].(bool)
	name, _ := payload.Claims["name"].(string)
	if payload.Subject == "" || email == "" || !emailVerified {
		return nil, fmt.Errorf("required verified google identity claims are missing")
	}
	return &interfaces.GooglePayload{
		Issuer: payload.Issuer, Audience: payload.Audience, Subject: payload.Subject,
		Email: strings.ToLower(strings.TrimSpace(email)), EmailVerified: emailVerified,
		Name: strings.TrimSpace(name), ExpiresAt: payload.Expires,
	}, nil
}
