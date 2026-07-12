package messages

type PaginationFilter struct {
	Page uint64
}

type PaginationFilterInterface interface {
	SetPage(page uint64)
	GetPage() uint64
}

func (p *PaginationFilter) SetPage(page uint64) {
	p.Page = page
}

func (p *PaginationFilter) GetPage() uint64 {
	return p.Page
}
