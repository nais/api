package pagination

type Edge[T any] struct {
	Node   T      `json:"node"`
	Cursor Cursor `json:"cursor"`
}

type Connection[T any] struct {
	Edges    []Edge[T] `json:"edges"`
	PageInfo PageInfo  `json:"pageInfo"`
}

func (c *Connection[T]) Nodes() []T {
	nodes := make([]T, len(c.Edges))
	for i, edge := range c.Edges {
		nodes[i] = edge.Node
	}
	return nodes
}

type PageInfo struct {
	TotalCount      int     `json:"totalCount"`
	HasNextPage     bool    `json:"hasNextPage"`
	HasPreviousPage bool    `json:"hasPreviousPage"`
	StartCursor     *Cursor `json:"startCursor"`
	EndCursor       *Cursor `json:"endCursor"`
}

func (p *PageInfo) PageStart() int32 {
	if p.StartCursor == nil {
		return 0
	}
	return p.StartCursor.Offset + 1
}

func (p *PageInfo) PageEnd() int32 {
	if p.EndCursor == nil {
		return 0
	}
	return p.EndCursor.Offset + 1
}

func NewConnection[T any](nodes []T, page *Pagination, total int32) *Connection[T] {
	return NewConvertConnection(nodes, page, total, func(from T) T { return from })
}

func NewConvertConnection[T any, F any](nodes []T, page *Pagination, total int32, fn func(from T) F) *Connection[F] {
	c, _ := NewConvertConnectionWithError(nodes, page, total, func(from T) (F, error) {
		return fn(from), nil
	})
	return c
}

func NewConnectionWithoutPagination[T any](nodes []T) *Connection[T] {
	page := &Pagination{
		limit: int32(len(nodes)),
	}
	return NewConnection(nodes, page, int32(len(nodes)))
}

func EmptyConnection[T any]() *Connection[T] {
	return &Connection[T]{
		Edges: []Edge[T]{},
		PageInfo: PageInfo{
			TotalCount: 0,
		},
	}
}

func NewConvertConnectionWithError[T any, F any](nodes []T, page *Pagination, total int32, fn func(from T) (F, error)) (*Connection[F], error) {
	edges := make([]Edge[F], len(nodes))
	for i, node := range nodes {
		converted, err := fn(node)
		if err != nil {
			return nil, err
		}
		edges[i] = Edge[F]{
			Node: converted,
			Cursor: Cursor{
				Offset: page.Offset() + int32(i),
			},
		}
	}

	var startCursor, endCursor *Cursor
	if len(edges) > 0 {
		startCursor = &edges[0].Cursor
		endCursor = &edges[len(edges)-1].Cursor
	}

	return &Connection[F]{
		Edges: edges,
		PageInfo: PageInfo{
			TotalCount:      int(total),
			StartCursor:     startCursor,
			EndCursor:       endCursor,
			HasNextPage:     page.Offset()+page.Limit() < total,
			HasPreviousPage: page.Offset() > 0,
		},
	}, nil
}
