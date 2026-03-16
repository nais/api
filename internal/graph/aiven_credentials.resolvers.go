package graph

import (
	"context"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/persistence/aivencredentials"
)

func (r *mutationResolver) CreateOpenSearchCredentials(ctx context.Context, input aivencredentials.CreateOpenSearchCredentialsInput) (*aivencredentials.CreateOpenSearchCredentialsPayload, error) {
	if err := authz.CanCreateAivenCredentials(ctx, input.TeamSlug); err != nil {
		return nil, err
	}
	return aivencredentials.CreateOpenSearchCredentials(ctx, input)
}

func (r *mutationResolver) CreateValkeyCredentials(ctx context.Context, input aivencredentials.CreateValkeyCredentialsInput) (*aivencredentials.CreateValkeyCredentialsPayload, error) {
	if err := authz.CanCreateAivenCredentials(ctx, input.TeamSlug); err != nil {
		return nil, err
	}
	return aivencredentials.CreateValkeyCredentials(ctx, input)
}

func (r *mutationResolver) CreateKafkaCredentials(ctx context.Context, input aivencredentials.CreateKafkaCredentialsInput) (*aivencredentials.CreateKafkaCredentialsPayload, error) {
	if err := authz.CanCreateAivenCredentials(ctx, input.TeamSlug); err != nil {
		return nil, err
	}
	return aivencredentials.CreateKafkaCredentials(ctx, input)
}
