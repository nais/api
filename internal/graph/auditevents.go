package graph

import (
	"encoding/json"
	"fmt"

	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/model/auditevent"
	"github.com/nais/api/internal/graph/scalar"
)

func toGraphAuditEvents(rows []*database.AuditEvent) ([]auditevent.AuditEventNode, error) {
	graphEvents := make([]auditevent.AuditEventNode, len(rows))
	for i, row := range rows {
		event, err := toEvent(row)
		if err != nil {
			return nil, err
		}

		graphEvents[i] = event
	}
	return graphEvents, nil
}

func toEvent(row *database.AuditEvent) (auditevent.AuditEventNode, error) {
	event := baseEvent(row)
	switch model.AuditEventResourceType(row.ResourceType) {
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
			return withData(row, func(data model.AuditEventTeamSetPurposeData) auditevent.AuditEventNode {
				msg := fmt.Sprintf("Set purpose to %q", data.Purpose)
				return auditevent.AuditEventTeamSetPurpose{BaseAuditEvent: event.WithMessage(msg), Data: data}
			})

		case model.AuditEventActionTeamSetDefaultSLACkChannel:
			return withData(row, func(data model.AuditEventTeamSetDefaultSlackChannelData) auditevent.AuditEventNode {
				msg := fmt.Sprintf("Set default Slack channel to %q", data.DefaultSlackChannel)
				return auditevent.AuditEventTeamSetDefaultSlackChannel{BaseAuditEvent: event.WithMessage(msg), Data: data}
			})

		case model.AuditEventActionTeamSetAlertsSLACkChannel:
			return withData(row, func(data model.AuditEventTeamSetAlertsSlackChannelData) auditevent.AuditEventNode {
				msg := fmt.Sprintf("Set Slack alert channel in %q to %q", data.Environment, data.ChannelName)
				return auditevent.AuditEventTeamSetAlertsSlackChannel{BaseAuditEvent: event.WithMessage(msg), Data: data}
			})
		}

	case model.AuditEventResourceTypeTeamMember:
		switch model.AuditEventAction(row.Action) {
		case model.AuditEventActionTeamMemberAdded:
			return withData(row, func(data model.AuditEventMemberAddedData) auditevent.AuditEventNode {
				msg := fmt.Sprintf("Added %q", data.MemberEmail)
				return auditevent.AuditEventMemberAdded{BaseAuditEvent: event.WithMessage(msg), Data: data}
			})

		case model.AuditEventActionTeamMemberRemoved:
			return withData(row, func(data model.AuditEventMemberRemovedData) auditevent.AuditEventNode {
				msg := fmt.Sprintf("Removed %q", data.MemberEmail)
				return auditevent.AuditEventMemberRemoved{BaseAuditEvent: event.WithMessage(msg), Data: data}
			})

		case model.AuditEventActionTeamMemberSetRole:
			return withData(row, func(data model.AuditEventMemberSetRoleData) auditevent.AuditEventNode {
				msg := fmt.Sprintf("Set %q to %q", data.MemberEmail, data.Role)
				return auditevent.AuditEventMemberSetRole{BaseAuditEvent: event.WithMessage(msg), Data: data}
			})
		}
	}
	return nil, fmt.Errorf("unsupported action %q for resource %q", row.Action, row.ResourceType)
}

func baseEvent(row *database.AuditEvent) auditevent.BaseAuditEvent {
	return auditevent.BaseAuditEvent{
		ID:           scalar.AuditEventIdent(row.ID),
		Action:       model.AuditEventAction(row.Action),
		Actor:        row.Actor,
		CreatedAt:    row.CreatedAt.Time,
		ResourceType: model.AuditEventResourceType(row.ResourceType),
		ResourceName: row.ResourceName,
		Team:         *row.TeamSlug,
	}
}

func withData[T any](row *database.AuditEvent, callback func(data T) auditevent.AuditEventNode) (auditevent.AuditEventNode, error) {
	var data T
	if row.Data != nil { // TODO: should we expect data?
		if err := json.Unmarshal(row.Data, &data); err != nil {
			return nil, err
		}
	}

	return callback(data), nil
}
