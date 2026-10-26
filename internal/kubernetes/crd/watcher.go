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

type StringOptions struct {
	Values        []string
	DefaultValues []string
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

func Options(ctx context.Context, environmentName, field string) (*StringOptions, error) {
	catalog, _ := ctx.Value(loadersKey).(*Catalog)
	if catalog == nil {
		return nil, &unavailableError{environmentName, field, fmt.Errorf("catalog is not configured")}
	}
	return catalog.Options(environmentName, field)
}

func (c *Catalog) Options(environmentName, name string) (*StringOptions, error) {
	options, err := c.options(environmentName, name)
	if err != nil {
		return nil, &unavailableError{environmentName, name, err}
	}
	return options, nil
}

func (c *Catalog) options(environmentName, name string) (*StringOptions, error) {
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
	defaults, _, err := unstructured.NestedStringSlice(obj.Object, "enumValues", name, "defaultValues")
	if err != nil {
		return nil, err
	}
	return &StringOptions{Values: values, DefaultValues: defaults}, nil
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
		options, err := field.extract(o)
		if err != nil {
			// Cache extraction failures as well, so an invalid update cannot leave old values valid.
			enums[name] = map[string]any{"error": err.Error()}
		} else {
			data := make(map[string]any)
			for key, values := range map[string][]string{"values": options.Values, "defaultValues": options.DefaultValues} {
				if values == nil {
					continue
				}
				items := make([]any, len(values))
				for i, value := range values {
					items[i] = value
				}
				data[key] = items
			}
			enums[name] = data
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

func (e Enum) extract(obj *unstructured.Unstructured) (*StringOptions, error) {
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
		property, found, err := unstructured.NestedMap(version, path...)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, fmt.Errorf("no string enum at %s in version %s", strings.Join(e.Path, "."), e.Version)
		}
		options, err := extractOptions(property)
		if err != nil {
			return nil, fmt.Errorf("options at %s in version %s: %w", strings.Join(e.Path, "."), e.Version, err)
		}
		return options, nil
	}
	return nil, fmt.Errorf("schema version %q is missing", e.Version)
}

func extractOptions(property map[string]any) (*StringOptions, error) {
	enumPath := []string{"enum"}
	switch property["type"] {
	case "string":
	case "array":
		itemType, _, err := unstructured.NestedString(property, "items", "type")
		if err != nil {
			return nil, err
		}
		if itemType != "string" {
			return nil, fmt.Errorf("array items must be strings")
		}
		enumPath = []string{"items", "enum"}
	default:
		return nil, fmt.Errorf("expected a string or an array of strings")
	}
	values, found, err := unstructured.NestedStringSlice(property, enumPath...)
	if err != nil {
		return nil, err
	}
	if !found || len(values) == 0 {
		return nil, fmt.Errorf("no string enum")
	}
	options := &StringOptions{Values: values}
	if property["type"] == "array" {
		options.DefaultValues, _, err = unstructured.NestedStringSlice(property, "default")
		if err != nil {
			return nil, err
		}
	} else {
		value, found, err := unstructured.NestedString(property, "default")
		if err != nil {
			return nil, err
		}
		if found {
			options.DefaultValues = []string{value}
		}
	}
	for _, value := range options.DefaultValues {
		if !slices.Contains(values, value) {
			return nil, fmt.Errorf("default value %q is not in the enum", value)
		}
	}
	return options, nil
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
