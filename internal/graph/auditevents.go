package graph

import (
	"encoding/json"
	"fmt"

	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/model/auditevent"
	"github.com/nais/api/internal/graph/scalar"
)

type (
	resourceActionMappers map[model.AuditEventResourceType]map[model.AuditEventAction]rowMapper
	rowMapper             func(row *database.AuditEvent) (auditevent.AuditEvent, error)
)

var mappers = resourceActionMappers{
	model.AuditEventResourceTypeTeam: {
		model.AuditEventActionTeamCreated: func(row *database.AuditEvent) (auditevent.AuditEvent, error) {
			return baseEvent(row, "Created team"), nil
		},
		model.AuditEventActionTeamDeletionRequested: func(row *database.AuditEvent) (auditevent.AuditEvent, error) {
			return baseEvent(row, "Requested team deletion"), nil
		},
		model.AuditEventActionTeamDeletionConfirmed: func(row *database.AuditEvent) (auditevent.AuditEvent, error) {
			return baseEvent(row, "Confirmed team deletion"), nil
		},
		model.AuditEventActionTeamRotatedDeployKey: func(row *database.AuditEvent) (auditevent.AuditEvent, error) {
			return baseEvent(row, "Rotated deploy key"), nil
		},
		model.AuditEventActionTeamSynchronized: func(row *database.AuditEvent) (auditevent.AuditEvent, error) {
			return baseEvent(row, "Scheduled team for synchronization"), nil
		},
		model.AuditEventActionTeamUpdated: func(row *database.AuditEvent) (auditevent.AuditEvent, error) {
			// TODO: should we split these into multiple events for each changed field? e.g. UpdatedPurpose, UpdatedSlackChannel, UpdatedSlackAlertsChannel?
			return baseEvent(row, "Updated team"), nil
		},
	},
	model.AuditEventResourceTypeTeamMember: {
		model.AuditEventActionTeamMemberAdded: eventWithData(
			func(data auditevent.AuditEventAddMemberData, base auditevent.BaseAuditEvent) auditevent.AuditEvent {
				return auditevent.NewAuditEventAddMember(base, data)
			},
		),
		model.AuditEventActionTeamMemberRemoved: eventWithData(
			func(data auditevent.AuditEventRemoveMemberData, base auditevent.BaseAuditEvent) auditevent.AuditEvent {
				return auditevent.NewAuditEventRemoveMember(base, data)
			},
		),
		model.AuditEventActionTeamMemberSetRole: eventWithData(
			func(data auditevent.AuditEventSetMemberRoleData, base auditevent.BaseAuditEvent) auditevent.AuditEvent {
				return auditevent.NewAuditEventSetMemberRole(base, data)
			},
		),
	},
}

func toGraphAuditEvents(rows []*database.AuditEvent) ([]auditevent.AuditEvent, error) {
	graphEvents := make([]auditevent.AuditEvent, len(rows))
	for i, row := range rows {
		event, err := toEvent(row)
		if err != nil {
			return nil, err
		}

		graphEvents[i] = event
	}
	return graphEvents, nil
}

func toEvent(row *database.AuditEvent) (auditevent.AuditEvent, error) {
	resource, ok := mappers[model.AuditEventResourceType(row.ResourceType)]
	if !ok {
		return nil, fmt.Errorf("unsupported resource type %q", row.ResourceType)
	}

	action, ok := resource[model.AuditEventAction(row.Action)]
	if !ok {
		return nil, fmt.Errorf("unsupported action %q for resource %q", row.Action, row.ResourceType)
	}

	return action(row)
}

func baseEvent(row *database.AuditEvent, message string) auditevent.BaseAuditEvent {
	return auditevent.BaseAuditEvent{
		ID:           scalar.AuditEventIdent(row.ID),
		Action:       model.AuditEventAction(row.Action),
		Actor:        row.Actor,
		CreatedAt:    row.CreatedAt.Time,
		Message:      message,
		ResourceType: model.AuditEventResourceType(row.ResourceType),
		ResourceName: row.ResourceName,
		Team:         *row.TeamSlug,
	}
}

func eventWithData[T any](
	constructor func(data T, base auditevent.BaseAuditEvent) auditevent.AuditEvent,
) func(row *database.AuditEvent) (auditevent.AuditEvent, error) {
	return func(row *database.AuditEvent) (auditevent.AuditEvent, error) {
		var data T
		if row.Data != nil { // TODO: should we expect data?
			if err := json.Unmarshal(row.Data, &data); err != nil {
				return nil, err
			}
		}

		base := baseEvent(row, "")
		return constructor(data, base), nil
	}
}
