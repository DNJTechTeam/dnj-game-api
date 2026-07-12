package messages

type PaginatedResponse[T any] struct {
	Data       []T        `json:"data"`
	Pagination Pagination `json:"pagination"`
}

type Pagination struct {
	CurrentPage Uint64String `json:"currentPage"`
	HasNextPage bool         `json:"hasNextPage"`
	Limit       int          `json:"limit"`
}
