package messages

type OnboardingRequestDTO struct {
	Email    string `json:"email"`
	Document string `json:"document"`
}

type VerificationCodeRequestDTO struct {
	Email            string `json:"email"`
	VerificationCode string `json:"verificationCode"`
}

type VerificationCodeResponseDTO struct {
	UserResponseDTO
	IdentityToken string `json:"identityToken"`
}
