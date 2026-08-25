package messages

type SpaceResponseDTO struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Slug         string  `json:"slug"`
	MapReference *string `json:"mapReference"`
}

type ListSpacesFilterDTO struct{ PaginationFilter }

type ActivityStateResponseDTO struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}
