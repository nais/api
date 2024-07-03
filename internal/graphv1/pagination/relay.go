package pagination

import (
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graphv1/scalar"
)

type Pagination struct {
	offset int32
	limit  int32
}

func (p *Pagination) Offset() int32 {
	if p == nil || p.offset < 0 {
		return 0
	}
	return p.offset
}

func (p *Pagination) Limit() int32 {
	if p == nil || p.limit <= 0 {
		return 20
	}
	return p.limit
}

func ParsePage(first *int, after *scalar.Cursor, last *int, before *scalar.Cursor) (*Pagination, error) {
	p := &Pagination{}

	switch {
	case first != nil && last != nil:
		return nil, apierror.Errorf("first and last cannot be used together")
	case first != nil && before != nil:
		return nil, apierror.Errorf("first and before cannot be used together")
	case last != nil && after != nil:
		return nil, apierror.Errorf("last and after cannot be used together")
	case last != nil && before == nil:
		return nil, apierror.Errorf("last must be used with before")
	}

	if first != nil {
		p.limit = int32(*first)
	} else if last != nil {
		p.limit = int32(*last)
	}

	if p.limit < 0 {
		return nil, apierror.Errorf("after/last must be greater than or equal to 0")
	}

	if after != nil {
		p.offset = after.Offset + 1
	} else if before != nil {
		p.offset = before.Offset - p.Limit()
	}

	if p.offset < 0 {
		p.offset = 0
	}

	return p, nil
}
