// Package grpcworkload serves the images of applications and jobs.
package grpcworkload

import (
	"cmp"
	"context"
	"slices"

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
	appEnvs, jobEnvs := s.appWatcher.EnvironmentNames(), s.jobWatcher.EnvironmentNames()
	envs := slices.Compact(slices.Sorted(slices.Values(append(slices.Clone(appEnvs), jobEnvs...))))

	resp := &protoapi.ListWorkloadImagesResponse{}
	for _, env := range envs {
		// A watcher that does not watch an environment (its CRD is not installed there)
		// has nothing to wait for.
		if slices.Contains(appEnvs, env) && !s.appWatcher.HasSynced(env) ||
			slices.Contains(jobEnvs, env) && !s.jobWatcher.HasSynced(env) {
			continue
		}

		workloads := []*protoapi.WorkloadImage{}
		for _, a := range s.appWatcher.GetByCluster(env) {
			workloads = append(workloads, &protoapi.WorkloadImage{
				TeamSlug: a.Obj.Namespace,
				Name:     a.Obj.Name,
				Kind:     protoapi.WorkloadKind_APPLICATION,
				Image:    cmp.Or(a.Obj.GetEffectiveImage(), a.Obj.GetImage()),
			})
		}
		for _, j := range s.jobWatcher.GetByCluster(env) {
			workloads = append(workloads, &protoapi.WorkloadImage{
				TeamSlug: j.Obj.Namespace,
				Name:     j.Obj.Name,
				Kind:     protoapi.WorkloadKind_JOB,
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
