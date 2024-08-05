package watcher

import (
	"errors"
	"slices"
	"strings"
	"sync"

	"k8s.io/apimachinery/pkg/labels"
)

var ErrNotFound = errors.New("not found")

type Filter func(obj Object) bool

func WithLabels(lbls labels.Selector) Filter {
	return func(obj Object) bool {
		return lbls.Matches(labels.Set(obj.GetLabels()))
	}
}

type List[T Object] []*environmentWrapper[T]

func (l List[T]) Clone() []T {
	ret := make([]T, len(l))
	for i, v := range l {
		ret[i] = v.obj.DeepCopyObject().(T)
	}
	return ret
}

type environmentWrapper[T Object] struct {
	cluster string
	obj     T
}

func (e environmentWrapper[T]) GetNamespace() string {
	return e.obj.GetNamespace()
}

func (e environmentWrapper[T]) GetName() string {
	return e.obj.GetName()
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

	o := &environmentWrapper[T]{cluster: cluster, obj: obj}

	if _, ok := d.cluster[cluster]; !ok {
		d.cluster[cluster] = make(List[T], 0)
	}
	d.cluster[cluster] = append(d.cluster[cluster], o)
	slices.SortFunc(d.cluster[cluster], func(i, j *environmentWrapper[T]) int {
		// Sort by cluster, then by namespace, then by name
		if c := strings.Compare(i.cluster, j.cluster); c != 0 {
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
	slices.SortFunc(d.namespaced[obj.GetNamespace()], func(i, j *environmentWrapper[T]) int {
		// Sort by cluster, then by name
		if c := strings.Compare(i.cluster, j.cluster); c != 0 {
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
			if o.cluster == cluster && o.GetName() == obj.GetName() {
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
			a.obj = obj
		}
	}
}

func (d *DataStore[T]) GetByNamespace(namespace string, filters ...Filter) []T {
	d.lock.RLock()
	defer d.lock.RUnlock()

	if _, ok := d.namespaced[namespace]; !ok {
		return nil
	}
	ret := d.namespaced[namespace].Clone()
	if len(filters) == 0 {
		return ret
	}

	return slices.DeleteFunc(ret, func(o T) bool {
		for _, f := range filters {
			if !f(o) {
				return true
			}
		}
		return false
	})
}

func (d *DataStore[T]) GetByCluster(cluster string, filters ...Filter) []T {
	d.lock.RLock()
	defer d.lock.RUnlock()

	if _, ok := d.cluster[cluster]; !ok {
		return nil
	}

	ret := d.cluster[cluster].Clone()
	if len(filters) == 0 {
		return ret
	}

	return slices.DeleteFunc(ret, func(o T) bool {
		for _, f := range filters {
			if !f(o) {
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
			return o.obj, nil
		}
	}
	var t T
	return t, ErrNotFound
}
