package watcher

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/v1/kubernetes"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
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

	cacheSyncs []cache.InformerSynced
}

func NewManager(scheme *runtime.Scheme, clusterConfig kubernetes.ClusterConfigMap, log logrus.FieldLogger, opts ...Option) (*Manager, error) {
	s := &settings{
		configCreator: func(cluster string) rest.Config {
			return clusterConfig[cluster]
		},
	}
	for _, opt := range opts {
		opt(s)
	}

	managers := map[string]*clusterManager{}

	for cluster := range clusterConfig {
		cfg := s.configCreator(cluster)
		var client dynamic.Interface
		var err error
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

func (m *Manager) WaitForReady(ctx context.Context) bool {
	return cache.WaitForCacheSync(ctx.Done(), m.cacheSyncs...)
}

func (m *Manager) addCacheSync(sync cache.InformerSynced) {
	m.cacheSyncs = append(m.cacheSyncs, sync)
}

func Watch[T Object](mgr *Manager, obj T, opts ...WatchOption) *Watcher[T] {
	settings := &watcherSettings{}
	for _, opt := range opts {
		opt(settings)
	}
	return newWatcher(mgr, obj, settings, mgr.log)
}
