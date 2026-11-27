package api

import (
	"github.com/nais/api/internal/grpc/grpcworkload"
	"github.com/nais/api/internal/kubernetes/watchers"
	"github.com/nais/api/internal/v0s"
)

func newV0sManager(cfg *Config, watchers *watchers.Watchers) (*v0s.Manager, error) {
	if cfg.Fakes.WithFakeKubernetes && cfg.V0s.Endpoint == "" {
		return v0s.NewFakeManager(
			grpcworkload.NewServer(watchers.AppWatcher, watchers.JobWatcher),
			cfg.V0s.SuppressionsEnabled,
		), nil
	}
	return v0s.NewManager(cfg.V0s.Endpoint, cfg.V0s.SuppressionsEnabled)
}
