package search

import (
	"fmt"
	"io"
	"strconv"

	"github.com/lithammer/fuzzysearch/fuzzy"
	"github.com/nais/api/internal/graph/pagination"
)

type (
	SearchNodeConnection = pagination.Connection[SearchNode]
	SearchNodeEdge       = pagination.Edge[SearchNode]
)

type SearchNode interface {
	IsSearchNode()
}

type Result struct {
	Node SearchNode
	Rank int
}

// Match returns the rank of a match between q and val. 0 means best match. -1 means no match.
func Match(q, val string) int {
	return fuzzy.RankMatchFold(q, val)
}

func Include(rank int) bool {
	if rank < 0 || rank > 30 {
		return false
	}
	return true
}

type SearchFilter struct {
	Query string      `json:"query"`
	Type  *SearchType `json:"type,omitempty"`
}

type SearchType string

func (e SearchType) String() string {
	return string(e)
}

func (e *SearchType) UnmarshalGQL(v interface{}) error {
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
