package graph

import (
	"context"

	"github.com/nais/api/internal/opensearchversion"
	"github.com/nais/api/internal/persistence/opensearch"
)

func (r *openSearchResolver) Version(ctx context.Context, obj *opensearch.OpenSearch) (string, error) {
	return opensearchversion.GetOpenSearchVersion(ctx, opensearchversion.AivenDataLoaderKey{
		Project:     obj.AivenProject,
		ServiceName: obj.Name,
	})
}
