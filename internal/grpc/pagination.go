package grpc

import "github.com/nais/api/pkg/apiclient/protoapi"

type Paginatable interface {
	GetLimit() int64
	GetOffset() int64
}

func pagination(r Paginatable) (limit int, offset int) {
	limit, offset = int(r.GetLimit()), int(r.GetOffset())

	if limit == 0 {
		limit = 100
	}

	return
}

func pageInfo(r Paginatable, total int) (pageInfo *protoapi.PageInfo) {
	limit, offset := pagination(r)

	pageInfo = &protoapi.PageInfo{
		TotalCount:      int64(total),
		HasNextPage:     offset+limit < total,
		HasPreviousPage: offset > 0,
	}

	return
}
