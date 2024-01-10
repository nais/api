package database

import (
	"context"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/search"
)

func (d *database) Search(ctx context.Context, q string, filter *model.SearchFilter) []*search.Result {
	return []*search.Result{}
}
