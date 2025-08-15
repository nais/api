package graph

import (
	"context"

	"github.com/nais/api/internal/persistence/opensearch"
	opensearchversion "github.com/nais/api/internal/persistence/opensearch/version"
)

func (r *openSearchResolver) Version(ctx context.Context, obj *opensearch.OpenSearch) (string, error) {
	return opensearchversion.GetOpenSearchVersion(ctx, opensearchversion.AivenDataLoaderKey{
		Project:     obj.AivenProject,
		ServiceName: obj.Name,
	})
}
