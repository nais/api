package watcher

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/kubernetes"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
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
	clientCreator           func(cluster string) (dynamic.Interface, Discovery, *rest.Config, error)
	replaceEnvironmentNames map[string]string
}

type Option func(*settings)

func WithClientCreator(fn func(cluster string) (dynamic.Interface, Discovery, *rest.Config, error)) Option {
	return func(m *settings) {
		m.clientCreator = fn
	}
}

func WithReplaceEnvironmentNames(m map[string]string) Option {
	return func(s *settings) {
		s.replaceEnvironmentNames = m
	}
}

type Manager struct {
	managers                map[string]*clusterManager
	scheme                  *runtime.Scheme
	log                     logrus.FieldLogger
	replaceEnvironmentNames map[string]string

	cacheSyncs      []cache.InformerSynced
	resourceCounter metric.Int64UpDownCounter
}

func NewManager(scheme *runtime.Scheme, clusterConfig kubernetes.ClusterConfigMap, log logrus.FieldLogger, opts ...Option) (*Manager, error) {
	meter := otel.GetMeterProvider().Meter("nais_api_watcher")
	udCounter, err := meter.Int64UpDownCounter("nais_api_watcher_resources", metric.WithDescription("Number of resources watched by the watcher"))
	if err != nil {
		return nil, fmt.Errorf("creating resources counter: %w", err)
	}

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

			config.UserAgent = "nais.io/api"
			client, err := dynamic.NewForConfig(config)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("creating REST client: %w", err)
			}
			dynamicClient, err := discovery.NewDiscoveryClientForConfig(config)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("creating discovery client: %w", err)
			}
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
		scheme:                  scheme,
		managers:                managers,
		log:                     log,
		resourceCounter:         udCounter,
		replaceEnvironmentNames: s.replaceEnvironmentNames,
	}, nil
}

func (m *Manager) Stop() {
	for _, mgr := range m.managers {
		if mgr.createdInformer != nil {
			mgr.createdInformer.Shutdown()
		}
		for _, inf := range mgr.createdFilteredInformers {
			inf.Shutdown()
		}
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
