package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/nais/api/internal/k8s/fake"
	"github.com/nais/tester/testmanager"
	"github.com/nais/tester/testmanager/parser"
	"github.com/pingcap/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	dynfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/yaml"
)

type named interface {
	GetName() string
}

type clients struct {
	ClientSet *k8sfake.Clientset
	Dynamic   *dynfake.FakeDynamicClient
}

type K8s struct {
	lock     sync.Mutex
	clusters map[string]clients
}

func NewK8sRunner() *K8s {
	return &K8s{
		clusters: make(map[string]clients),
	}
}

func (k *K8s) Ext() string {
	return "k8s"
}

func (k *K8s) Run(ctx context.Context, logf func(format string, args ...any), body []byte, state map[string]any) error {
	f, err := parser.Parse(body, state)
	if err != nil {
		return fmt.Errorf("gql.Parse: %w", err)
	}

	data := &struct {
		Cluster   string `yaml:"cluster"`
		Namespace string `yaml:"namespace"`
	}{}

	if err := yaml.Unmarshal([]byte(f.Query), data); err != nil {
		return fmt.Errorf("yaml.Unmarshal: %w", err)
	}

	return f.Execute(state, func() (any, error) {
		scheme := fake.NewScheme()

		v, core := fake.ParseResource(scheme, []byte(f.Returns), "")

		k.lock.Lock()
		defer k.lock.Unlock()

		clients, ok := k.clusters[data.Cluster]
		if !ok {
			return nil, fmt.Errorf("cluster %q not found", data.Cluster)
		}

		u := v.(named)
		if core {
			var obj any
			switch v.GetObjectKind().GroupVersionKind().Kind {
			case "Secret":
				obj, err = clients.ClientSet.CoreV1().Secrets(data.Namespace).Get(ctx, u.GetName(), metav1.GetOptions{})
				if err != nil {
					if errors.IsNotFound(err) {
						return nil, fmt.Errorf("secret %q not found in %q", u.GetName(), data.Namespace)
					}
					return nil, err
				}
			default:
				return nil, fmt.Errorf("unsupported kind %q", v.GetObjectKind().GroupVersionKind().Kind)
			}

			ret := map[string]any{}
			b, err := yaml.Marshal(obj)
			if err != nil {
				return nil, err
			}

			if err := yaml.Unmarshal(b, &ret); err != nil {
				return nil, err
			}

			return ret, nil
		} else {
			clients.Dynamic.Resource(v.GetObjectKind().GroupVersionKind().GroupVersion().WithResource(v.GetObjectKind().GroupVersionKind().Kind)).Namespace(data.Namespace).Get(ctx, u.GetName(), metav1.GetOptions{})
		}

		return u, nil
	})
}

func (k *K8s) ClientsCreator(ctx context.Context, t *testing.T) func(cluster string) (kubernetes.Interface, dynamic.Interface, error) {
	fn := fake.Clients(os.DirFS(k.K8sPath(ctx)))

	return func(cluster string) (kubernetes.Interface, dynamic.Interface, error) {
		c1, c2, err := fn(cluster)
		if err != nil {
			return nil, nil, err
		}

		k.lock.Lock()
		defer k.lock.Unlock()

		k.clusters[cluster] = clients{
			ClientSet: c1.(*k8sfake.Clientset),
			Dynamic:   c2.(*dynfake.FakeDynamicClient),
		}

		return c1, c2, nil
	}
}

func (k *K8s) K8sPath(ctx context.Context) string {
	return filepath.Join("testdata", "tests", testmanager.TestDir(ctx), "k8s")
}
