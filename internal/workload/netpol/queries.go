package netpol

import (
	"context"
	"strings"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
)

func ListForWorkload(ctx context.Context, teamSlug slug.Slug, environmentName string, workloadName string, policy *nais_io_v1.AccessPolicy) *NetworkPolicy {
	if policy == nil {
		return &NetworkPolicy{
			Inbound:  &InboundNetworkPolicy{},
			Outbound: &OutboundNetworkPolicy{},
		}
	}

	// No network polcies in onprem environments
	if strings.Contains(environmentName, "-fss") {
		return &NetworkPolicy{
			Inbound:  &InboundNetworkPolicy{},
			Outbound: &OutboundNetworkPolicy{},
		}
	}

	defaultSlug := func(s string) slug.Slug {
		if s == "" {
			return teamSlug
		}
		return slug.Slug(s)
	}

	inbound := &InboundNetworkPolicy{}
	if policy.Inbound != nil {
		for _, rule := range policy.Inbound.Rules {
			if rule.Cluster != "" && strings.Contains(rule.Cluster, "-fss") {
				continue
			}
			if strings.HasSuffix(rule.Application, "-token-generator") && rule.Namespace == "aura" && strings.Contains(environmentName, "dev") {
				continue
			}
			inbound.Rules = append(inbound.Rules, &NetworkPolicyRule{
				TargetWorkloadName: rule.Application,
				TargetTeamSlug:     defaultSlug(rule.Namespace),
				EnvironmentName:    environmentName,
				TeamSlug:           teamSlug,
				WorkloadName:       workloadName,
				Cluster:            rule.Cluster,
			})
		}
	}

	outbound := &OutboundNetworkPolicy{}
	if policy.Outbound != nil {
		for _, rule := range policy.Outbound.Rules {
			if rule.Cluster != "" && strings.Contains(rule.Cluster, "-fss") {
				continue
			}
			if strings.HasSuffix(rule.Application, "-token-generator") && rule.Namespace == "aura" && strings.Contains(environmentName, "dev") {
				continue
			}
			outbound.Rules = append(outbound.Rules, &NetworkPolicyRule{
				TargetWorkloadName: rule.Application,
				TargetTeamSlug:     defaultSlug(rule.Namespace),
				EnvironmentName:    environmentName,
				IsOutbound:         true,
				TeamSlug:           teamSlug,
				WorkloadName:       workloadName,
				Cluster:            rule.Cluster,
			})
		}

		if policy.Outbound.External != nil {
			for _, ext := range policy.Outbound.External {
				// TODO: Add default ports?
				ports := make([]int, 0)
				for _, port := range ext.Ports {
					ports = append(ports, int(port.Port))
				}
				if ext.Host != "" {
					outbound.External = append(outbound.External, &ExternalNetworkPolicyHost{
						Target: ext.Host,
						Ports:  ports,
					})
				} else {
					outbound.External = append(outbound.External, &ExternalNetworkPolicyIpv4{
						Target: ext.IPv4,
						Ports:  ports,
					})
				}
			}
		}
	}

	return &NetworkPolicy{
		Inbound:  inbound,
		Outbound: outbound,
	}
}

func AllowsInboundWorkload(ctx context.Context, teamSlug slug.Slug, environmentName, workloadName string, allowsTeamSlug slug.Slug, allowsWorkloadName string) bool {
	ap := accessPolicyForWorkload(ctx, teamSlug, environmentName, workloadName)
	if ap == nil || ap.Inbound == nil {
		return false
	}

	return allowsWorkload(ap.Inbound.Rules.GetRules(), teamSlug, environmentName, allowsTeamSlug, allowsWorkloadName)
}

func AllowsOutboundWorkload(ctx context.Context, teamSlug slug.Slug, environmentName, workloadName string, allowsTeamSlug slug.Slug, allowsWorkloadName string) bool {
	ap := accessPolicyForWorkload(ctx, teamSlug, environmentName, workloadName)
	if ap == nil || ap.Outbound == nil {
		return false
	}

	return allowsWorkload(ap.Outbound.Rules, teamSlug, environmentName, allowsTeamSlug, allowsWorkloadName)
}

// accessPolicyForWorkload returns the AccessPolicy for a workload, if it exists. The function looks up applications
// first, then jobs.
func accessPolicyForWorkload(ctx context.Context, teamSlug slug.Slug, environmentName, workloadName string) *nais_io_v1.AccessPolicy {
	app, _ := application.Get(ctx, teamSlug, environmentName, workloadName)
	if app != nil && app.Spec != nil {
		return app.Spec.AccessPolicy
	}

	job, _ := job.Get(ctx, teamSlug, environmentName, workloadName)
	if job != nil && job.Spec != nil {
		return job.Spec.AccessPolicy
	}

	return nil
}

func allowsWorkload(rules []nais_io_v1.AccessPolicyRule, teamSlug slug.Slug, environmentName string, allowsTeamSlug slug.Slug, allowsWorkloadName string) bool {
	for _, rule := range rules {
		// If cluster is empty or matches the environment name
		if equalOrWildcard(rule.Cluster, environmentName) || rule.Cluster == "" {
			// If application matches or is a wildcard
			if equalOrWildcard(rule.Application, allowsWorkloadName) {
				// If namespace matches or is a wildcard, or if it's empty and the team slug matches
				if equalOrWildcard(rule.Namespace, allowsTeamSlug.String()) || (rule.Namespace == "" && allowsTeamSlug == teamSlug) {
					return true
				}
			}
		}
	}

	return false
}

func equalOrWildcard(a, b string) bool {
	return a == "*" || a == b
}
