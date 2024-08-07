package iterator

import (
	"context"

	"github.com/nais/api/pkg/apiclient/protoapi"
)

type Iterable[T any] interface {
	GetPageInfo() *protoapi.PageInfo
	GetNodes() []T
}

type Iterator[T any, I Iterable[T]] struct {
	offset int64
	limit  int64
	req    func(limit, offset int64) (I, error)

	// internal
	buffer      []T
	hasNextPage bool
	index       int
	err         error
}

func New[T any, I Iterable[T]](ctx context.Context, limit int64, req func(limit, offset int64) (I, error)) *Iterator[T, I] {
	return &Iterator[T, I]{
		limit:       limit,
		req:         req,
		index:       -1,
		hasNextPage: true,
	}
}

func (i *Iterator[T, I]) Next() bool {
	i.index++
	i.updateBuffer()

	if i.err != nil {
		return false
	}

	return i.index < len(i.buffer)
}

func (i *Iterator[T, I]) Value() T {
	return i.buffer[i.index]
}

func (i *Iterator[T, I]) Err() error {
	return i.err
}

func (i *Iterator[T, I]) updateBuffer() {
	if i.err != nil {
		return
	}

	if !i.hasNextPage {
		return
	}

	if len(i.buffer) == 0 || i.index >= len(i.buffer) {
		iter, err := i.req(i.limit, i.offset)
		if err != nil {
			i.err = err
			return
		}

		i.offset += i.limit
		i.buffer = iter.GetNodes()
		i.index = 0
		i.hasNextPage = iter.GetPageInfo().GetHasNextPage()
	}
}
