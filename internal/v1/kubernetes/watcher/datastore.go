package watcher

import (
	"slices"
	"strings"
	"sync"

	"k8s.io/apimachinery/pkg/labels"
)

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

type Filter func(obj Object, cluster string) bool

func WithLabels(lbls labels.Selector) Filter {
	return func(obj Object, cluster string) bool {
		return lbls.Matches(labels.Set(obj.GetLabels()))
	}
}

type List[T Object] []*EnvironmentWrapper[T]

func (l List[T]) Clone() []*EnvironmentWrapper[T] {
	ret := make([]*EnvironmentWrapper[T], len(l))
	for i, v := range l {
		ret[i] = &EnvironmentWrapper[T]{
			Cluster: v.Cluster,
			Obj:     v.Obj.DeepCopyObject().(T),
		}
	}
	return ret
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

type DataStore[T Object] struct {
	lock       sync.RWMutex
	namespaced map[string]List[T]
	cluster    map[string]List[T]
}

func NewDataStore[T Object]() *DataStore[T] {
	return &DataStore[T]{
		namespaced: make(map[string]List[T]),
		cluster:    make(map[string]List[T]),
	}
}

func (d *DataStore[T]) Add(cluster string, obj T) {
	d.lock.Lock()
	defer d.lock.Unlock()

	o := &EnvironmentWrapper[T]{Cluster: cluster, Obj: obj}

	if _, ok := d.cluster[cluster]; !ok {
		d.cluster[cluster] = make(List[T], 0)
	}
	d.cluster[cluster] = append(d.cluster[cluster], o)
	slices.SortFunc(d.cluster[cluster], func(i, j *EnvironmentWrapper[T]) int {
		// Sort by cluster, then by namespace, then by name
		if c := strings.Compare(i.Cluster, j.Cluster); c != 0 {
			return c
		}
		if n := strings.Compare(i.GetNamespace(), j.GetNamespace()); n != 0 {
			return n
		}
		return strings.Compare(i.GetName(), j.GetName())
	})

	if _, ok := d.namespaced[obj.GetNamespace()]; !ok {
		d.namespaced[obj.GetNamespace()] = make(List[T], 0)
	}
	d.namespaced[obj.GetNamespace()] = append(d.namespaced[obj.GetNamespace()], o)
	slices.SortFunc(d.namespaced[obj.GetNamespace()], func(i, j *EnvironmentWrapper[T]) int {
		// Sort by cluster, then by name
		if c := strings.Compare(i.Cluster, j.Cluster); c != 0 {
			return c
		}
		return strings.Compare(i.GetName(), j.GetName())
	})
}

func (d *DataStore[T]) Remove(cluster string, obj T) {
	d.lock.Lock()
	defer d.lock.Unlock()

	if _, ok := d.cluster[cluster]; ok {
		for i, o := range d.cluster[cluster] {
			if o.GetName() == obj.GetName() && o.GetNamespace() == obj.GetNamespace() {
				d.cluster[cluster] = append(d.cluster[cluster][:i], d.cluster[cluster][i+1:]...)
				break
			}
		}
		if len(d.cluster[cluster]) == 0 {
			delete(d.cluster, cluster)
		}
	}

	if _, ok := d.namespaced[obj.GetNamespace()]; ok {
		for i, o := range d.namespaced[obj.GetNamespace()] {
			if o.Cluster == cluster && o.GetName() == obj.GetName() {
				d.namespaced[obj.GetNamespace()] = append(d.namespaced[obj.GetNamespace()][:i], d.namespaced[obj.GetNamespace()][i+1:]...)
				break
			}
		}

		if len(d.namespaced[obj.GetNamespace()]) == 0 {
			delete(d.namespaced, obj.GetNamespace())
		}
	}
}

func (d *DataStore[T]) Update(cluster string, obj T) {
	d.lock.Lock()
	defer d.lock.Unlock()

	for _, a := range d.cluster[cluster] {
		if a.GetName() == obj.GetName() && a.GetNamespace() == obj.GetNamespace() {
			a.Obj = obj
		}
	}
}

func (d *DataStore[T]) GetByNamespace(namespace string, filters ...Filter) []*EnvironmentWrapper[T] {
	d.lock.RLock()
	defer d.lock.RUnlock()

	if _, ok := d.namespaced[namespace]; !ok {
		return nil
	}
	ret := d.namespaced[namespace].Clone()
	if len(filters) == 0 {
		return ret
	}

	return slices.DeleteFunc(ret, func(o *EnvironmentWrapper[T]) bool {
		for _, f := range filters {
			if !f(o.Obj, o.Cluster) {
				return true
			}
		}
		return false
	})
}

func (d *DataStore[T]) GetByCluster(cluster string, filters ...Filter) []*EnvironmentWrapper[T] {
	d.lock.RLock()
	defer d.lock.RUnlock()

	if _, ok := d.cluster[cluster]; !ok {
		return nil
	}

	ret := d.cluster[cluster].Clone()
	if len(filters) == 0 {
		return ret
	}

	return slices.DeleteFunc(ret, func(o *EnvironmentWrapper[T]) bool {
		for _, f := range filters {
			if !f(o.Obj, o.Cluster) {
				return true
			}
		}
		return false
	})
}

func (d *DataStore[T]) Get(cluster, namespace, name string) (T, error) {
	d.lock.RLock()
	defer d.lock.RUnlock()

	for _, o := range d.cluster[cluster] {
		if o.GetName() == name && o.GetNamespace() == namespace {
			return o.Obj, nil
		}
	}
	var t T
	return t, &ErrorNotFound{Cluster: cluster, Namespace: namespace, Name: name}
}

func (d *DataStore[T]) All() []*EnvironmentWrapper[T] {
	d.lock.RLock()
	defer d.lock.RUnlock()

	size := 0
	for _, l := range d.cluster {
		size += len(l)
	}

	ret := make([]*EnvironmentWrapper[T], size)
	if size == 0 {
		return ret
	}

	i := 0
	for _, l := range d.cluster {
		for _, o := range l {
			ret[i] = o
			i++
		}
	}
	return ret
}
