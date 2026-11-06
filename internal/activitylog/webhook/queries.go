package webhook

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/activitylog/webhook/webhooksql"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/slug"
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

	sourceTeamSlugs := input.SourceTeamSlugs
	if sourceTeamSlugs == nil && input.TeamSlug != nil {
		sourceTeamSlugs = []slug.Slug{*input.TeamSlug}
	}
	sourceTeamSlugs, err := validateSourceTeams(input.TeamSlug, sourceTeamSlugs)
	if err != nil {
		return nil, err
	}
	if err := validateSourceTeamsExist(ctx, sourceTeamSlugs); err != nil {
		return nil, err
	}

	var row *webhooksql.WebhookSubscription
	err = database.Transaction(ctx, func(ctx context.Context) error {
		var err error
		row, err = db(ctx).CreateSubscription(ctx, webhooksql.CreateSubscriptionParams{
			TeamSlug:        input.TeamSlug,
			SourceTeamSlugs: sourceTeamSlugs,
			Url:             input.URL,
			Secret:          input.Secret,
			EventTypes:      input.EventTypes,
			CreatedBy:       actor.User.Identity(),
		})
		if err != nil {
			return err
		}
		return activitylog.Create(ctx, activitylog.CreateInput{
			Action:       activitylog.ActivityLogEntryActionCreated,
			Actor:        actor.User,
			ResourceType: ActivityLogEntryResourceTypeWebhook,
			ResourceName: row.ID.String(),
			TeamSlug:     row.TeamSlug,
		})
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

	var sourceTeamSlugs []slug.Slug
	if input.SourceTeamSlugs != nil {
		sourceTeamSlugs, err = validateSourceTeams(existing.TeamSlug, input.SourceTeamSlugs)
		if err != nil {
			return nil, err
		}
		if err := validateSourceTeamsExist(ctx, sourceTeamSlugs); err != nil {
			return nil, err
		}
	}

	var row *webhooksql.WebhookSubscription
	err = database.Transaction(ctx, func(ctx context.Context) error {
		var err error
		row, err = db(ctx).UpdateSubscription(ctx, webhooksql.UpdateSubscriptionParams{
			ID:              uid,
			SourceTeamSlugs: sourceTeamSlugs,
			Url:             input.URL,
			Secret:          input.Secret,
			EventTypes:      input.EventTypes,
			Enabled:         input.Enabled,
		})
		if err != nil {
			return err
		}
		return activitylog.Create(ctx, activitylog.CreateInput{
			Action:       activitylog.ActivityLogEntryActionUpdated,
			Actor:        authz.ActorFromContext(ctx).User,
			ResourceType: ActivityLogEntryResourceTypeWebhook,
			ResourceName: row.ID.String(),
			TeamSlug:     row.TeamSlug,
			Data: &WebhookUpdatedActivityLogEntryData{
				UpdatedFields: webhookUpdatedFields(existing, toGraphSubscription(row)),
			},
		})
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

	if err := database.Transaction(ctx, func(ctx context.Context) error {
		if err := db(ctx).DeleteSubscription(ctx, uid); err != nil {
			return err
		}
		return activitylog.Create(ctx, activitylog.CreateInput{
			Action:       activitylog.ActivityLogEntryActionDeleted,
			Actor:        authz.ActorFromContext(ctx).User,
			ResourceType: ActivityLogEntryResourceTypeWebhook,
			ResourceName: existing.UUID.String(),
			TeamSlug:     existing.TeamSlug,
		})
	}); err != nil {
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

	sourceTeamSlugs := row.SourceTeamSlugs
	// Older instances may still create own-team webhooks during a rolling deployment.
	if sourceTeamSlugs == nil {
		sourceTeamSlugs = []slug.Slug{}
		if row.TeamSlug != nil {
			sourceTeamSlugs = []slug.Slug{*row.TeamSlug}
		}
	}

	return &WebhookSubscription{
		UUID:                row.ID,
		TeamSlug:            row.TeamSlug,
		SourceTeamSlugs:     sourceTeamSlugs,
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

func validateSourceTeams(owner *slug.Slug, sourceTeamSlugs []slug.Slug) ([]slug.Slug, error) {
	if owner == nil {
		if len(sourceTeamSlugs) > 0 {
			return nil, apierror.Errorf("Global webhooks cannot select source teams.")
		}
		return []slug.Slug{}, nil
	}
	result := make([]slug.Slug, 0, len(sourceTeamSlugs))
	for _, s := range sourceTeamSlugs {
		if err := s.Validate(); err != nil {
			return nil, err
		}
		if !slices.Contains(result, s) {
			result = append(result, s)
		}
	}
	return result, nil
}

func validateSourceTeamsExist(ctx context.Context, sourceTeamSlugs []slug.Slug) error {
	if len(sourceTeamSlugs) == 0 {
		return nil
	}
	existing, err := db(ctx).ListExistingSourceTeams(ctx, sourceTeamSlugs)
	if err != nil {
		return fmt.Errorf("validating webhook source teams: %w", err)
	}
	for _, s := range sourceTeamSlugs {
		if !slices.Contains(existing, s) {
			return apierror.Errorf("Source team %q does not exist.", s)
		}
	}
	return nil
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

func validateEventTypes(teamSlug *slug.Slug, eventTypes []string) error {
	for _, et := range eventTypes {
		if !activitylog.IsValidActivityType(et) {
			return fmt.Errorf("invalid event type: %q", et)
		}
		if activitylog.IsIgnoredForWebhook(activitylog.ActivityLogActivityType(et)) {
			return fmt.Errorf("event type %q is excluded from webhooks", et)
		}
		if teamSlug != nil && et != "*" {
			if !activitylog.IsTeamScoped(activitylog.ActivityLogActivityType(et)) {
				return fmt.Errorf("event type %q is global-only and cannot be subscribed to by a team-scoped webhook", et)
			}
		}
	}
	return nil
}
