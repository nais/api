package watcher

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/kubernetes"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	schemepkg "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

type Discovery interface {
	ServerResourcesForGroupVersion(groupVersion string) (*metav1.APIResourceList, error)
}

type settings struct {
	clientCreator func(cluster string) (dynamic.Interface, Discovery, *rest.Config, error)
}

type Option func(*settings)

func WithClientCreator(fn func(cluster string) (dynamic.Interface, Discovery, *rest.Config, error)) Option {
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
		clientCreator: func(cluster string) (dynamic.Interface, Discovery, *rest.Config, error) {
			config, ok := clusterConfig[cluster]
			if !ok {
				return nil, nil, nil, fmt.Errorf("no config for cluster %s", cluster)
			}

			if cluster == "management" && config == nil {
				var err error
				config, err = rest.InClusterConfig()
				if err != nil {
					return nil, nil, nil, fmt.Errorf("creating in-cluster config: %w", err)
				}
			}

			if config.NegotiatedSerializer == nil {
				config.NegotiatedSerializer = serializer.WithoutConversionCodecFactory{CodecFactory: schemepkg.Codecs}
			}

			restClient, err := rest.UnversionedRESTClientFor(config)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("creating REST client: %w", err)
			}

			client := dynamic.New(restClient)
			dynamicClient := discovery.NewDiscoveryClient(restClient)
			return client, dynamicClient, config, nil
		},
	}
	for _, opt := range opts {
		opt(s)
	}

	managers := map[string]*clusterManager{}

	for cluster := range clusterConfig {
		client, discovery, cfg, err := s.clientCreator(cluster)
		if err != nil {
			return nil, fmt.Errorf("creating client for cluster %s: %w", cluster, err)
		}
		mgr, err := newClusterManager(scheme, client, discovery, cfg, log.WithField("cluster", cluster))
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

func (m *Manager) GetDynamicClients() map[string]dynamic.Interface {
	clients := map[string]dynamic.Interface{}
	for cluster, mgr := range m.managers {
		clients[cluster] = mgr.client
	}

	return clients
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
