package v0s

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"

	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/pagination"
)

const maxPageSize = 200

type pageCursor struct {
	Version   int    `json:"version"`
	Selection string `json:"selection"`
	Token     string `json:"token"`
	// Offset tracks the display position; only Token controls the backend seek.
	Offset int `json:"offset"`
}

type page struct {
	size     int
	cursor   pageCursor
	previous bool
}

func selectionKey(operation, filter string) string {
	sum := sha256.Sum256([]byte(operation + ":" + filter))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func parsePage(first *int, after *pagination.Cursor, selection string) (*page, error) {
	p := &page{size: pagination.DefaultPageSize, cursor: pageCursor{Version: 1, Selection: selection}, previous: after != nil}
	if first != nil {
		p.size = *first
	}
	if p.size < 1 || p.size > maxPageSize {
		return nil, apierror.Errorf("first must be between 1 and %d", maxPageSize)
	}
	if after != nil {
		data, err := base64.RawURLEncoding.DecodeString(after.Keyset)
		var cursor pageCursor
		if err != nil || json.Unmarshal(data, &cursor) != nil ||
			cursor.Version != 1 || cursor.Selection != selection || cursor.Token == "" ||
			cursor.Offset < 0 || cursor.Offset > math.MaxInt32-maxPageSize {
			return nil, apierror.Errorf("Cursor does not belong to this vulnerability selection")
		}
		p.cursor = cursor
	}
	return p, nil
}

func readPage[T any](ctx context.Context, p *page,
	fetch func(context.Context, int32, string) ([]T, []string, string, error), keep func(T) bool,
) ([]pagination.Edge[T], pagination.PageInfo, error) {
	edges := make([]pagination.Edge[T], 0, p.size+1)
	token := p.cursor.Token
	offset := 0
	if p.previous {
		offset = p.cursor.Offset + 1
	}
	info := pagination.PageInfo{HasPreviousPage: p.previous}
	for {
		if err := ctx.Err(); err != nil {
			return nil, pagination.PageInfo{}, err
		}
		values, cursors, next, err := fetch(ctx, int32(p.size+1-len(edges)), token)
		if err != nil {
			return nil, pagination.PageInfo{}, err
		}
		if len(values) != len(cursors) {
			return nil, pagination.PageInfo{}, apierror.Errorf("Vulnerability pagination is not available yet.")
		}
		for i, value := range values {
			if cursors[i] == "" {
				return nil, pagination.PageInfo{}, fmt.Errorf("v0s returned an empty item cursor")
			}
			if !keep(value) {
				continue
			}
			position := offset + len(edges)
			end := pageCursor{Version: 1, Selection: p.cursor.Selection, Token: cursors[i], Offset: position}
			data, err := json.Marshal(end)
			if err != nil {
				return nil, pagination.PageInfo{}, fmt.Errorf("encode vulnerability cursor: %w", err)
			}
			edges = append(edges, pagination.Edge[T]{Node: value, Cursor: pagination.Cursor{
				Keyset: base64.RawURLEncoding.EncodeToString(data),
				Offset: position,
			}})
			if len(edges) > p.size {
				info.HasNextPage = true
				edges = edges[:p.size]
				break
			}
		}
		if info.HasNextPage || next == "" {
			if len(edges) > 0 {
				info.StartCursor = &edges[0].Cursor
				info.EndCursor = &edges[len(edges)-1].Cursor
			}
			return edges, info, nil
		}
		if next == token {
			return nil, pagination.PageInfo{}, fmt.Errorf("v0s returned a non-advancing cursor")
		}
		token = next
	}
}
