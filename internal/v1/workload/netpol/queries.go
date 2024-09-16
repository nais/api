package netpol

import (
	"context"

	"github.com/nais/api/internal/slug"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
)

func ListForWorkload(ctx context.Context, policy *nais_io_v1.AccessPolicy) (*NetworkPolicy, error) {
	inbound := &InboundNetworkPolicy{}
	for _, rule := range policy.Inbound.Rules {
		inbound.Rules = append(inbound.Rules, &NetworkPolicyRule{
			TargetWorkloadName: rule.Application,
			TargetTeamSlug:     slug.Slug(rule.Namespace),
			TargetEnvironment:  rule.Cluster,
		})
	}

	outbound := &OutboundNetworkPolicy{}
	for _, rule := range policy.Outbound.Rules {
		outbound.Rules = append(outbound.Rules, &NetworkPolicyRule{
			TargetWorkloadName: rule.Application,
			TargetTeamSlug:     slug.Slug(rule.Namespace),
			TargetEnvironment:  rule.Cluster,
		})
	}

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

	return &NetworkPolicy{
		Inbound:  inbound,
		Outbound: outbound,
	}, nil
}
