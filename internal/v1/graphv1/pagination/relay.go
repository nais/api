package pagination

import (
	"github.com/nais/api/internal/v1/graphv1/apierror"
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

func ParsePage(first *int, after *Cursor, last *int, before *Cursor) (*Pagination, error) {
	p := &Pagination{}

	if first != nil && last != nil {
		first = nil
	}

	switch {
	case first != nil && before != nil:
		return nil, apierror.Errorf("first and before cannot be used together")
	case last != nil && after != nil:
		return nil, apierror.Errorf("last and after cannot be used together")
	case last != nil && before == nil:
		return nil, apierror.Errorf("last must be used with before")
	}

	if first != nil {
		f := int32(*first)
		if f < 1 {
			return nil, apierror.Errorf("first must be greater than or equal to 1")
		}
		p.limit = f
	} else if last != nil {
		l := int32(*last)
		if l < 1 {
			return nil, apierror.Errorf("last must be greater than or equal to 1")
		}
		p.limit = l
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
