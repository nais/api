package fake

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/watcher"
	liberator_aiven_io_v1alpha1 "github.com/nais/liberator/pkg/apis/aiven.io/v1alpha1"
	data_nais_io_v1 "github.com/nais/liberator/pkg/apis/data.nais.io/v1"
	unleash_nais_io_v1 "github.com/nais/unleasherator/api/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	dynfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/rest"
	k8stesting "k8s.io/client-go/testing"
	"sigs.k8s.io/yaml"
)

type clients struct {
	Dynamic dynamic.Interface
}

type FakeKindsResolver struct {
	scheme *runtime.Scheme
}

var _ watcher.KindResolver = &FakeKindsResolver{}

func (f *FakeKindsResolver) KindsFor(resource schema.GroupVersionResource) ([]schema.GroupVersionKind, error) {
	kt := f.scheme.KnownTypes(resource.GroupVersion())
	if kt == nil {
		return nil, fmt.Errorf("unknown group version: %q", resource.GroupVersion())
	}

	for v := range kt {
		gvr, _ := meta.UnsafeGuessKindToResource(resource.GroupVersion().WithKind(v))
		gvr.Resource = depluralized(gvr.Resource)
		if gvr.Group == resource.Group && gvr.Version == resource.Version && gvr.Resource == resource.Resource {
			// This is a match
			gvk := resource.GroupVersion().WithKind(v)
			return []schema.GroupVersionKind{gvk}, nil
		}
	}
	return nil, fmt.Errorf("unknown group version: %q", resource.GroupVersion())
}

// Clients returns a new fake kubernetes clientset for each directory at root in the given directory.
// Each yaml file in the directory will be created as a resource, where resources in a "teams" directory
// will be created in a namespace with the same name as the file.
func Clients(dir fs.FS) func(cluster string) (dynamic.Interface, watcher.KindResolver, *rest.Config, error) {
	scheme, err := kubernetes.NewScheme()
	if err != nil {
		panic(err)
	}

	restMapper := &FakeKindsResolver{
		scheme: scheme,
	}

	if dir == nil {
		return func(cluster string) (dynamic.Interface, watcher.KindResolver, *rest.Config, error) {
			return newDynamicClient(scheme), restMapper, nil, nil
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

	return func(cluster string) (dynamic.Interface, watcher.KindResolver, *rest.Config, error) {
		c, ok := ret[cluster]
		if !ok {
			fmt.Println("no fake client for cluster", cluster)
			return newDynamicClient(scheme), restMapper, nil, nil
			// return nil, fmt.Errorf("no fake client for cluster %s", cluster)
		}

		return c.Dynamic, restMapper, nil, nil
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

	return r
}

// This is a hack around how k8s unsafeGuesses resource plurals
func depluralized(s string) string {
	switch s {
	case "valkeies":
		return "valkeys"
	case "opensearchs", "opensearches":
		return "opensearches"
	case "unleashs":
		return "unleashes"
	case "remoteunleashs":
		return "remoteunleashes"
	case "postgreses":
		return "postgres"
	}

	return s
}

func NewDynamicClient(scheme *runtime.Scheme) *dynfake.FakeDynamicClient {
	newScheme := runtime.NewScheme()
	for gvk := range scheme.AllKnownTypes() {
		if newScheme.Recognizes(gvk) {
			continue
		}
		// Ensure we are always supporting unstructured objects
		// This to prevent various problems with the fake client
		if strings.HasSuffix(gvk.Kind, "List") {
			newScheme.AddKnownTypeWithName(gvk, &unstructured.UnstructuredList{})
			continue
		}
		newScheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
	}

	client := dynfake.NewSimpleDynamicClientWithCustomListKinds(newScheme,
		map[schema.GroupVersionResource]string{
			liberator_aiven_io_v1alpha1.GroupVersion.WithResource("valkeys"):      "ValkeyList",
			liberator_aiven_io_v1alpha1.GroupVersion.WithResource("opensearches"): "OpenSearchList",
			unleash_nais_io_v1.GroupVersion.WithResource("unleashes"):             "UnleashList",
			unleash_nais_io_v1.GroupVersion.WithResource("remoteunleashes"):       "RemoteUnleashList",
			data_nais_io_v1.GroupVersion.WithResource("postgres"):                 "PostgresList",
		})

	// Add reactor for JSON Patch support
	client.PrependReactor("patch", "*", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		patchAction, ok := action.(k8stesting.PatchAction)
		if !ok {
			return false, nil, nil
		}

		// Only handle JSON Patch type
		if patchAction.GetPatchType() != types.JSONPatchType {
			return false, nil, nil
		}

		// Get the existing object from the tracker
		gvr := patchAction.GetResource()
		ns := patchAction.GetNamespace()
		name := patchAction.GetName()

		obj, err := client.Tracker().Get(gvr, ns, name)
		if err != nil {
			return true, nil, err
		}

		// Get the original object as unstructured to preserve apiVersion/kind
		original, ok := obj.(*unstructured.Unstructured)
		if !ok {
			return true, nil, fmt.Errorf("expected *unstructured.Unstructured, got %T", obj)
		}

		// Convert to JSON
		objJSON, err := json.Marshal(obj)
		if err != nil {
			return true, nil, fmt.Errorf("marshaling object: %w", err)
		}

		// Apply the JSON patch
		patch, err := jsonpatch.DecodePatch(patchAction.GetPatch())
		if err != nil {
			return true, nil, fmt.Errorf("decoding patch: %w", err)
		}

		modifiedJSON, err := patch.Apply(objJSON)
		if err != nil {
			return true, nil, fmt.Errorf("applying patch: %w", err)
		}

		// Convert back to unstructured
		modified := &unstructured.Unstructured{}
		if err := json.Unmarshal(modifiedJSON, &modified.Object); err != nil {
			return true, nil, fmt.Errorf("unmarshaling modified object: %w", err)
		}

		// Preserve apiVersion and kind from original object (JSON patch may not include them)
		if modified.GetAPIVersion() == "" {
			modified.SetAPIVersion(original.GetAPIVersion())
		}
		if modified.GetKind() == "" {
			modified.SetKind(original.GetKind())
		}

		// Update the object in the tracker
		if err := client.Tracker().Update(gvr, modified, ns); err != nil {
			return true, nil, fmt.Errorf("updating object: %w", err)
		}

		return true, modified, nil
	})

	return client
}

func AddObjectToDynamicClient(scheme *runtime.Scheme, fc *dynfake.FakeDynamicClient, objs ...runtime.Object) {
	type namespaced interface {
		GetNamespace() string
	}

	for _, obj := range objs {
		if obj.GetObjectKind().GroupVersionKind().Kind == "List" {
			list := obj.(*unstructured.Unstructured)
			ul, err := list.ToList()
			if err != nil {
				panic(err)
			}
			for _, item := range ul.Items {
				AddObjectToDynamicClient(scheme, fc, &item)
			}
			continue
		}

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
			if err := fc.Tracker().Create(gvr, obj, ns); err != nil {
				panic(err)
			}
		}
	}
}

func newDynamicClient(scheme *runtime.Scheme, objs ...runtime.Object) dynamic.Interface {
	fc := NewDynamicClient(scheme)
	AddObjectToDynamicClient(scheme, fc, objs...)
	return fc
}
