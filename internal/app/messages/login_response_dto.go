package messages

type LoginResponseDTO struct {
	HasUpdatedPassword bool   `json:"hasUpdatedPassword"`
	EmailConfirmed     bool   `json:"emailConfirmed"`
	Email              string `json:"email"`
}
