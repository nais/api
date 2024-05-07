package unleash

import (
	"fmt"
	"github.com/nais/api/internal/k8s"
	unleash_nais_io_v1 "github.com/nais/unleasherator/api/v1"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd/api"
	"net/http"
	"time"
)

type k8sClient struct {
	clientSet     kubernetes.Interface
	dynamicClient dynamic.Interface
	informers     []informers.GenericInformer
}

type clusterClients map[string]*k8sClient
type settings struct {
	clientsCreator func(cluster string) (kubernetes.Interface, dynamic.Interface, error)
}

type Opt func(*settings)

func WithClientsCreator(f func(cluster string) (kubernetes.Interface, dynamic.Interface, error)) Opt {
	return func(s *settings) {
		s.clientsCreator = f
	}
}

// GCP only, need static config for onprem
func createClientMap(tenant string, clusters []string, opts ...Opt) (clusterClients, error) {
	s := &settings{}
	for _, opt := range opts {
		opt(s)
	}
	clients := clusterClients{}

	for _, cluster := range clusters {
		restConfig := rest.Config{
			// TODO: does this dns exist for management?
			Host: fmt.Sprintf("https://apiserver.%s.%s.cloud.nais.io", cluster, tenant),
			AuthProvider: &api.AuthProviderConfig{
				Name: k8s.GoogleAuthPlugin,
			},
			WrapTransport: func(rt http.RoundTripper) http.RoundTripper {
				return otelhttp.NewTransport(rt, otelhttp.WithServerName(cluster))
			},
		}
		if s.clientsCreator == nil {
			s.clientsCreator = func(cluster string) (kubernetes.Interface, dynamic.Interface, error) {
				clientSet, err := kubernetes.NewForConfig(&restConfig)
				if err != nil {
					return nil, nil, fmt.Errorf("create clientset: %w", err)
				}

				dynamicClient, err := dynamic.NewForConfig(&restConfig)
				if err != nil {
					return nil, nil, fmt.Errorf("create dynamic client: %w", err)
				}
				return clientSet, dynamicClient, nil
			}
		}

		clientSet, dynamicClient, err := s.clientsCreator(cluster)
		if err != nil {
			return nil, fmt.Errorf("create clientsets: %w", err)
		}

		clients[cluster] = &k8sClient{
			clientSet:     clientSet,
			dynamicClient: dynamicClient,
			informers:     createInformers(dynamicClient),
		}
	}

	return clients, nil
}

func createInformers(dynamicClient dynamic.Interface) []informers.GenericInformer {
	dinf := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, 4*time.Hour)
	infs := make([]informers.GenericInformer, 0)
	infs = append(infs, dinf.ForResource(unleash_nais_io_v1.GroupVersion.WithResource("unleashes")))
	return infs
}
