package unleash

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/slug"
	"github.com/nais/bifrost/pkg/bifrostclient"
	"github.com/sirupsen/logrus"
)

func GetByIdent(ctx context.Context, id ident.Ident) (*UnleashInstance, error) {
	teamSlug, name, err := parseUnleashInstanceIdent(id)
	if err != nil {
		return nil, err
	}

	instance, err := ForTeam(ctx, teamSlug)
	if err != nil {
		return nil, err
	}

	if instance == nil || instance.Name != name {
		return nil, &watcher.ErrorNotFound{
			Cluster:   "management",
			Namespace: teamSlug.String(),
			Name:      name,
		}
	}
	return instance, nil
}

func ForTeam(ctx context.Context, teamSlug slug.Slug) (*UnleashInstance, error) {
	return fromContext(ctx).unleashWatcher.Get("management", ManagementClusterNamespace, teamSlug.String())
}

func Create(ctx context.Context, input *CreateUnleashForTeamInput) (*UnleashInstance, error) {
	client := fromContext(ctx).bifrostClient

	// TODO implement auth, set iap header with actor from context or use psk - must update bifrost to support this
	fromContext(ctx).log.WithFields(logrus.Fields{
		"team":            input.TeamSlug.String(),
		"allowedClusters": fromContext(ctx).allowedClusters,
		"releaseChannel":  input.ReleaseChannel,
	}).Debug("creating unleash instance with allowed clusters")

	name := input.TeamSlug.String()
	enableFederation := true
	allowedTeams := input.TeamSlug.String()
	allowedClusters := fromContext(ctx).allowedClusters

	req := bifrostclient.UnleashConfigRequest{
		Name:             &name,
		EnableFederation: &enableFederation,
		AllowedTeams:     &allowedTeams,
		AllowedClusters:  &allowedClusters,
	}

	// Set release channel if specified (otherwise bifrost uses its default)
	if input.ReleaseChannel != nil && *input.ReleaseChannel != "" {
		req.ReleaseChannelName = input.ReleaseChannel
	}

	resp, err := client.CreateInstance(ctx, req)
	if err != nil {
		return nil, err
	}

	unleashInstance := bifrostUnleashToK8s(resp.JSON201)

	err = activitylog.Create(ctx, activitylog.CreateInput{
		Action:       activitylog.ActivityLogEntryActionCreated,
		Actor:        authz.ActorFromContext(ctx).User,
		ResourceType: activityLogEntryResourceTypeUnleash,
		ResourceName: input.TeamSlug.String(),
		TeamSlug:     &input.TeamSlug,
	})
	if err != nil {
		return nil, err
	}

	return toUnleashInstance(unleashInstance), nil
}

func alterTeamAccess(ctx context.Context, teamSlug slug.Slug, allowedTeams []slug.Slug) (*UnleashInstance, error) {
	client := fromContext(ctx).bifrostClient

	allowed := make([]string, len(allowedTeams))
	for i, t := range allowedTeams {
		allowed[i] = t.String()
	}
	allowedTeamsStr := strings.Join(allowed, ",")

	req := bifrostclient.UnleashConfigRequest{
		AllowedTeams: &allowedTeamsStr,
	}

	resp, err := client.UpdateInstance(ctx, teamSlug.String(), req)
	if err != nil {
		return nil, err
	}

	unleashInstance := bifrostUnleashToK8s(resp.JSON200)
	return toUnleashInstance(unleashInstance), nil
}

func AllowTeamAccess(ctx context.Context, input AllowTeamAccessToUnleashInput) (*UnleashInstance, error) {
	unleash, err := ForTeam(ctx, input.TeamSlug)
	if err != nil {
		return nil, err
	}

	if hasAccessToUnleash(input.AllowedTeamSlug, unleash) {
		return unleash, nil
	}

	ins, err := alterTeamAccess(ctx, input.TeamSlug, append(unleash.AllowedTeamSlugs, input.AllowedTeamSlug))
	if err != nil {
		return nil, err
	}

	err = activitylog.Create(ctx, activitylog.CreateInput{
		Action:       activitylog.ActivityLogEntryActionUpdated,
		Actor:        authz.ActorFromContext(ctx).User,
		ResourceType: activityLogEntryResourceTypeUnleash,
		ResourceName: input.TeamSlug.String(),
		Data: &UnleashInstanceUpdatedActivityLogEntryData{
			AllowedTeamSlug: &input.AllowedTeamSlug,
		},
		TeamSlug: &input.TeamSlug,
	})
	if err != nil {
		return nil, err
	}

	return ins, nil
}

