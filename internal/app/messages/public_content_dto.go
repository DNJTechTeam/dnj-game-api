package messages

import "time"

type PublicSpaceResponseDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type ScheduleItemResponseDTO struct {
	ID          string                  `json:"id"`
	Title       string                  `json:"title"`
	Description *string                 `json:"description"`
	StartsAt    time.Time               `json:"startsAt"`
	EndsAt      time.Time               `json:"endsAt"`
	Sector      *PublicSpaceResponseDTO `json:"sector"`
	State       string                  `json:"state"`
}

type ScheduleResponseDTO struct {
	Items       []ScheduleItemResponseDTO `json:"items"`
	GeneratedAt time.Time                 `json:"generatedAt"`
}

type ListScheduleFilterDTO struct {
	View   string
	Sector string
}

type ListPublicActivitiesFilterDTO struct {
	PaginationFilter
	Kind    string
	SpaceID string
}

type PublicActivityResponseDTO struct {
	ID              string                  `json:"id"`
	Space           *PublicSpaceResponseDTO `json:"space"`
	Slug            string                  `json:"slug"`
	Name            string                  `json:"name"`
	Description     *string                 `json:"description"`
	Kind            string                  `json:"kind"`
	StartsAt        *time.Time              `json:"startsAt"`
	EndsAt          *time.Time              `json:"endsAt"`
	CheckInPoints   int                     `json:"checkInPoints"`
	MomentPoints    int                     `json:"momentPoints"`
	CooldownSeconds int                     `json:"cooldownSeconds"`
	AllowsMoment    bool                    `json:"allowsMoment"`
	State           *string                 `json:"state"`
}

type ListFavoritesFilterDTO struct{ PaginationFilter }
