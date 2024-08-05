package kafkatopic

import (
	"context"
	"slices"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/graphv1/ident"
	"github.com/nais/api/internal/v1/graphv1/modelv1"
	"github.com/nais/api/internal/v1/graphv1/pagination"
)

func GetByIdent(ctx context.Context, id ident.Ident) (*KafkaTopic, error) {
	teamSlug, environment, name, err := parseIdent(id)
	if err != nil {
		return nil, err
	}

	return Get(ctx, teamSlug, environment, name)
}

func Get(ctx context.Context, teamSlug slug.Slug, environment, name string) (*KafkaTopic, error) {
	return fromContext(ctx).kafkaTopicLoader.Load(ctx, resourceIdentifier{
		namespace:   teamSlug.String(),
		environment: environment,
		name:        name,
	})
}

func ListForTeam(ctx context.Context, teamSlug slug.Slug, page *pagination.Pagination, orderBy *KafkaTopicOrder) (*KafkaTopicConnection, error) {
	all, err := fromContext(ctx).k8sClient.getKafkaTopicsForTeam(ctx, teamSlug)
	if err != nil {
		return nil, err
	}

	if orderBy != nil {
		switch orderBy.Field {
		case KafkaTopicOrderFieldName:
			slices.SortStableFunc(all, func(a, b *KafkaTopic) int {
				return modelv1.Compare(a.Name, b.Name, orderBy.Direction)
			})
		case KafkaTopicOrderFieldEnvironment:
			slices.SortStableFunc(all, func(a, b *KafkaTopic) int {
				return modelv1.Compare(a.EnvironmentName, b.EnvironmentName, orderBy.Direction)
			})
		}
	}

	instances := pagination.Slice(all, page)
	return pagination.NewConnection(instances, page, int32(len(all))), nil
}
