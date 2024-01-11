package graph

import (
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/thirdparty/hookd"
)

func deployToModel(deploys []hookd.Deploy) []*model.Deployment {
	ret := make([]*model.Deployment, 0)

	for _, deploy := range deploys {
		deploy := deploy
		ret = append(ret, &model.Deployment{
			ID:        scalar.DeploymentIdent(deploy.DeploymentInfo.ID),
			Statuses:  mapStatuses(deploy.Statuses),
			Resources: mapResources(deploy.Resources),
			Team: model.Team{
				Slug: deploy.DeploymentInfo.Team,
			},
			Env:        deploy.DeploymentInfo.Cluster,
			Created:    deploy.DeploymentInfo.Created,
			Repository: deploy.DeploymentInfo.GithubRepository,
		})

	}

	return ret
}

func mapResources(resources []hookd.Resource) []*model.DeploymentResource {
	ret := make([]*model.DeploymentResource, 0)
	for _, resource := range resources {
		ret = append(ret, &model.DeploymentResource{
			ID:        scalar.DeploymentResourceIdent(resource.ID),
			Group:     resource.Group,
			Kind:      resource.Kind,
			Name:      resource.Name,
			Namespace: resource.Namespace,
			Version:   resource.Version,
		})
	}
	return ret
}

func mapStatuses(statuses []hookd.Status) []*model.DeploymentStatus {
	ret := make([]*model.DeploymentStatus, 0)
	for _, status := range statuses {
		ret = append(ret, &model.DeploymentStatus{
			ID:      scalar.DeploymentStatusIdent(status.ID),
			Status:  status.Status,
			Message: &status.Message,
			Created: status.Created,
		})
	}
	return ret
}
