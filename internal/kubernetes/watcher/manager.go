package watcher

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/nais/api/internal/kubernetes"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	schemepkg "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/cache"
)

type cacheSyncEntry struct {
	cluster string
	gvr     string
	synced  cache.InformerSynced
}

type settings struct {
	clientCreator func(cluster string) (dynamic.Interface, KindResolver, *rest.Config, error)
}

type KindResolver interface {
	KindsFor(resource schema.GroupVersionResource) ([]schema.GroupVersionKind, error)
}

type Option func(*settings)

func WithClientCreator(fn func(cluster string) (dynamic.Interface, KindResolver, *rest.Config, error)) Option {
	return func(m *settings) {
		m.clientCreator = fn
	}
}

type Manager struct {
	managers map[string]*clusterManager
	scheme   *runtime.Scheme
	log      logrus.FieldLogger

	cacheSyncs      []cache.InformerSynced
	cacheSyncInfo   []cacheSyncEntry
	ready           atomic.Bool
	resourceCounter metric.Int64UpDownCounter
}

func NewManager(scheme *runtime.Scheme, clusterConfig kubernetes.ClusterConfigMap, log logrus.FieldLogger, opts ...Option) (*Manager, error) {
	meter := otel.GetMeterProvider().Meter("nais_api_watcher")
	udCounter, err := meter.Int64UpDownCounter("nais_api_watcher_resources", metric.WithDescription("Number of resources watched by the watcher"))
	if err != nil {
		return nil, fmt.Errorf("creating resources counter: %w", err)
	}

	s := &settings{
		clientCreator: func(cluster string) (dynamic.Interface, KindResolver, *rest.Config, error) {
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
			discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("creating discovery client: %w", err)
			}
			cachedDiscoveryClient := memory.NewMemCacheClient(discoveryClient)

			return client, restmapper.NewDeferredDiscoveryRESTMapper(cachedDiscoveryClient), config, nil
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
		scheme:          scheme,
		managers:        managers,
		log:             log,
		resourceCounter: udCounter,
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
	doneCh := make(chan bool, 1)
	go func() {
		doneCh <- cache.WaitForCacheSync(ctx.Done(), m.cacheSyncs...)
	}()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case ok := <-doneCh:
			if ok {
				m.ready.Store(true)
			} else {
				m.logUnsynced("cache sync failed/cancelled, still unsynced")
			}
			return ok
		case <-ticker.C:
			m.logUnsynced("still waiting for cache sync")
		}
	}
}

func (m *Manager) logUnsynced(msg string) {
	for _, e := range m.cacheSyncInfo {
		if !e.synced() {
			m.log.WithFields(logrus.Fields{
				"cluster": e.cluster,
				"gvr":     e.gvr,
			}).Warn(msg)
		}
	}
}

// IsReady returns true if all informer caches have been synced.
// This is a non-blocking check that inspects the current state of every
// registered informer, so it will start reporting true as soon as the last
// outstanding cache finishes syncing — even if the initial WaitForReady call
// timed out.
func (m *Manager) IsReady() bool {
	if m.ready.Load() {
		return true
	}
	for _, e := range m.cacheSyncInfo {
		if !e.synced() {
			return false
		}
	}
	m.ready.Store(true)
	return true
}

func (m *Manager) GetDynamicClients() map[string]dynamic.Interface {
	clients := map[string]dynamic.Interface{}
	for cluster, mgr := range m.managers {
		clients[cluster] = mgr.client
	}

	return clients
}

func (m *Manager) ResourceMappers() map[string]KindResolver {
	clients := map[string]KindResolver{}
	for cluster, mgr := range m.managers {
		clients[cluster] = mgr.resourceMapper
	}

	return clients
}

func (m *Manager) addCacheSync(cluster, gvr string, sync cache.InformerSynced) {
	m.cacheSyncs = append(m.cacheSyncs, sync)
	m.cacheSyncInfo = append(m.cacheSyncInfo, cacheSyncEntry{
		cluster: cluster,
		gvr:     gvr,
		synced:  sync,
	})
}

func Watch[T Object](mgr *Manager, obj T, opts ...WatchOption) *Watcher[T] {
	settings := &watcherSettings{}
	for _, opt := range opts {
		opt(settings)
	}
	return newWatcher(mgr, obj, settings, mgr.log)
}
