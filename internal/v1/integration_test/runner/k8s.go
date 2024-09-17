package runner

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	v1kube "github.com/nais/api/internal/v1/kubernetes"
	"github.com/nais/api/internal/v1/kubernetes/fake"
	"github.com/nais/tester/testmanager"
	"github.com/nais/tester/testmanager/parser"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	dynfake "k8s.io/client-go/dynamic/fake"
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
		scheme, err := v1kube.NewScheme()
		if err != nil {
			return nil, err
		}

		v := fake.ParseResource(scheme, []byte(f.Returns), "")

		k.lock.Lock()
		defer k.lock.Unlock()

		clients, ok := k.clusters[data.Cluster]
		if !ok {
			return nil, fmt.Errorf("cluster %q not found", data.Cluster)
		}

		u := v.(named)

		clients.Dynamic.Resource(v.GetObjectKind().GroupVersionKind().GroupVersion().WithResource(v.GetObjectKind().GroupVersionKind().Kind)).Namespace(data.Namespace).Get(ctx, u.GetName(), metav1.GetOptions{})

		return u, nil
	})
}

func (k *K8s) K8sPath(ctx context.Context) string {
	return filepath.Join("testdata", "tests", testmanager.TestDir(ctx), "k8s")
}
