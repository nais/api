package netpol

import (
	"github.com/nais/api/internal/slug"
)

type ExternalNetworkPolicyTarget interface {
	IsExternalNetworkPolicyTarget()
}

type ExternalNetworkPolicyHost struct {
	Target string `json:"target"`
	Ports  []int  `json:"ports"`
}

func (ExternalNetworkPolicyHost) IsExternalNetworkPolicyTarget() {}

type ExternalNetworkPolicyIpv4 struct {
	Target string `json:"target"`
	Ports  []int  `json:"ports"`
}

func (ExternalNetworkPolicyIpv4) IsExternalNetworkPolicyTarget() {}

type InboundNetworkPolicy struct {
	Rules []*NetworkPolicyRule `json:"rules"`
}

type NetworkPolicy struct {
	Inbound  *InboundNetworkPolicy  `json:"inbound"`
	Outbound *OutboundNetworkPolicy `json:"outbound"`
}

type NetworkPolicyRule struct {
	TargetWorkloadName string    `json:"targetWorkloadName"`
	TargetTeamSlug     slug.Slug `json:"targetTeamSlug"`

	EnvironmentName string    `json:"-"`
	TeamSlug        slug.Slug `json:"-"`
	WorkloadName    string    `json:"-"`
	IsOutbound      bool      `json:"-"`
	Cluster         string    `json:"-"`
	IsLikelyNetPol  bool      `json:"-"`
}

type OutboundNetworkPolicy struct {
	Rules    []*NetworkPolicyRule          `json:"rules"`
	External []ExternalNetworkPolicyTarget `json:"external"`
}
