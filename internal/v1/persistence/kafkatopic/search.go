package kafkatopic

import (
	"context"

	"github.com/nais/api/internal/v1/searchv1"
)

func init() {
	searchv1.Register("KAFKA_TOPIC", func(ctx context.Context, q string) []*searchv1.Result {
		ret, err := Search(ctx, q)
		if err != nil {
			return nil
		}
		return ret
	})
}
