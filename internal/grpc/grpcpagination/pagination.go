package grpcpagination

import "github.com/nais/api/pkg/apiclient/protoapi"

type Paginatable interface {
	GetLimit() int64
	GetOffset() int64
}

func Pagination(r Paginatable) (limit int, offset int) {
	limit, offset = int(r.GetLimit()), int(r.GetOffset())
	if limit == 0 {
		limit = 100
	}
	return
}

func PageInfo(r Paginatable, total int) *protoapi.PageInfo {
	limit, offset := Pagination(r)
	return &protoapi.PageInfo{
		TotalCount:      int64(total),
		HasNextPage:     offset+limit < total,
		HasPreviousPage: offset > 0,
	}
}
