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
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd/api"
)

const PrometheusUrl = "https://nais-prometheus.%s.cloud.nais.io"

type Manager struct {
	tenantClusters clusterClients
	mgmCluster     *k8sClient
	mgmtNamespace  string
	prometheus     Prometheus
	bifrostClient  BifrostClient
	settings       *settings
	log            logrus.FieldLogger
}

type Config struct {
	Enabled       bool
	Namespace     string
	BifrostApiUrl string
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
		unleashEnabled bool
	}
)

type Opt func(*settings)

func WithClientsCreator(f func(cluster string) (kubernetes.Interface, dynamic.Interface, error)) Opt {
	return func(s *settings) {
		s.clientsCreator = f
	}
}

func NewManager(tenant string, clusters []string, config Config, log logrus.FieldLogger, opts ...Opt) (*Manager, error) {
	s := &settings{}
	for _, opt := range opts {
		opt(s)
	}

	s.unleashEnabled = config.Enabled

	clientMap, err := tenantClusters(tenant, clusters, opts...)
	if err != nil {
		return nil, err
	}

	mgmt, err := mgmtCluster(config.Namespace, opts...)
	if err != nil {
		return nil, err
	}

	m := &Manager{
		mgmtNamespace:  config.Namespace,
		tenantClusters: clientMap,
		mgmCluster:     mgmt,
		settings:       s,
		log:            log,
	}

	m.bifrostClient = NewBifrostClient(config.BifrostApiUrl, log)

	promClient, err := promapi.NewClient(promapi.Config{
		Address: fmt.Sprintf(PrometheusUrl, tenant),
	})
	if err != nil {
		return nil, err
	}

	m.prometheus = promv1.NewAPI(promClient)
	// if clientsCreator is set, it means that faking is enabled. should probably send in the flag itself to avoid this comment
	if s.clientsCreator != nil {
		m.bifrostClient = NewFakeBifrostClient(mgmt.dynamicClient)
		m.prometheus = NewFakePrometheusClient()
	}

	return m, nil
}

func (m Manager) Start(ctx context.Context, log logrus.FieldLogger) error {
	if !m.settings.unleashEnabled {
		log.Info("unleash is disabled, skipping informers")
		return nil
	}

	for _, informer := range m.mgmCluster.informers {
		log.WithField("cluster", "management").WithField("informer", "unleash").Info("started informer")
		go informer.Informer().Run(ctx.Done())
	}

	for _, informer := range m.mgmCluster.informers {
		if err := hasSynced(ctx, "management", informer, log); err != nil {
			return err
		}
	}
	return nil
}

func mgmtCluster(namespace string, opts ...Opt) (*k8sClient, error) {
	s := settings{}
	for _, opt := range opts {
		opt(&s)
	}

	client, dynamicClient, err := createClients(
		"",
		"management",
		opts...,
	)
	if err != nil {
		return nil, err
	}

	dinf := dynamicinformer.NewFilteredDynamicSharedInformerFactory(dynamicClient, 4*time.Hour, namespace, nil)

	infs := make([]informers.GenericInformer, 0)
	infs = append(infs, dinf.ForResource(unleash_nais_io_v1.GroupVersion.WithResource("unleashes")))

	return &k8sClient{
		clientSet:     client,
		dynamicClient: dynamicClient,
		informers:     infs,
	}, nil
}

func tenantClusters(tenant string, clusters []string, opts ...Opt) (clusterClients, error) {
	clients := clusterClients{}
	for _, cluster := range clusters {
		client, dynamicClient, err := createClients(
			fmt.Sprintf("https://apiserver.%s.%s.cloud.nais.io", cluster, tenant),
			cluster,
			opts...,
		)
		if err != nil {
			return nil, err
		}
		clients[cluster] = &k8sClient{
			clientSet:     client,
			dynamicClient: dynamicClient,
			informers:     []informers.GenericInformer{},
		}
	}
	return clients, nil
}

func createClients(apiServer, clusterName string, opts ...Opt) (kubernetes.Interface, dynamic.Interface, error) {
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
				return nil, nil, fmt.Errorf("failed to get kubeconfig: %w", err)
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

	return s.clientsCreator(clusterName)
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
