package sqlinstance

import (
	"context"

	"github.com/nais/api/internal/search"
)

func init() {
	search.Register("SQL_INSTANCE", func(ctx context.Context, q string) []*search.Result {
		ret, err := Search(ctx, q)
		if err != nil {
			return nil
		}
		return ret
	})
}