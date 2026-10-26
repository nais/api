package crd

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/nais/api/internal/kubernetes/watcher"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type Enum struct {
	Name    string
	CRDName string
	Version string
	Path    []string
}

var registeredEnums = make(map[string]Enum)

// RegisterEnum must be called during package initialization, before watchers start.
// It panics for invalid definitions or duplicate names.
func RegisterEnum(field Enum) {
	if field.Name == "" || field.CRDName == "" || field.Version == "" || len(field.Path) == 0 || slices.Contains(field.Path, "") {
		panic("CRD enum registration requires a name, definition, version, and property path")
	}
	if _, exists := registeredEnums[field.Name]; exists {
		panic("duplicate CRD enum registration: " + field.Name)
	}
	field.Path = slices.Clone(field.Path)
	registeredEnums[field.Name] = field
}

type Catalog struct {
	watcher *watcher.Watcher[*unstructured.Unstructured]
	fields  map[string]Enum
}

func NewWatcher(ctx context.Context, mgr *watcher.Manager) *Catalog {
	c := &Catalog{fields: maps.Clone(registeredEnums)}
	w := watcher.Watch(mgr, &unstructured.Unstructured{},
		watcher.WithGVR(schema.GroupVersionResource{
			Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions",
		}),
		watcher.WithTransformer(c.transform),
		watcher.WithConverter(func(o *unstructured.Unstructured, _ string) (any, bool) {
			return o, true
		}),
	)
	c.watcher = w
	w.Start(ctx)
	return c
}

type ctxKey int

const loadersKey ctxKey = iota

func NewLoaderContext(ctx context.Context, catalog *Catalog) context.Context {
	return context.WithValue(ctx, loadersKey, catalog)
}

func Values(ctx context.Context, environmentName, field string) ([]string, error) {
	catalog, _ := ctx.Value(loadersKey).(*Catalog)
	if catalog == nil {
		return nil, &unavailableError{environmentName, field, fmt.Errorf("catalog is not configured")}
	}
	return catalog.Values(environmentName, field)
}

func (c *Catalog) Values(environmentName, name string) ([]string, error) {
	values, err := c.values(environmentName, name)
	if err != nil {
		return nil, &unavailableError{environmentName, name, err}
	}
	return values, nil
}

func (c *Catalog) values(environmentName, name string) ([]string, error) {
	field, ok := c.fields[name]
	if !ok {
		return nil, fmt.Errorf("enum is not registered")
	}
	if !c.watcher.HasSynced(environmentName) {
		return nil, fmt.Errorf("environment cache is not synchronized")
	}
	obj, err := c.watcher.Get(environmentName, "", field.CRDName)
	if err != nil {
		return nil, err
	}
	if message, _, err := unstructured.NestedString(obj.Object, "enumValues", name, "error"); err != nil {
		return nil, err
	} else if message != "" {
		return nil, fmt.Errorf("%s", message)
	}
	values, found, err := unstructured.NestedStringSlice(obj.Object, "enumValues", name, "values")
	if err != nil {
		return nil, err
	}
	if !found || len(values) == 0 {
		return nil, fmt.Errorf("enum values are missing")
	}
	return values, nil
}

// Only retain registered values, not the full schemas or last-applied annotations.
func (c *Catalog) transform(obj any) (any, error) {
	o, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return nil, fmt.Errorf("expected an unstructured definition, got %T", obj)
	}
	if _, projected := o.Object["enumValues"]; projected {
		return o, nil
	}
	enums := make(map[string]any)
	for name, field := range c.fields {
		if o.GetName() != field.CRDName {
			continue
		}
		values, err := field.extract(o)
		if err != nil {
			// Cache extraction failures as well, so an invalid update cannot leave old values valid.
			enums[name] = map[string]any{"error": err.Error()}
		} else {
			items := make([]any, len(values))
			for i, value := range values {
				items[i] = value
			}
			enums[name] = map[string]any{"values": items}
		}
	}
	projected := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": o.GetAPIVersion(),
		"kind":       o.GetKind(),
		"enumValues": enums,
	}}
	projected.SetName(o.GetName())
	projected.SetNamespace(o.GetNamespace())
	projected.SetResourceVersion(o.GetResourceVersion())
	projected.SetUID(o.GetUID())
	projected.SetLabels(o.GetLabels())
	return projected, nil
}

func (e Enum) extract(obj *unstructured.Unstructured) ([]string, error) {
	rawVersions, _, err := unstructured.NestedFieldNoCopy(obj.Object, "spec", "versions")
	if err != nil {
		return nil, err
	}
	versions, ok := rawVersions.([]any)
	if !ok {
		return nil, fmt.Errorf("schema versions must be a list")
	}
	for _, item := range versions {
		version, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid schema version")
		}
		if version["name"] != e.Version {
			continue
		}
		if version["served"] != true {
			return nil, fmt.Errorf("version %q is not served", e.Version)
		}
		path := []string{"schema", "openAPIV3Schema"}
		for _, property := range e.Path {
			path = append(path, "properties", property)
		}
		values, found, err := unstructured.NestedStringSlice(version, append(path, "enum")...)
		if err != nil {
			return nil, err
		}
		if !found || len(values) == 0 {
			return nil, fmt.Errorf("no string enum at %s in version %s", strings.Join(e.Path, "."), e.Version)
		}
		return values, nil
	}
	return nil, fmt.Errorf("schema version %q is missing", e.Version)
}

type unavailableError struct {
	environmentName string
	field           string
	cause           error
}

func (e *unavailableError) Error() string {
	return fmt.Sprintf("%s: %v", e.GraphError(), e.cause)
}

func (e *unavailableError) Unwrap() error { return e.cause }

func (e *unavailableError) GraphError() string {
	return fmt.Sprintf("Available values for %s are unavailable in environment %q. Please try again later.", e.field, e.environmentName)
}
