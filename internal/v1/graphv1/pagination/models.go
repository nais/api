package pagination

type Edge[T any] struct {
	Node   T      `json:"node"`
	Cursor Cursor `json:"cursor"`
}

type Connection[T any] struct {
	Edges    []Edge[T] `json:"edges"`
	PageInfo PageInfo  `json:"pageInfo"`
}

type PageInfo struct {
	TotalCount      int     `json:"totalCount"`
	HasNextPage     bool    `json:"hasNextPage"`
	HasPreviousPage bool    `json:"hasPreviousPage"`
	StartCursor     *Cursor `json:"startCursor"`
	EndCursor       *Cursor `json:"endCursor"`
}

func NewConnection[T any](nodes []T, page *Pagination, total int32) *Connection[T] {
	return NewConvertConnection(nodes, page, total, func(from T) T { return from })
}

func NewConvertConnection[T any, F any](nodes []T, page *Pagination, total int32, fn func(from T) F) *Connection[F] {
	edges := make([]Edge[F], len(nodes))
	for i, node := range nodes {
		edges[i] = Edge[F]{
			Node: fn(node),
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
	}
}
