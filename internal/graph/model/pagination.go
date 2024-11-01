package model

type Pagination struct {
	Offset int
	Limit  int
}

func NewPagination(offset, limit *int) *Pagination {
	off := 0
	lim := 20
	if offset != nil && *offset > 0 {
		off = *offset
	}
	if limit != nil && *limit > 0 {
		lim = *limit
	}

	return &Pagination{
		Offset: off,
		Limit:  lim,
	}
}

func NewPageInfo(p *Pagination, total int) PageInfo {
	hasNext := p.Offset+p.Limit < total
	hasPrev := p.Offset > 0
	return PageInfo{
		HasNextPage:     hasNext,
		HasPreviousPage: hasPrev,
		TotalCount:      total,
	}
}
