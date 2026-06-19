package graph

import (
	"context"

	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/team"
	"github.com/nais/api/internal/webhook"
)

func (r *mutationResolver) CreateWebhook(ctx context.Context, input webhook.CreateWebhookInput) (*webhook.CreateWebhookPayload, error) {
	return webhook.Create(ctx, input)
}

func (r *mutationResolver) UpdateWebhook(ctx context.Context, input webhook.UpdateWebhookInput) (*webhook.UpdateWebhookPayload, error) {
	return webhook.Update(ctx, input)
}

func (r *mutationResolver) DeleteWebhook(ctx context.Context, input webhook.DeleteWebhookInput) (*webhook.DeleteWebhookPayload, error) {
	return webhook.Delete(ctx, input)
}

func (r *queryResolver) GlobalWebhooks(ctx context.Context, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[*webhook.WebhookSubscription], error) {
	if err := authz.RequireGlobalAdmin(ctx); err != nil {
		return nil, err
	}

	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return webhook.ListGlobal(ctx, page)
}

func (r *queryResolver) WebhookEventTypes(ctx context.Context) ([]*activitylog.WebhookEventTypeInfo, error) {
	all := activitylog.KnownEventTypes()
	result := make([]*activitylog.WebhookEventTypeInfo, len(all))
	for i := range all {
		result[i] = &all[i]
	}
	return result, nil
}

func (r *teamResolver) Webhooks(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[*webhook.WebhookSubscription], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return webhook.ListForTeam(ctx, obj.Slug, page)
}

func (r *webhookSubscriptionResolver) MaskedSecret(ctx context.Context, obj *webhook.WebhookSubscription) (string, error) {
	return webhook.MaskedSecret(obj.Secret), nil
}

func (r *webhookSubscriptionResolver) Deliveries(ctx context.Context, obj *webhook.WebhookSubscription, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[*webhook.WebhookDelivery], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return webhook.ListDeliveries(ctx, obj.UUID, page)
}

func (r *Resolver) WebhookSubscription() gengql.WebhookSubscriptionResolver {
	return &webhookSubscriptionResolver{r}
}

type webhookSubscriptionResolver struct{ *Resolver }
