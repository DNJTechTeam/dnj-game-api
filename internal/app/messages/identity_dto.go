package messages

type GoogleAuthRequestDTO struct {
	IDToken string `json:"idToken" binding:"required"`
}

type CompleteOnboardingRequestDTO struct {
	Document    string       `json:"document" binding:"required"`
	MobilePhone string       `json:"mobilePhone" binding:"required"`
	GroupID     Uint64String `json:"groupId" binding:"required"`
}

type IdentityUserResponseDTO struct {
	ID                 Uint64String     `json:"id"`
	Email              string           `json:"email"`
	Name               string           `json:"name"`
	MobilePhone        string           `json:"mobilePhone"`
	DocumentMasked     string           `json:"documentMasked"`
	Role               string           `json:"role"`
	Group              *GroupSummaryDTO `json:"group"`
	OnboardingComplete bool             `json:"onboardingComplete"`
}

type IdentitySessionResponseDTO struct {
	AccessToken        string                   `json:"accessToken"`
	TokenType          string                   `json:"tokenType"`
	ExpiresIn          int64                    `json:"expiresIn"`
	CSRFToken          string                   `json:"csrfToken"`
	OnboardingRequired bool                     `json:"onboardingRequired"`
	User               *IdentityUserResponseDTO `json:"user"`
	RefreshToken       string                   `json:"-"`
}

type CurrentSessionResponseDTO struct {
	OnboardingRequired bool                     `json:"onboardingRequired"`
	User               *IdentityUserResponseDTO `json:"user"`
}

type LogoutResponseDTO struct {
	Status string `json:"status"`
}
