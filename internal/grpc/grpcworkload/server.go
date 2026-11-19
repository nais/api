// Package grpcworkload serves the images of applications and jobs.
package grpcworkload

import (
	"cmp"
	"context"

	"github.com/nais/api/internal/kubernetes/watchers"
	"github.com/nais/api/pkg/apiclient/protoapi"
)

type Server struct {
	appWatcher *watchers.AppWatcher
	jobWatcher *watchers.JobWatcher
	protoapi.UnimplementedWorkloadsServer
}

func NewServer(appWatcher *watchers.AppWatcher, jobWatcher *watchers.JobWatcher) *Server {
	return &Server{appWatcher: appWatcher, jobWatcher: jobWatcher}
}

func (s *Server) ListImages(context.Context, *protoapi.ListWorkloadImagesRequest) (*protoapi.ListWorkloadImagesResponse, error) {
	resp := &protoapi.ListWorkloadImagesResponse{}
	for _, env := range s.appWatcher.EnvironmentNames() {
		if !s.appWatcher.HasSynced(env) || !s.jobWatcher.HasSynced(env) {
			continue
		}

		workloads := []*protoapi.WorkloadImage{}
		for _, a := range s.appWatcher.GetByCluster(env) {
			workloads = append(workloads, &protoapi.WorkloadImage{
				TeamSlug: a.Obj.Namespace,
				Name:     a.Obj.Name,
				Image:    cmp.Or(a.Obj.GetEffectiveImage(), a.Obj.GetImage()),
			})
		}
		for _, j := range s.jobWatcher.GetByCluster(env) {
			workloads = append(workloads, &protoapi.WorkloadImage{
				TeamSlug: j.Obj.Namespace,
				Name:     j.Obj.Name,
				Image:    cmp.Or(j.Obj.GetEffectiveImage(), j.Obj.GetImage()),
			})
		}
		resp.Environments = append(resp.Environments, &protoapi.EnvironmentWorkloadImages{
			Environment: env,
			Workloads:   workloads,
		})
	}
	return resp, nil
}
