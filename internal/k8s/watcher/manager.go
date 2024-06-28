package watcher

import (
	"fmt"
	"log/slog"

	"github.com/nais/api/internal/k8s"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
)

type settings struct {
	configCreator func(cluster string) rest.Config
}

type Option func(*settings)

func WithConfigCreator(fn func(cluster string) rest.Config) Option {
	return func(m *settings) {
		m.configCreator = fn
	}
}

type Manager struct {
	managers map[string]*clusterManager
	scheme   *runtime.Scheme
	log      *slog.Logger
}

func NewManager(scheme *runtime.Scheme, tenant string, cfg k8s.Config, log *slog.Logger, opts ...Option) (*Manager, error) {
	ccm, err := k8s.CreateClusterConfigMap(tenant, cfg)
	if err != nil {
		return nil, fmt.Errorf("creating cluster config map: %w", err)
	}

	s := &settings{
		configCreator: func(cluster string) rest.Config {
			return ccm[cluster]
		},
	}
	for _, opt := range opts {
		opt(s)
	}

	managers := map[string]*clusterManager{}

	for cluster := range ccm {
		cfg := s.configCreator(cluster)
		mgr, err := newClusterManager(scheme, &cfg, log.With(slog.String("cluster", cluster)))
		if err != nil {
			return nil, fmt.Errorf("creating cluster manager: %w", err)
		}

		managers[cluster] = mgr
	}

	return &Manager{
		scheme:   scheme,
		managers: managers,
		log:      log,
	}, nil
}

func (m *Manager) Stop() {
	for _, mgr := range m.managers {
		mgr.informer.Shutdown()
	}
}

func Watch[T Object](mgr *Manager, obj T, opts ...WatchOption) *Watcher[T] {
	settings := &watcherSettings{}
	for _, opt := range opts {
		opt(settings)
	}
	return newWatcher(mgr, obj, settings, mgr.log)
}
