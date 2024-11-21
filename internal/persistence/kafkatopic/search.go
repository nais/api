package kafkatopic

import (
	"context"

	"github.com/nais/api/internal/search"
)

func init() {
	search.Register("KAFKA_TOPIC", func(ctx context.Context, q string) []*search.Result {
		ret, err := Search(ctx, q)
		if err != nil {
			return nil
		}
		return ret
	})
}
