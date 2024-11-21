package grpcpagination

import (
	"math"

	"github.com/nais/api/pkg/apiclient/protoapi"
)

type Paginatable interface {
	GetLimit() int64
	GetOffset() int64
}

func Pagination(r Paginatable) (limit int32, offset int32) {
	l, o := r.GetLimit(), r.GetOffset()
	if l > math.MaxInt32 {
		panic("limit exceeds maximum int32")
	}
	if o > math.MaxInt32 {
		panic("offset exceeds maximum int32")
	}

	limit, offset = int32(l), int32(o) // #nosec G115
	if limit == 0 {
		limit = 100
	}
	return
}

func PageInfo(r Paginatable, total int) *protoapi.PageInfo {
	limit, offset := Pagination(r)
	return &protoapi.PageInfo{
		TotalCount:      int64(total),
		HasNextPage:     int(offset)+int(limit) < total,
		HasPreviousPage: offset > 0,
	}
}
