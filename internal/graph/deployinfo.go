package graph

import "github.com/nais/api/internal/thirdparty/hookd"

// filterDeploysByNameAndKind filters a list of deployments by name and kind
func filterDeploysByNameAndKind(deploys []hookd.Deploy, name, kind string) []hookd.Deploy {
	filtered := make([]hookd.Deploy, 0)
deploys:
	for _, deploy := range deploys {
		for _, resource := range deploy.Resources {
			if resource.Name == name && resource.Kind == kind {
				filtered = append(filtered, deploy)
				continue deploys
			}
		}
	}
	return filtered
}
