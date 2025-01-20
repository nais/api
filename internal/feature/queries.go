package feature

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/model"
)

func Get(ctx context.Context) (*Features, error) {
	return fromContext(ctx).features, nil
}

func getByIdent(ctx context.Context, id ident.Ident) (model.Node, error) {
	feature, err := parseIdent(id)
	if err != nil {
		return nil, err
	}

	container, err := Get(ctx)
	if err != nil {
		return nil, err
	}

	switch feature {
	case "unleash":
		return container.Unleash, nil
	case "redis":
		return container.Redis, nil
	case "valkey":
		return container.Valkey, nil
	case "kafka":
		return container.Kafka, nil
	case "openSearch":
		return container.OpenSearch, nil
	case "container":
		return container, nil
	default:
		return nil, fmt.Errorf("unknown feature: %s", feature)
	}
}
