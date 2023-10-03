package memory

import (
	"sync"

	"github.com/google/uuid"
	"github.com/wk8/go-ordered-map/v2"

	"github.com/denisvmedia/inventario/registry"
)

type Registry[T any] struct {
	items *orderedmap.OrderedMap[string, T]
	lock  sync.RWMutex
}

func NewRegistry[T any]() *Registry[T] {
	var item T
	_, ok := (any)(&item).(registry.Idable)
	if !ok {
		panic("registry: T must implement idable interface")
	}

	return &Registry[T]{
		items: orderedmap.New[string, T](),
	}
}

func (r *Registry[T]) Create(item T) (*T, error) {
	iitem := (any)(&item).(registry.Idable) //nolint:errcheck // checked in NewRegistry
	iitem.SetID(uuid.New().String())

	r.lock.Lock()
	r.items.Set(iitem.GetID(), item)
	r.lock.Unlock()

	return &item, nil
}

func (r *Registry[T]) Get(id string) (*T, error) {
	r.lock.RLock()
	item, ok := r.items.Get(id)
	r.lock.RUnlock()
	if !ok {
		return nil, registry.ErrNotFound
	}
	return &item, nil
}

func (r *Registry[T]) List() ([]T, error) {
	items := make([]T, 0, r.items.Len())
	r.lock.RLock()
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		items = append(items, pair.Value)
	}
	r.lock.RUnlock()
	return items, nil
}

func (r *Registry[T]) Update(item T) (*T, error) {
	iitem := (any)(&item).(registry.Idable) //nolint:errcheck // checked in NewRegistry

	r.lock.Lock()
	defer r.lock.Unlock()

	_, ok := r.items.Get(iitem.GetID())
	if !ok {
		return nil, registry.ErrNotFound
	}

	r.items.Set(iitem.GetID(), item)
	return &item, nil
}

func (r *Registry[T]) Delete(id string) error {
	r.lock.Lock()
	r.items.Delete(id)
	r.lock.Unlock()
	return nil
}

func (r *Registry[T]) Count() (int, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()
	return r.items.Len(), nil
}
