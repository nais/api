package fake

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/nais/api/internal/kubernetes"
	liberator_aiven_io_v1alpha1 "github.com/nais/liberator/pkg/apis/aiven.io/v1alpha1"
	unleash_nais_io_v1 "github.com/nais/unleasherator/api/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynfake "k8s.io/client-go/dynamic/fake"
	"sigs.k8s.io/yaml"
)

type clients struct {
	Dynamic dynamic.Interface
}

// Clients returns a new fake kubernetes clientset for each directory at root in the given directory.
// Each yaml file in the directory will be created as a resource, where resources in a "teams" directory
// will be created in a namespace with the same name as the file.
func Clients(dir fs.FS) func(cluster string) (dynamic.Interface, error) {
	scheme, err := kubernetes.NewScheme()
	if err != nil {
		panic(err)
	}

	if dir == nil {
		return func(cluster string) (dynamic.Interface, error) {
			return newDynamicClient(scheme), nil
		}
	}

	resources, err := ParseResources(scheme, dir)
	if err != nil {
		panic(err)
	}

	ret := make(map[string]clients)

	for cluster, objs := range resources {
		ret[cluster] = clients{
			Dynamic: newDynamicClient(scheme, objs...),
		}
	}

	return func(cluster string) (dynamic.Interface, error) {
		c, ok := ret[cluster]
		if !ok {
			fmt.Println("no fake client for cluster", cluster)
			return newDynamicClient(scheme), nil
			// return nil, fmt.Errorf("no fake client for cluster %s", cluster)
		}

		return c.Dynamic, nil
	}
}

func parseCluster(path string) string {
	p := strings.SplitN(path, string(os.PathSeparator), 2)
	if len(p) == 0 {
		return ""
	}

	return p[0]
}

func ParseResources(scheme *runtime.Scheme, dir fs.FS) (map[string][]runtime.Object, error) {
	resources := make(map[string][]runtime.Object)
	err := fs.WalkDir(dir, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if filepath.Ext(path) != ".yaml" {
			return nil
		}

		cluster := parseCluster(path)
		if cluster == "" {
			return nil
		}

		resources[cluster] = append(resources[cluster], parseResources(scheme, dir, path)...)

		return nil
	})

	return resources, err
}

func parseResources(scheme *runtime.Scheme, dir fs.FS, path string) []runtime.Object {
	b, err := fs.ReadFile(dir, path)
	if err != nil {
		panic(err.Error())
	}
	parts := bytes.Split(b, []byte("\n---"))
	ns := strings.Trim(filepath.Base(filepath.Dir(path)), string(filepath.Separator))

	ret := []runtime.Object{}
	for _, p := range parts {
		if len(bytes.TrimSpace(p)) == 0 {
			continue
		}

		v := ParseResource(scheme, p, ns)

		ret = append(ret, v)
	}

	return ret
}

func ParseResource(scheme *runtime.Scheme, b []byte, ns string) (v runtime.Object) {
	r := &unstructured.Unstructured{}
	if err := yaml.Unmarshal(b, &r); err != nil {
		panic(err)
	}

	kt := scheme.KnownTypes(r.GetObjectKind().GroupVersionKind().GroupVersion())
	if kt == nil {
		panic(fmt.Errorf("unknown group version: %q", r.GetObjectKind().GroupVersionKind().GroupVersion()))
	}

	if ns != "" {
		r.SetNamespace(ns)
		lbls := r.GetLabels()
		if lbls == nil {
			lbls = make(map[string]string)
		}
		lbls["team"] = ns
		r.SetLabels(lbls)
	}

	if t, ok := kt[r.GetObjectKind().GroupVersionKind().Kind]; ok {
		v = reflect.New(t).Interface().(runtime.Object)
		if err := scheme.Convert(r, v, nil); err != nil {
			panic(err)
		}
		v.GetObjectKind().SetGroupVersionKind(r.GetObjectKind().GroupVersionKind())

	} else {
		panic(fmt.Errorf("unknown kind: %q", r.GetObjectKind().GroupVersionKind()))
	}
	return v
}

// This is a hack around how k8s unsafeGuesses resource plurals
func depluralized(s string) string {
	switch s {
	case "redises":
		return "redis"
	case "opensearchs", "opensearches":
		return "opensearches"
	case "unleashs":
		return "unleashes"
	case "remoteunleashs":
		return "remoteunleashes"
	}

	return s
}

func NewDynamicClient(scheme *runtime.Scheme) *dynfake.FakeDynamicClient {
	return dynfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{
			liberator_aiven_io_v1alpha1.GroupVersion.WithResource("redis"):        "RedisList",
			liberator_aiven_io_v1alpha1.GroupVersion.WithResource("opensearches"): "OpenSearchList",
			unleash_nais_io_v1.GroupVersion.WithResource("unleashes"):             "UnleashList",
			unleash_nais_io_v1.GroupVersion.WithResource("remoteunleashes"):       "RemoteUnleashList",
		})
}

func AddObjectToDynamicClient(scheme *runtime.Scheme, fc *dynfake.FakeDynamicClient, objs ...runtime.Object) {
	type namespaced interface {
		GetNamespace() string
	}

	for _, obj := range objs {
		gvks, _, err := scheme.ObjectKinds(obj)
		if err != nil {
			panic(err)
		}

		if len(gvks) == 0 {
			panic(fmt.Errorf("no registered kinds for %v", obj))
		}
		for _, gvk := range gvks {
			gvr, _ := meta.UnsafeGuessKindToResource(gvk)

			gvr.Resource = depluralized(gvr.Resource)
			// Get namespace from object
			ns := obj.(namespaced).GetNamespace()
			fc.Tracker().Create(gvr, obj, ns)
		}
	}
}

func newDynamicClient(scheme *runtime.Scheme, objs ...runtime.Object) dynamic.Interface {
	fc := NewDynamicClient(scheme)
	AddObjectToDynamicClient(scheme, fc, objs...)
	return fc
}
