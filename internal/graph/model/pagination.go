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

func PaginatedSlice[T any](slice []T, p *Pagination) ([]T, PageInfo) {
	if len(slice) < p.Offset {
		return make([]T, 0), PageInfo{
			HasNextPage:     false,
			HasPreviousPage: p.Offset > 0,
			TotalCount:      len(slice),
		}
	}

	if len(slice) < p.Offset+p.Limit {
		return slice[p.Offset:], PageInfo{
			HasNextPage:     false,
			HasPreviousPage: p.Offset > 0,
			TotalCount:      len(slice),
		}
	}

	return slice[p.Offset : p.Offset+p.Limit], PageInfo{
		HasNextPage:     len(slice) > p.Offset+p.Limit,
		HasPreviousPage: p.Offset > 0,
		TotalCount:      len(slice),
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
