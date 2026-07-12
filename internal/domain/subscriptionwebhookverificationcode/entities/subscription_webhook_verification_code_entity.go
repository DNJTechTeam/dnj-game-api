package entities

import "time"

// SubscriptionWebhookVerificationCode is the normalized record produced from
// a subscription webhook, used to onboard a User via a 6-digit code sent by
// email. UserID stays nil until the code is verified successfully.
type SubscriptionWebhookVerificationCode struct {
	ID                    uint64
	SubscriptionWebhookID uint64
	Email                 string
	Name                  string
	MobilePhone           string
	Document              string
	VerificationCode      string
	Group                 string
	UserID                *uint64
	CreatedAt             time.Time
}