func RevokeTeamAccess(ctx context.Context, input RevokeTeamAccessToUnleashInput) (*UnleashInstance, error) {
	unleash, err := ForTeam(ctx, input.TeamSlug)
	if err != nil {
		return nil, err
	}

	if !hasAccessToUnleash(input.TeamSlug, unleash) {
		return unleash, nil
	}

	ins, err := alterTeamAccess(ctx, input.TeamSlug, slices.DeleteFunc(unleash.AllowedTeamSlugs, func(e slug.Slug) bool {
		return e == input.RevokedTeamSlug
	}))
	if err != nil {
		return nil, err
	}

	err = activitylog.Create(ctx, activitylog.CreateInput{
		Action:       activitylog.ActivityLogEntryActionUpdated,
		Actor:        authz.ActorFromContext(ctx).User,
		ResourceType: activityLogEntryResourceTypeUnleash,
		ResourceName: input.TeamSlug.String(),
		Data: &UnleashInstanceUpdatedActivityLogEntryData{
			RevokedTeamSlug: &input.RevokedTeamSlug,
		},
		TeamSlug: &input.TeamSlug,
	})
	if err != nil {
		return nil, err
	}

	return ins, nil
}

func Toggles(ctx context.Context, teamSlug slug.Slug) (int, error) {
	val, err := fromContext(ctx).PromQuery(ctx, fmt.Sprintf("sum(feature_toggles_total{job=~%q, namespace=%q})", teamSlug.String(), ManagementClusterNamespace))
	if err != nil {
		return 0, err
	}
	return int(val), nil
}

func APITokens(ctx context.Context, teamSlug slug.Slug) (int, error) {
	val, err := fromContext(ctx).PromQuery(ctx, fmt.Sprintf("sum(client_apps_total{job=~%q, namespace=%q, range=\"allTime\"})", teamSlug.String(), ManagementClusterNamespace))
	if err != nil {
		return 0, err
	}
	return int(val), nil
}

func CPUUsage(ctx context.Context, teamSlug slug.Slug) (float64, error) {
	val, err := fromContext(ctx).PromQuery(ctx, fmt.Sprintf("irate(process_cpu_user_seconds_total{job=%q, namespace=%q}[2m])", teamSlug.String(), ManagementClusterNamespace))
	if err != nil {
		return 0, err
	}
	return float64(val), nil
}

func MemoryUsage(ctx context.Context, teamSlug slug.Slug) (float64, error) {
	val, err := fromContext(ctx).PromQuery(ctx, fmt.Sprintf("process_resident_memory_bytes{job=%q, namespace=%q}", teamSlug.String(), ManagementClusterNamespace))
	if err != nil {
		return 0, err
	}
	return float64(val), nil
}

// GetReleaseChannels fetches available release channels from bifrost
func GetReleaseChannels(ctx context.Context) ([]*UnleashReleaseChannel, error) {
	client := fromContext(ctx).bifrostClient

	resp, err := client.ListChannels(ctx)
	if err != nil {
		return nil, err
	}

	channels := make([]*UnleashReleaseChannel, len(*resp.JSON200))
	for i, r := range *resp.JSON200 {
		channels[i] = toReleaseChannel(&r)
	}

	return channels, nil
}

// UpdateInstance updates an Unleash instance's version configuration
func UpdateInstance(ctx context.Context, input *UpdateUnleashInstanceInput) (*UnleashInstance, error) {
	// Validate input
	if err := input.Validate(ctx); err != nil {
		return nil, err
	}

	client := fromContext(ctx).bifrostClient

	// Verify the instance exists
	instance, err := ForTeam(ctx, input.TeamSlug)
	if err != nil {
		return nil, err
	}
	if instance == nil {
		return nil, fmt.Errorf("unleash instance not found for team %s", input.TeamSlug)
	}

	// Log is intentionally not including user input to avoid log injection
	fromContext(ctx).log.Debug("updating unleash instance version configuration")

	req := bifrostclient.UnleashConfigRequest{}
	if input.ReleaseChannel != nil {
		req.ReleaseChannelName = input.ReleaseChannel
	}

	resp, err := client.UpdateInstance(ctx, input.TeamSlug.String(), req)
	if err != nil {
		return nil, err
	}

	unleashInstance := bifrostUnleashToK8s(resp.JSON200)

	// Log the release channel change
	data := &UnleashInstanceUpdatedActivityLogEntryData{
		UpdatedReleaseChannel: input.ReleaseChannel,
	}

	err = activitylog.Create(ctx, activitylog.CreateInput{
		Action:       activitylog.ActivityLogEntryActionUpdated,
		Actor:        authz.ActorFromContext(ctx).User,
		ResourceType: activityLogEntryResourceTypeUnleash,
		ResourceName: input.TeamSlug.String(),
		Data:         data,
		TeamSlug:     &input.TeamSlug,
	})
	if err != nil {
		return nil, err
	}

	return toUnleashInstance(unleashInstance), nil
}

// @TODO decide how we want to specify which team can manage Unleash from Console
func hasAccessToUnleash(team slug.Slug, unleash *UnleashInstance) bool {
	for _, t := range unleash.AllowedTeamSlugs {
		if t == team {
			return true
		}
	}

	return false
}
