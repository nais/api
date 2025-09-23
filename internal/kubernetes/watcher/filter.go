package watcher

import "k8s.io/apimachinery/pkg/labels"

type ErrorNotFound struct {
	Cluster   string
	Namespace string
	Name      string
}

func (e *ErrorNotFound) Error() string {
	return "not found: " + e.Cluster + "/" + e.Namespace + "/" + e.Name
}

func (e *ErrorNotFound) GraphError() string {
	return "Resource not found: " + e.Cluster + "/" + e.Namespace + "/" + e.Name
}

func (e *ErrorNotFound) As(v any) bool {
	if _, ok := v.(*ErrorNotFound); ok {
		return true
	}

	return false
}

func (e *ErrorNotFound) Is(v error) bool {
	if _, ok := v.(*ErrorNotFound); ok {
		return true
	}

	return false
}

type EnvironmentWrapper[T Object] struct {
	Cluster string
	Obj     T
}

func (e EnvironmentWrapper[T]) GetNamespace() string {
	return e.Obj.GetNamespace()
}

func (e EnvironmentWrapper[T]) GetName() string {
	return e.Obj.GetName()
}

type filterOptions struct {
	labels         labels.Selector
	clusters       []string
	withoutDeleted bool
}

type Filter func(o *filterOptions)

func WithLabels(lbls labels.Selector) Filter {
	return func(o *filterOptions) {
		o.labels = lbls
	}
}

func InCluster(cluster string) Filter {
	return func(o *filterOptions) {
		o.clusters = append(o.clusters, cluster)
	}
}

func WithoutDeleted() Filter {
	return func(o *filterOptions) {
		o.withoutDeleted = true
	}
}
