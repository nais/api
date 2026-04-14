package tunnel

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/slug"
)

const (
	ActivityLogEntryResourceTypeTunnel activitylog.ActivityLogEntryResourceType = "TUNNEL"
)

func init() {
	activitylog.RegisterTransformer(ActivityLogEntryResourceTypeTunnel, func(entry activitylog.GenericActivityLogEntry) (activitylog.ActivityLogEntry, error) {
		switch entry.Action {
		case activitylog.ActivityLogEntryActionCreated:
			data, err := activitylog.UnmarshalData[tunnelCreatedData](entry)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal Tunnel created activity log entry data: %w", err)
			}
			return TunnelCreatedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Created Tunnel"),
				TunnelName:              data.TunnelName,
				TargetHost:              data.TargetHost,
			}, nil
		case activitylog.ActivityLogEntryActionDeleted:
			data, err := activitylog.UnmarshalData[tunnelDeletedData](entry)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal Tunnel deleted activity log entry data: %w", err)
			}
			return TunnelDeletedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Deleted Tunnel"),
				TunnelName:              data.TunnelName,
			}, nil
		default:
			return nil, fmt.Errorf("unsupported tunnel activity log entry action: %q", entry.Action)
		}
	})

	activitylog.RegisterFilter("TUNNEL_CREATED", activitylog.ActivityLogEntryActionCreated, ActivityLogEntryResourceTypeTunnel)
	activitylog.RegisterFilter("TUNNEL_DELETED", activitylog.ActivityLogEntryActionDeleted, ActivityLogEntryResourceTypeTunnel)
}

type tunnelCreatedData struct {
	TunnelName string `json:"tunnelName"`
	TeamSlug   string `json:"teamSlug"`
	TargetHost string `json:"targetHost"`
}

type tunnelDeletedData struct {
	TunnelName string `json:"tunnelName"`
	TeamSlug   string `json:"teamSlug"`
}

type TunnelCreatedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
	TunnelName string `json:"tunnelName"`
	TargetHost string `json:"targetHost"`
}

type TunnelDeletedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
	TunnelName string `json:"tunnelName"`
}

func LogTunnelCreated(ctx context.Context, t *Tunnel) error {
	teamSlug := slug.Slug(t.TeamSlug)
	return activitylog.Create(ctx, activitylog.CreateInput{
		Action:       activitylog.ActivityLogEntryActionCreated,
		Actor:        authz.ActorFromContext(ctx).User,
		ResourceType: ActivityLogEntryResourceTypeTunnel,
		ResourceName: t.Name,
		TeamSlug:     &teamSlug,
		Data: tunnelCreatedData{
			TunnelName: t.Name,
			TeamSlug:   t.TeamSlug,
			TargetHost: t.Target.Host,
		},
	})
}

func LogTunnelDeleted(ctx context.Context, tunnelName string, teamSlug string) error {
	ts := slug.Slug(teamSlug)
	return activitylog.Create(ctx, activitylog.CreateInput{
		Action:       activitylog.ActivityLogEntryActionDeleted,
		Actor:        authz.ActorFromContext(ctx).User,
		ResourceType: ActivityLogEntryResourceTypeTunnel,
		ResourceName: tunnelName,
		TeamSlug:     &ts,
		Data: tunnelDeletedData{
			TunnelName: tunnelName,
			TeamSlug:   teamSlug,
		},
	})
}
