package webhook

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/webhook/webhooksql"
)

func GetSubscription(ctx context.Context, id uuid.UUID) (*WebhookSubscription, error) {
	return fromContext(ctx).subscriptionLoader.Load(ctx, id)
}

func GetSubscriptionByIdent(ctx context.Context, id ident.Ident) (*WebhookSubscription, error) {
	uid, err := parseSubscriptionIdent(id)
	if err != nil {
		return nil, err
	}
	return GetSubscription(ctx, uid)
}

func GetDelivery(ctx context.Context, id uuid.UUID) (*WebhookDelivery, error) {
	return fromContext(ctx).deliveryLoader.Load(ctx, id)
}

func GetDeliveryByIdent(ctx context.Context, id ident.Ident) (*WebhookDelivery, error) {
	uid, err := parseDeliveryIdent(id)
	if err != nil {
		return nil, err
	}
	return GetDelivery(ctx, uid)
}

func Create(ctx context.Context, input CreateWebhookInput) (*CreateWebhookPayload, error) {
	actor := authz.ActorFromContext(ctx)

	if err := authz.CanCreateWebhook(ctx, input.TeamSlug); err != nil {
		return nil, err
	}

	if err := validateEventTypes(input.TeamSlug, input.EventTypes); err != nil {
		return nil, err
	}

	row, err := db(ctx).CreateSubscription(ctx, webhooksql.CreateSubscriptionParams{
		TeamSlug:   input.TeamSlug,
		Url:        input.URL,
		Secret:     input.Secret,
		EventTypes: input.EventTypes,
		CreatedBy:  actor.User.Identity(),
	})
	if err != nil {
		return nil, fmt.Errorf("creating webhook subscription: %w", err)
	}

	sub := toGraphSubscription(row)

	if d := fromContext(ctx).dispatcher; d != nil {
		// Ping errors are non-fatal and already recorded as a delivery entry
		_ = d.Ping(ctx, sub)
	}

	return &CreateWebhookPayload{
		Webhook: sub,
	}, nil
}

func Update(ctx context.Context, input UpdateWebhookInput) (*UpdateWebhookPayload, error) {
	uid, err := parseSubscriptionIdent(input.ID)
	if err != nil {
		return nil, err
	}

	existing, err := GetSubscription(ctx, uid)
	if err != nil {
		return nil, err
	}

	if err := authz.CanUpdateWebhook(ctx, existing.TeamSlug); err != nil {
		return nil, err
	}

	if input.EventTypes != nil {
		if err := validateEventTypes(existing.TeamSlug, input.EventTypes); err != nil {
			return nil, err
		}
	}

	row, err := db(ctx).UpdateSubscription(ctx, webhooksql.UpdateSubscriptionParams{
		ID:         uid,
		Url:        input.URL,
		Secret:     input.Secret,
		EventTypes: input.EventTypes,
		Enabled:    input.Enabled,
	})
	if err != nil {
		return nil, fmt.Errorf("updating webhook subscription: %w", err)
	}

	return &UpdateWebhookPayload{
		Webhook: toGraphSubscription(row),
	}, nil
}

func Delete(ctx context.Context, input DeleteWebhookInput) (*DeleteWebhookPayload, error) {
	uid, err := parseSubscriptionIdent(input.ID)
	if err != nil {
		return nil, err
	}

	existing, err := GetSubscription(ctx, uid)
	if err != nil {
		return nil, err
	}

	if err := authz.CanDeleteWebhook(ctx, existing.TeamSlug); err != nil {
		return nil, err
	}

	if err := db(ctx).DeleteSubscription(ctx, uid); err != nil {
		return nil, fmt.Errorf("deleting webhook subscription: %w", err)
	}

	return &DeleteWebhookPayload{
		WebhookID: input.ID,
	}, nil
}

