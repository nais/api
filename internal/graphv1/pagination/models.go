package pagination

import "github.com/nais/api/internal/graphv1/scalar"

type Edge[T any] struct {
	Node   T             `json:"node"`
	Cursor scalar.Cursor `json:"cursor"`
}

type Connection[T any] struct {
	Edges    []Edge[T] `json:"edges"`
	PageInfo PageInfo  `json:"pageInfo"`
}

type PageInfo struct {
	// The total amount if items accessible.
	TotalCount int `json:"totalCount"`
	// Whether or not there exists a next page in the data set.
	HasNextPage bool `json:"hasNextPage"`
	// Whether or not there exists a previous page in the data set.
	HasPreviousPage bool          `json:"hasPreviousPage"`
	StartCursor     scalar.Cursor `json:"startCursor"`
	EndCursor       scalar.Cursor `json:"endCursor"`
}

func NewConnection[T any](nodes []T, limit, total int64, cursor scalar.Cursor) Connection[T] {
	edges := make([]Edge[T], len(nodes))
	for i, node := range nodes {
		edges[i] = Edge[T]{
			Node: node,
			Cursor: scalar.Cursor{
				Offset: cursor.Offset + int64(i),
			},
		}
	}

	return Connection[T]{
		Edges: edges,
		PageInfo: PageInfo{
			TotalCount:      int(total),
			StartCursor:     cursor,
			EndCursor:       scalar.Cursor{Offset: cursor.Offset + int64(len(nodes))},
			HasNextPage:     cursor.Offset+limit < total,
			HasPreviousPage: cursor.Offset > 0,
		},
	}
}
