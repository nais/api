package graph

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/loader"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/model/auditevent"
	"github.com/nais/api/internal/graph/scalar"
)

func toGraphAuditEvents(rows []*database.AuditEvent) ([]model.AuditEventNode, error) {
	graphEvents := make([]model.AuditEventNode, len(rows))
	for i, row := range rows {
		event, err := toEvent(row)
		if err != nil {
			return nil, err
		}

		graphEvents[i] = event
	}
	return graphEvents, nil
}

func toEvent(row *database.AuditEvent) (model.AuditEventNode, error) {
	event := baseEvent(row)
	switch model.AuditEventResourceType(row.ResourceType) {
	case model.AuditEventResourceTypeApp:
		switch model.AuditEventAction(row.Action) {
		case model.AuditEventActionDeleted:
			return event.WithMessage("Deleted application"), nil

		case model.AuditEventActionRestarted:
			return event.WithMessage("Restarted application"), nil
		}

	case model.AuditEventResourceTypeNaisjob:
		switch model.AuditEventAction(row.Action) {
		case model.AuditEventActionDeleted:
			return event.WithMessage("Deleted job"), nil
		}

	case model.AuditEventResourceTypeSecret:
		switch model.AuditEventAction(row.Action) {
		case model.AuditEventActionCreated:
			return event.WithMessage("Created secret"), nil
		case model.AuditEventActionUpdated:
			return event.WithMessage("Updated secret"), nil
		case model.AuditEventActionDeleted:
			return event.WithMessage("Deleted secret"), nil
		}

	case model.AuditEventResourceTypeTeam:
		switch model.AuditEventAction(row.Action) {
		case model.AuditEventActionTeamCreated:
			return event.WithMessage("Created team"), nil

		case model.AuditEventActionTeamDeletionRequested:
			return event.WithMessage("Requested team deletion"), nil

		case model.AuditEventActionTeamDeletionConfirmed:
			return event.WithMessage("Confirmed team deletion"), nil

		case model.AuditEventActionTeamDeployKeyRotated:
			return event.WithMessage("Rotated deploy key"), nil

		case model.AuditEventActionTeamSynchronized:
			return event.WithMessage("Scheduled team for synchronization"), nil

		case model.AuditEventActionTeamSetPurpose:
			return withData(row, func(data model.AuditEventTeamSetPurposeData) model.AuditEventNode {
				msg := fmt.Sprintf("Set team description to %q", data.Purpose)
				return auditevent.AuditEventTeamSetPurpose{BaseAuditEvent: event.WithMessage(msg), Data: data}
			})

		case model.AuditEventActionTeamSetDefaultSLACkChannel:
			return withData(row, func(data model.AuditEventTeamSetDefaultSlackChannelData) model.AuditEventNode {
				msg := fmt.Sprintf("Set default Slack channel to %q", data.DefaultSlackChannel)
				return auditevent.AuditEventTeamSetDefaultSlackChannel{BaseAuditEvent: event.WithMessage(msg), Data: data}
			})

		case model.AuditEventActionTeamSetAlertsSLACkChannel:
			return withData(row, func(data model.AuditEventTeamSetAlertsSlackChannelData) model.AuditEventNode {
				msg := fmt.Sprintf("Set Slack alert channel in %q to %q", data.Environment, data.ChannelName)
				return auditevent.AuditEventTeamSetAlertsSlackChannel{BaseAuditEvent: event.WithMessage(msg), Data: data}
			})
		}

	case model.AuditEventResourceTypeTeamMember:
		switch model.AuditEventAction(row.Action) {
		case model.AuditEventActionTeamMemberAdded:
			return withData(row, func(data model.AuditEventMemberAddedData) model.AuditEventNode {
				msg := fmt.Sprintf("Added %q with role %q", data.MemberEmail, data.Role)
				return auditevent.AuditEventMemberAdded{BaseAuditEvent: event.WithMessage(msg), Data: data}
			})

		case model.AuditEventActionTeamMemberRemoved:
			return withData(row, func(data model.AuditEventMemberRemovedData) model.AuditEventNode {
				msg := fmt.Sprintf("Removed %q", data.MemberEmail)
				return auditevent.AuditEventMemberRemoved{BaseAuditEvent: event.WithMessage(msg), Data: data}
			})

		case model.AuditEventActionTeamMemberSetRole:
			return withData(row, func(data model.AuditEventMemberSetRoleData) model.AuditEventNode {
				msg := fmt.Sprintf("Set %q to %q", data.MemberEmail, data.Role)
				return auditevent.AuditEventMemberSetRole{BaseAuditEvent: event.WithMessage(msg), Data: data}
			})
		}

	case model.AuditEventResourceTypeTeamRepository:
		switch model.AuditEventAction(row.Action) {
		case model.AuditEventActionAdded:
			return withData(row, func(data model.AuditEventTeamAddRepositoryData) model.AuditEventNode {
				msg := fmt.Sprintf("Added %q", data.RepositoryName)
				return auditevent.AuditEventTeamAddRepository{BaseAuditEvent: event.WithMessage(msg), Data: data}
			})
		case model.AuditEventActionRemoved:
			return withData(row, func(data model.AuditEventTeamRemoveRepositoryData) model.AuditEventNode {
				msg := fmt.Sprintf("Removed %q", data.RepositoryName)
				return auditevent.AuditEventTeamRemoveRepository{BaseAuditEvent: event.WithMessage(msg), Data: data}
			})
		}

	case model.AuditEventResourceTypeUnleash:
		switch model.AuditEventAction(row.Action) {
		case model.AuditEventActionCreated:
			return event.WithMessage("Created Unleash"), nil
		case model.AuditEventActionUpdated:
			return event.WithMessage("Updated Unleash"), nil
		}

	}
	return nil, fmt.Errorf("unsupported action %q for resource %q", row.Action, row.ResourceType)
}

func baseEvent(row *database.AuditEvent) auditevent.BaseAuditEvent {
	event := auditevent.BaseAuditEvent{
		ID:           scalar.AuditEventIdent(row.ID),
		Action:       model.AuditEventAction(row.Action),
		Actor:        row.Actor,
		CreatedAt:    row.CreatedAt.Time,
		ResourceType: model.AuditEventResourceType(row.ResourceType),
		ResourceName: row.ResourceName,
	}

	if row.TeamSlug != nil {
		event.GQLVars.Team = *row.TeamSlug
	}

	if row.Environment != nil {
		event.GQLVars.Environment = *row.Environment
	}

	return event
}

func withData[T any](row *database.AuditEvent, callback func(data T) model.AuditEventNode) (model.AuditEventNode, error) {
	var data T
	if row.Data != nil { // TODO: should we expect data?
		if err := json.Unmarshal(row.Data, &data); err != nil {
			return nil, err
		}
	}

	return callback(data), nil
}

func resolveEventTeam(ctx context.Context, obj auditevent.BaseAuditEvent) (*model.Team, error) {
	if obj.GQLVars.Team == "" {
		return nil, nil
	}

	return loader.GetTeam(ctx, obj.GQLVars.Team)
}

func resolveEventEnv(ctx context.Context, obj auditevent.BaseAuditEvent) (*model.Env, error) {
	if obj.GQLVars.Environment == "" || obj.GQLVars.Team == "" {
		return nil, nil
	}

	return loader.GetTeamEnvironment(ctx, obj.GQLVars.Team, obj.GQLVars.Environment)
}
