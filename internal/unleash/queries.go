package unleash

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/nais/api/internal/audit"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/slug"
	bifrost "github.com/nais/bifrost/pkg/unleash"
	unleash_nais_io_v1 "github.com/nais/unleasherator/api/v1"
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

func Create(ctx context.Context, input *CreateUnleashInstanceInput) (*UnleashInstance, error) {
	client := fromContext(ctx).bifrostClient
	// if !m.settings.unleashEnabled {
	// 	return &model.Unleash{
	// 		Enabled: false,
	// 	}, fmt.Errorf("unleash is not enabled")
	// }

	// TODO implement auth, set iap header with actor from context or use psk - must update bifrost to support this
	bi := bifrost.UnleashConfig{
		Name:             input.TeamSlug.String(),
		AllowedTeams:     input.TeamSlug.String(),
		EnableFederation: true,
		AllowedClusters:  "dev-gcp,prod-gcp,dev-fss,prod-fss",
	}
	unleashResponse, err := client.Post(ctx, "/unleash/new", bi)
	if err != nil {
		return nil, err
	}

	var unleashInstance unleash_nais_io_v1.Unleash
	err = json.NewDecoder(unleashResponse.Body).Decode(&unleashInstance)
	if err != nil {
		return nil, fmt.Errorf("decoding unleash instance: %w", err)
	}

	err = audit.Create(ctx, audit.CreateInput{
		Action:       audit.AuditActionCreated,
		Actor:        authz.ActorFromContext(ctx).User,
		ResourceType: auditResourceTypeUnleash,
		ResourceName: input.TeamSlug.String(),
		Data: &UnleashInstanceCreatedAuditEntryData{
			Name: input.TeamSlug.String(),
		},
		TeamSlug: &input.TeamSlug,
	})
	if err != nil {
		return nil, err
	}

	return toUnleashInstance(&unleashInstance), nil
}

func alterTeamAccess(ctx context.Context, teamSlug slug.Slug, allowedTeams []slug.Slug) (*UnleashInstance, error) {
	// if !m.settings.unleashEnabled {
	// 	return &model.Unleash{Enabled: false}, fmt.Errorf("unleash is not enabled")
	// }
	client := fromContext(ctx).bifrostClient

	allowed := make([]string, len(allowedTeams))
	for i, t := range allowedTeams {
		allowed[i] = t.String()
	}

	bi := bifrost.UnleashConfig{
		Name:         teamSlug.String(),
		AllowedTeams: strings.Join(allowed, ","),
	}
	unleashResponse, err := client.Post(ctx, fmt.Sprintf("/unleash/%s/edit", teamSlug.String()), bi)
	if err != nil {
		return nil, err
	}

	var unleashInstance unleash_nais_io_v1.Unleash
	err = json.NewDecoder(unleashResponse.Body).Decode(&unleashInstance)
	if err != nil {
		return nil, fmt.Errorf("decoding unleash instance: %w", err)
	}

	return toUnleashInstance(&unleashInstance), nil
}

func AllowTeamAccess(ctx context.Context, input AllowTeamAccessToUnleashInput) (*UnleashInstance, error) {
	unleash, err := ForTeam(ctx, input.TeamSlug)
	if err != nil {
		return nil, err
	}

	if hasAccessToUnleash(input.AllowedTeamSlug, unleash) {
		// Early exit, nothing to update
		return unleash, nil
	}

	ins, err := alterTeamAccess(ctx, input.TeamSlug, append(unleash.AllowedTeamSlugs, input.AllowedTeamSlug))
	if err != nil {
		return nil, err
	}

	err = audit.Create(ctx, audit.CreateInput{
		Action:       audit.AuditActionUpdated,
		Actor:        authz.ActorFromContext(ctx).User,
		ResourceType: auditResourceTypeUnleash,
		ResourceName: input.TeamSlug.String(),
		Data: &UnleashInstanceUpdatedAuditEntryData{
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
		// Early exit, nothing to update
		return unleash, nil
	}

	ins, err := alterTeamAccess(ctx, input.TeamSlug, slices.DeleteFunc(unleash.AllowedTeamSlugs, func(e slug.Slug) bool {
		return e == input.RevokedTeamSlug
	}))
	if err != nil {
		return nil, err
	}

	err = audit.Create(ctx, audit.CreateInput{
		Action:       audit.AuditActionUpdated,
		Actor:        authz.ActorFromContext(ctx).User,
		ResourceType: auditResourceTypeUnleash,
		ResourceName: input.TeamSlug.String(),
		Data: &UnleashInstanceUpdatedAuditEntryData{
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

// @TODO decide how we want to specify which team can manage Unleash from Console
func hasAccessToUnleash(team slug.Slug, unleash *UnleashInstance) bool {
	for _, t := range unleash.AllowedTeamSlugs {
		if t == team {
			return true
		}
	}

	return false
}