func ListForTeam(ctx context.Context, teamSlug slug.Slug, page *pagination.Pagination) (*WebhookSubscriptionConnection, error) {
	q := db(ctx)

	rows, err := q.ListSubscriptionsForTeam(ctx, webhooksql.ListSubscriptionsForTeamParams{
		TeamSlug: &teamSlug,
		Offset:   page.Offset(),
		Limit:    page.Limit(),
	})
	if err != nil {
		return nil, err
	}

	var total int64
	if len(rows) > 0 {
		total = rows[0].TotalCount
	}

	return pagination.NewConvertConnection(rows, page, total, func(row *webhooksql.ListSubscriptionsForTeamRow) *WebhookSubscription {
		return toGraphSubscription(&row.WebhookSubscription)
	}), nil
}

func ListGlobal(ctx context.Context, page *pagination.Pagination) (*WebhookSubscriptionConnection, error) {
	q := db(ctx)

	rows, err := q.ListGlobalSubscriptions(ctx, webhooksql.ListGlobalSubscriptionsParams{
		Offset: page.Offset(),
		Limit:  page.Limit(),
	})
	if err != nil {
		return nil, err
	}

	var total int64
	if len(rows) > 0 {
		total = rows[0].TotalCount
	}

	return pagination.NewConvertConnection(rows, page, total, func(row *webhooksql.ListGlobalSubscriptionsRow) *WebhookSubscription {
		return toGraphSubscription(&row.WebhookSubscription)
	}), nil
}

func ListDeliveries(ctx context.Context, subscriptionID uuid.UUID, page *pagination.Pagination) (*WebhookDeliveryConnection, error) {
	q := db(ctx)

	rows, err := q.ListDeliveriesForSubscription(ctx, webhooksql.ListDeliveriesForSubscriptionParams{
		SubscriptionID: subscriptionID,
		Offset:         page.Offset(),
		Limit:          page.Limit(),
	})
	if err != nil {
		return nil, err
	}

	var total int64
	if len(rows) > 0 {
		total = rows[0].TotalCount
	}

	return pagination.NewConvertConnection(rows, page, total, func(row *webhooksql.ListDeliveriesForSubscriptionRow) *WebhookDelivery {
		return toGraphDelivery(&row.WebhookDelivery)
	}), nil
}

func toGraphSubscription(row *webhooksql.WebhookSubscription) *WebhookSubscription {
	var disabledAt *time.Time
	if row.DisabledAt.Valid {
		disabledAt = &row.DisabledAt.Time
	}

	return &WebhookSubscription{
		UUID:                row.ID,
		TeamSlug:            row.TeamSlug,
		URL:                 row.Url,
		Secret:              row.Secret,
		EventTypes:          row.EventTypes,
		Enabled:             row.Enabled,
		ConsecutiveFailures: int(row.ConsecutiveFailures),
		DisabledAt:          disabledAt,
		CreatedBy:           row.CreatedBy,
		CreatedAt:           row.CreatedAt.Time,
		UpdatedAt:           row.UpdatedAt.Time,
	}
}

func toGraphDelivery(row *webhooksql.WebhookDelivery) *WebhookDelivery {
	body := string(row.RequestBody)

	var respStatus *int
	if row.ResponseStatus != nil {
		s := int(*row.ResponseStatus)
		respStatus = &s
	}

	return &WebhookDelivery{
		UUID:           row.ID,
		SubscriptionID: row.SubscriptionID,
		EventType:      row.EventType,
		RequestBody:    body,
		ResponseStatus: respStatus,
		ResponseBody:   row.ResponseBody,
		DurationMs:     int(row.DurationMs),
		Success:        row.Success,
		CreatedAt:      row.CreatedAt.Time,
	}
}

// MaskedSecret returns the secret with all but the last 4 characters masked.
func MaskedSecret(secret string) string {
	if len(secret) <= 4 {
		return "****"
	}
	return "****" + secret[len(secret)-4:]
}

func validateEventTypes(teamSlug *slug.Slug, eventTypes []string) error {
	for _, et := range eventTypes {
		if !activitylog.IsValidActivityType(et) {
			return fmt.Errorf("invalid event type: %q", et)
		}
		if teamSlug != nil && et != "*" {
			if !activitylog.IsTeamScoped(activitylog.ActivityLogActivityType(et)) {
				return fmt.Errorf("event type %q is global-only and cannot be subscribed to by a team-scoped webhook", et)
			}
		}
	}
	return nil
}
