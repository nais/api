package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/nais/api/internal/v1/kubernetes/fake"
	"github.com/nais/tester/lua/runner"
	"github.com/nais/tester/lua/spec"
	lua "github.com/yuin/gopher-lua"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynfake "k8s.io/client-go/dynamic/fake"
)

var _ spec.Runner = &K8s{}

type K8s struct {
	Scheme *runtime.Scheme

	lock    sync.Mutex
	clients map[string]*dynfake.FakeDynamicClient
	rootDir string
}

func NewK8sRunner(scheme *runtime.Scheme, rootDir string, clusters []string) *K8s {
	clients := make(map[string]*dynfake.FakeDynamicClient, len(clusters))
	for _, cluster := range clusters {
		clients[cluster] = fake.NewDynamicClient(scheme)
	}

	return &K8s{
		Scheme:  scheme,
		clients: clients,
		rootDir: rootDir,
	}
}

func (k *K8s) Name() string {
	return "k8s"
}

func (k *K8s) Functions() []*spec.Function {
	return []*spec.Function{
		{
			Name: "check",
			Args: []spec.Argument{
				{Name: "apiVersion", Type: []spec.ArgumentType{spec.ArgumentTypeString}, Doc: "API version"},
				{Name: "kind", Type: []spec.ArgumentType{spec.ArgumentTypeString}, Doc: "Kind"},
				{Name: "cluster", Type: []spec.ArgumentType{spec.ArgumentTypeString}, Doc: "Cluster name"},
				{Name: "namespace", Type: []spec.ArgumentType{spec.ArgumentTypeString}, Doc: "Namespace / team name"},
				{Name: "name", Type: []spec.ArgumentType{spec.ArgumentTypeString}, Doc: "Resource name"},
				{Name: "resp", Type: []spec.ArgumentType{spec.ArgumentTypeTable}, Doc: "Response to match against"},
			},
			Doc:  "Check if a resource exists in a cluster",
			Func: k.check,
		},
	}
}

func (k *K8s) HelperFunctions() []*spec.Function {
	return []*spec.Function{
		{
			Name: "readK8sResources",
			Args: []spec.Argument{
				{Name: "dir", Type: []spec.ArgumentType{spec.ArgumentTypeString}, Doc: "Directory containing k8s resources"},
			},
			Doc:  "Read in k8s resources from a directory",
			Func: k.readK8sResources,
		},
	}
}

func (k *K8s) ClientCreator(cluster string) (dynamic.Interface, error) {
	c, ok := k.clients[cluster]
	if !ok {
		return nil, fmt.Errorf("cluster %q not found", cluster)
	}
	return c, nil
}

func (k *K8s) check(L *lua.LState) int {
	apiVersion := L.CheckString(1)
	kind := L.CheckString(2)
	cluster := L.CheckString(3)
	namespace := L.CheckString(4)
	name := L.CheckString(5)
	resp := L.CheckTable(6)

	k.lock.Lock()
	defer k.lock.Unlock()

	clients, ok := k.clients[cluster]
	if !ok {
		L.RaiseError("cluster %q not found", cluster)
	}

	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		L.RaiseError("parse group version: %v", err)
	}

	res, err := clients.Resource(gv.WithResource(kind)).Namespace(namespace).Get(L.Context(), name, metav1.GetOptions{})
	if err != nil {
		L.RaiseError("get resource: %v", err)
	}

	runner.StdCheck(L, resp, res.Object)

	return 0
}

func (k *K8s) readK8sResources(L *lua.LState) int {
	dir := L.CheckString(1)

	resources, err := fake.ParseResources(k.Scheme, os.DirFS(filepath.Join(k.rootDir, dir)))
	if err != nil {
		L.RaiseError("parse resources: %v", err)
	}

	for cluster, objs := range resources {
		fake.AddObjectToDynamicClient(k.Scheme, k.clients[cluster], objs...)
	}

	return 0
}

// func (k *K8s) Run(ctx context.Context, logf func(format string, args ...any), body []byte, state map[string]any) error {
// 	f, err := parser.Parse(body, state)
// 	if err != nil {
// 		return fmt.Errorf("gql.Parse: %w", err)
// 	}

// 	data := &struct {
// 		Cluster   string `yaml:"cluster"`
// 		Namespace string `yaml:"namespace"`
// 	}{}

// 	if err := yaml.Unmarshal([]byte(f.Query), data); err != nil {
// 		return fmt.Errorf("yaml.Unmarshal: %w", err)
// 	}

// 	return f.Execute(state, func() (any, error) {
// 		scheme, err := v1kube.NewScheme()
// 		if err != nil {
// 			return nil, err
// 		}

// 		v := fake.ParseResource(scheme, []byte(f.Returns), "")

// 		k.lock.Lock()
// 		defer k.lock.Unlock()

// 		clients, ok := k.clusters[data.Cluster]
// 		if !ok {
// 			return nil, fmt.Errorf("cluster %q not found", data.Cluster)
// 		}

// 		u := v.(named)

// 		clients.Dynamic.Resource(v.GetObjectKind().GroupVersionKind().GroupVersion().WithResource(v.GetObjectKind().GroupVersionKind().Kind)).Namespace(data.Namespace).Get(ctx, u.GetName(), metav1.GetOptions{})

// 		return u, nil
// 	})
// }

// func (k *K8s) K8sPath(ctx context.Context) string {
// 	return filepath.Join("testdata", "tests", testmanager.TestDir(ctx), "k8s")
// }
