package messages

import "time"

type CreateSpecialEventRequestDTO struct {
	Title           string   `json:"title"`
	Description     string   `json:"description"`
	Points          int      `json:"points"`
	DurationMinutes int      `json:"durationMinutes"`
	Targets         []string `json:"targets"`
}
type SpecialEventIDRequestDTO struct {
	EventID string `json:"eventId"`
}
type ManagerSpecialEventDTO struct {
	ID            string     `json:"id"`
	Title         string     `json:"title"`
	Description   *string    `json:"description,omitempty"`
	Points        int        `json:"points"`
	Status        string     `json:"status"`
	ExpiresAt     time.Time  `json:"expiresAt"`
	QRAvailableAt *time.Time `json:"qrAvailableAt,omitempty"`
}
type ManagerSpecialEventsResponseDTO struct {
	Events []ManagerSpecialEventDTO `json:"events"`
}
type SpecialEventQRResponseDTO struct {
	QRToken   string    `json:"qrToken"`
	ExpiresAt time.Time `json:"expiresAt"`
}
type ActiveSpecialEventDTO struct {
	ID            string     `json:"id"`
	Title         string     `json:"title"`
	Status        string     `json:"status"`
	StartsAt      time.Time  `json:"startsAt"`
	EndsAt        time.Time  `json:"endsAt"`
	TeaserSeconds int        `json:"teaserSeconds"`
	Points        int        `json:"points"`
	QRAvailableAt *time.Time `json:"qrAvailableAt"`
	QRToken       *string    `json:"qrToken,omitempty"`
}
type ActiveSpecialEventResponseDTO struct {
	Event           *ActiveSpecialEventDTO `json:"event"`
	MomentChallenge any                    `json:"momentChallenge"`
}
type LiveDisplaySpecialEventDTO struct {
	ID      string     `json:"id"`
	Title   string     `json:"title"`
	Status  string     `json:"status"`
	Points  int        `json:"points"`
	EndsAt  time.Time  `json:"endsAt"`
	ReadyAt *time.Time `json:"readyAt"`
	QRToken *string    `json:"qrToken"`
}
