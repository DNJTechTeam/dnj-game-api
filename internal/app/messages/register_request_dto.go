package messages

type RegisterRequestDTO struct {
	Email       string `json:"email"`
	Name        string `json:"name"`
	Password    string `json:"password"`
	MobilePhone string `json:"mobilePhone"`
}
