package pagination

import (
	"fmt"
	"math"

	"github.com/nais/api/internal/graph/apierror"
)

type Pagination struct {
	offset int
	limit  int
}

func (p *Pagination) Offset() int32 {
	if p == nil || p.offset < 0 {
		return 0
	}
	if p.offset > math.MaxInt32 {
		panic(fmt.Sprintf("offset out of bounds: %d", p.offset))
	}
	return int32(p.offset)
}

func (p *Pagination) Limit() int32 {
	if p == nil || p.limit <= 0 {
		return 20
	}
	if p.limit > math.MaxInt32 {
		panic(fmt.Sprintf("limit out of bounds: %d", p.limit))
	}
	return int32(p.limit)
}

func ParsePage(first *int, after *Cursor, last *int, before *Cursor) (*Pagination, error) {
	p := &Pagination{}

	if first != nil && last != nil {
		first = nil
	}

	switch {
	case first != nil && before.valid():
		return nil, apierror.Errorf("first and before cannot be used together")
	case last != nil && after != nil:
		return nil, apierror.Errorf("last and after cannot be used together")
	case last != nil && before == nil:
		return nil, apierror.Errorf("last must be used with before")
	case before != nil && before.empty:
		return nil, apierror.Errorf("before cannot be empty")
	}

	if first != nil {
		f := *first
		if f < 1 {
			return nil, apierror.Errorf("first must be greater than or equal to 1")
		}
		p.limit = f
	} else if last != nil {
		l := *last
		if l < 1 {
			return nil, apierror.Errorf("last must be greater than or equal to 1")
		}
		p.limit = l
	}

	if after != nil && !after.empty {
		p.offset = after.Offset + 1
	} else if before != nil {
		p.offset = before.Offset - int(p.Limit())
	}

	if p.offset < 0 {
		p.offset = 0
	}

	return p, nil
}
