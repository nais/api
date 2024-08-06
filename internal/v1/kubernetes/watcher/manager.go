package watcher

import (
	"fmt"

	"k8s.io/client-go/dynamic"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
)

type settings struct {
	clientCreator func(cluster string) (dynamic.Interface, error)
	configCreator func(cluster string) rest.Config
}

type Option func(*settings)

func WithConfigCreator(fn func(cluster string) rest.Config) Option {
	return func(m *settings) {
		m.configCreator = fn
	}
}

func WithClientCreator(fn func(cluster string) (dynamic.Interface, error)) Option {
	return func(m *settings) {
		m.clientCreator = fn
	}
}

type Manager struct {
	managers map[string]*clusterManager
	scheme   *runtime.Scheme
	log      logrus.FieldLogger
}

func NewManager(scheme *runtime.Scheme, tenant string, cfg Config, log logrus.FieldLogger, opts ...Option) (*Manager, error) {
	ccm, err := CreateClusterConfigMap(tenant, cfg)
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
		var client dynamic.Interface
		if s.clientCreator != nil {
			client, err = s.clientCreator(cluster)
			if err != nil {
				return nil, fmt.Errorf("creating dynamic client: %w", err)
			}
		}
		mgr, err := newClusterManager(client, scheme, &cfg, log.WithField("cluster", cluster))
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
