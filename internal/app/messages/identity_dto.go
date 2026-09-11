package messages

type GoogleAuthRequestDTO struct {
	IDToken string `json:"idToken" binding:"required"`
}

type EmailSignupRequestDTO struct {
	Email string `json:"email" binding:"required,email"`
}

type EmailSignupResponseDTO struct {
	Status    string `json:"status"`
	DebugCode string `json:"debugCode,omitempty"`
}

type VerifyEmailSignupRequestDTO struct {
	Email string `json:"email" binding:"required,email"`
	Code  string `json:"code" binding:"required"`
}

type CompleteOnboardingRequestDTO struct {
	Document    string               `json:"document" binding:"required"`
	MobilePhone string               `json:"mobilePhone" binding:"required"`
	GroupID     NullableUint64String `json:"groupId"`
}

type IdentityUserResponseDTO struct {
	ID                 Uint64String     `json:"id"`
	Email              string           `json:"email"`
	Name               string           `json:"name"`
	AvatarURL          *string          `json:"avatarUrl,omitempty"`
	MobilePhone        string           `json:"mobilePhone"`
	DocumentMasked     string           `json:"documentMasked"`
	Role               string           `json:"role"`
	Scope              string           `json:"scope,omitempty"`
	Group              *GroupSummaryDTO `json:"group"`
	OnboardingComplete bool             `json:"onboardingComplete"`
}

type IdentitySessionResponseDTO struct {
	AccessToken        string                   `json:"accessToken"`
	TokenType          string                   `json:"tokenType"`
	ExpiresIn          int64                    `json:"expiresIn"`
	// RefreshToken is the opaque rotating token; bearer clients persist it and
	// send it back in the JSON body of /auth/refresh and /auth/logout.
	RefreshToken     string `json:"refreshToken"`
	RefreshExpiresIn int64  `json:"refreshExpiresIn"`
	// CSRFToken only matters for the legacy cookie-based flow.
	CSRFToken          string                   `json:"csrfToken"`
	OnboardingRequired bool                     `json:"onboardingRequired"`
	User               *IdentityUserResponseDTO `json:"user"`
}

// RefreshTokenRequestDTO is the JSON body accepted by /auth/refresh and
// /auth/logout for bearer clients (no cookies, no CSRF).
type RefreshTokenRequestDTO struct {
	RefreshToken string `json:"refreshToken"`
}

type CurrentSessionResponseDTO struct {
	OnboardingRequired bool                     `json:"onboardingRequired"`
	User               *IdentityUserResponseDTO `json:"user"`
}

type LogoutResponseDTO struct {
	Status string `json:"status"`
}
