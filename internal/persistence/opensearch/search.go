package opensearch

import (
	"context"

	"github.com/nais/api/internal/search"
)

func init() {
	search.Register("OPENSEARCH", func(ctx context.Context, q string) []*search.Result {
		ret, err := Search(ctx, q)
		if err != nil {
			return nil
		}
		return ret
	})
}
