package search

import (
	"fmt"
	"io"
	"strconv"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/pagination"
)

type (
	SearchNodeConnection = pagination.Connection[SearchNode]
	SearchNodeEdge       = pagination.Edge[SearchNode]
)

type SearchNode interface {
	ID() ident.Ident
	IsSearchNode()
}

type Result struct {
	Node SearchNode
	Rank int
}

type SearchFilter struct {
	Query string      `json:"query"`
	Type  *SearchType `json:"type,omitempty"`
}

type SearchType string

func (e SearchType) String() string {
	return string(e)
}

func (e *SearchType) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = SearchType(str)
	return nil
}

func (e SearchType) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}
