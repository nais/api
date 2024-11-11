package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/nais/api/internal/kubernetes/fake"
	"github.com/nais/tester/lua/runner"
	"github.com/nais/tester/lua/spec"
	lua "github.com/yuin/gopher-lua"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/rest"
)

var _ spec.Runner = &K8s{}

type K8s struct {
	Scheme *runtime.Scheme

	lock    sync.Mutex
	clients map[string]*dynfake.FakeDynamicClient
	rootDir string
}

func NewK8sRunner(scheme *runtime.Scheme, rootDir string, clusters []string) *K8s {
	clients := map[string]*dynfake.FakeDynamicClient{
		"management": fake.NewDynamicClient(scheme),
	}
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

func (k *K8s) ClientCreator(cluster string) (dynamic.Interface, *rest.Config, error) {
	c, ok := k.clients[cluster]
	if !ok {
		return nil, nil, fmt.Errorf("cluster %q not found", cluster)
	}
	return c, nil, nil
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

	// Allow time for informers to sync
	time.Sleep(10 * time.Millisecond)

	return 0
}
