package bucket

import (
	"context"

	"github.com/nais/api/internal/search"
)

func init() {
	search.Register("BUCKET", func(ctx context.Context, q string) []*search.Result {
		ret, err := Search(ctx, q)
		if err != nil {
			return nil
		}
		return ret
	})
}
