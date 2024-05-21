package unleash

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/nais/api/internal/k8s"
	unleash_nais_io_v1 "github.com/nais/unleasherator/api/v1"
	promapi "github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd/api"
)

type Manager struct {
	tenantClusters clusterClients
	mgmCluster     *k8sClient
	mgmtNamespace  string
	prometheus     promv1.API
	bifrostClient  BifrostClient
}

type k8sClient struct {
	clientSet     kubernetes.Interface
	dynamicClient dynamic.Interface
	informers     []informers.GenericInformer
}

type (
	clusterClients map[string]*k8sClient
	settings       struct {
		clientsCreator func(cluster string) (kubernetes.Interface, dynamic.Interface, error)
		bifrostEnabled bool
	}
)

type Opt func(*settings)

func WithClientsCreator(f func(cluster string) (kubernetes.Interface, dynamic.Interface, error)) Opt {
	return func(s *settings) {
		s.clientsCreator = f
	}
}

func WithBifrostEnabled() Opt {
	return func(s *settings) {
		s.bifrostEnabled = true
	}
}

func NewManager(tenant, namespace string, clusters []string, opts ...Opt) (*Manager, error) {
	s := &settings{}
	for _, opt := range opts {
		opt(s)
	}

	clientMap, err := tenantClusters(tenant, clusters, opts...)
	if err != nil {
		return nil, err
	}

	mgmt, err := mgmtCluster(opts...)
	if err != nil {
		return nil, err
	}

	promClient, err := promapi.NewClient(promapi.Config{
		Address: fmt.Sprintf("https://nais-prometheus.%s.cloud.nais.io", tenant),
	})
	if err != nil {
		return nil, err
	}

	// TODO: setup logger outside of this package
	bifrost := NewBifrostClient("", logrus.New())
	// if clientsCreator is set, it means that faking is enabled. should probably send in the flag itself to avoid this comment
	if s.clientsCreator != nil {
		bifrost = NewFakeBifrostClient(mgmt.dynamicClient)
	}

	return &Manager{
		mgmtNamespace:  namespace,
		tenantClusters: clientMap,
		mgmCluster:     mgmt,
		prometheus:     promv1.NewAPI(promClient),
		bifrostClient:  bifrost,
	}, nil
}

func (m Manager) Start(ctx context.Context, log logrus.FieldLogger) error {
	for cluster, informers := range m.tenantClusters {
		log.WithField("cluster", cluster).Infof("starting informers")
		for _, informer := range informers.informers {
			go informer.Informer().Run(ctx.Done())
		}
	}

	log.WithField("cluster", "management").Infof("starting informers")
	for _, informer := range m.mgmCluster.informers {
		go informer.Informer().Run(ctx.Done())
	}

	for env, informers := range m.tenantClusters {
		for _, informer := range informers.informers {
			if err := hasSynced(ctx, env, informer, log); err != nil {
				return err
			}
		}
	}

	for _, informer := range m.mgmCluster.informers {
		if err := hasSynced(ctx, "management", informer, log); err != nil {
			return err
		}
	}
	return nil
}

func mgmtCluster(opts ...Opt) (*k8sClient, error) {
	s := settings{}
	for _, opt := range opts {
		opt(&s)
	}
	if s.bifrostEnabled {
		return createClient(
			"",
			"management",
			[]schema.GroupVersionResource{
				unleash_nais_io_v1.GroupVersion.WithResource("unleashes"),
			},
			opts...,
		)
	}

	return createClient(
		"",
		"management",
		[]schema.GroupVersionResource{},
		opts...,
	)
}

func tenantClusters(tenant string, clusters []string, opts ...Opt) (clusterClients, error) {
	clients := clusterClients{}
	for _, cluster := range clusters {
		c, err := createClient(
			fmt.Sprintf("https://apiserver.%s.%s.cloud.nais.io", cluster, tenant),
			cluster,
			[]schema.GroupVersionResource{
				unleash_nais_io_v1.GroupVersion.WithResource("remoteunleashes"),
			},
			opts...,
		)
		if err != nil {
			return nil, err
		}
		clients[cluster] = c
	}
	return clients, nil
}

func createClient(apiServer, clusterName string, resources []schema.GroupVersionResource, opts ...Opt) (*k8sClient, error) {
	s := &settings{}
	for _, opt := range opts {
		opt(s)
	}

	restConfig := &rest.Config{
		Host: apiServer,
		AuthProvider: &api.AuthProviderConfig{
			Name: k8s.GoogleAuthPlugin,
		},
		WrapTransport: func(rt http.RoundTripper) http.RoundTripper {
			return otelhttp.NewTransport(rt, otelhttp.WithServerName(clusterName))
		},
	}

	if s.clientsCreator == nil {
		var err error
		if clusterName == "management" {
			restConfig, err = rest.InClusterConfig()
			if err != nil {
				return nil, fmt.Errorf("failed to get kubeconfig: %w", err)
			}
		}

		s.clientsCreator = func(cluster string) (kubernetes.Interface, dynamic.Interface, error) {
			clientSet, err := kubernetes.NewForConfig(restConfig)
			if err != nil {
				return nil, nil, fmt.Errorf("create clientset: %w", err)
			}

			dynamicClient, err := dynamic.NewForConfig(restConfig)
			if err != nil {
				return nil, nil, fmt.Errorf("create dynamic client: %w", err)
			}
			return clientSet, dynamicClient, nil
		}
	}

	clientSet, dynamicClient, err := s.clientsCreator(clusterName)
	if err != nil {
		return nil, fmt.Errorf("create clientsets: %w", err)
	}

	return &k8sClient{
		clientSet:     clientSet,
		dynamicClient: dynamicClient,
		informers:     createInformers(dynamicClient, resources),
	}, nil
}

// @TODO: use namespace from config
func createInformers(dynamicClient dynamic.Interface, resources []schema.GroupVersionResource) []informers.GenericInformer {
	dinf := dynamicinformer.NewFilteredDynamicSharedInformerFactory(dynamicClient, 4*time.Hour, "bifrost-unleash", nil)

	infs := make([]informers.GenericInformer, 0)
	for _, resources := range resources {
		infs = append(infs, dinf.ForResource(resources))
	}
	return infs
}

func hasSynced(ctx context.Context, cluster string, informer informers.GenericInformer, log logrus.FieldLogger) error {
	for !informer.Informer().HasSynced() {
		log.Infof("waiting for informer in " + cluster + " to sync")

		select {
		case <-ctx.Done():
			return fmt.Errorf("informers not started: %w", ctx.Err())
		default:
			time.Sleep(2 * time.Second)
		}
	}
	return nil
}
