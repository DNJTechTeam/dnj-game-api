package messages

// OnboardingRequestDTO — Document is always required. Email is only read
// when the matched record has no email on file yet (see
// OnboardingResponseDTO's EMAIL_REQUIRED status); when the record already
// has an email, this field is ignored and the stored email is used instead.
type OnboardingRequestDTO struct {
	Document string `json:"document" example:"12345678900"`
	Email    string `json:"email" example:"jovem@example.com"`
}

const (
	// OnboardingStatusCodeSent means the verification code was emailed —
	// Email carries the obfuscated address it was sent to.
	OnboardingStatusCodeSent = "CODE_SENT"
	// OnboardingStatusEmailRequired means the document was found but has no
	// email on file. Nothing was sent. The caller must collect the email
	// from the user and call this same endpoint again with it filled in.
	OnboardingStatusEmailRequired = "EMAIL_REQUIRED"
)

// OnboardingResponseDTO — Email is only present (and always obfuscated —
// first + last character of the local-part, full domain, e.g.
// "c***a@hotmail.com") when Status is CODE_SENT.
type OnboardingResponseDTO struct {
	Status string `json:"status" enums:"CODE_SENT,EMAIL_REQUIRED" example:"CODE_SENT"`
	Email  string `json:"email,omitempty" example:"c***a@hotmail.com"`
}

type VerificationCodeRequestDTO struct {
	Email            string `json:"email"`
	VerificationCode string `json:"verificationCode"`
}

type VerificationCodeResponseDTO struct {
	UserResponseDTO
	IdentityToken string `json:"identityToken"`
}
