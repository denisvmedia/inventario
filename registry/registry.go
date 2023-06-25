package registry

import (
	"sync"

	"github.com/google/uuid"
	"github.com/wk8/go-ordered-map/v2"
)

type idable interface {
	GetID() string
	SetID(id string)
}

type Registry[T any] interface {
	// Create creates a new T in the registry.
	Create(T) (*T, error)

	// Get returns a T from the registry.
	Get(id string) (*T, error)

	// List returns a list of Ts from the registry.
	List() ([]T, error)

	// Update updates a T in the registry.
	Update(T) (*T, error)

	// Delete deletes a T from the registry.
	Delete(id string) error

	// Count returns the number of Ts in the registry.
	Count() (int, error)
}

type MemoryRegistry[T any] struct {
	items *orderedmap.OrderedMap[string, T]
	lock  sync.RWMutex
}

func NewMemoryRegistry[T any]() *MemoryRegistry[T] {
	var item T
	_, ok := (any)(&item).(idable)
	if !ok {
		panic("registry: T must implement idable interface")
	}

	return &MemoryRegistry[T]{
		items: orderedmap.New[string, T](),
	}
}

func (r *MemoryRegistry[T]) Create(item T) (*T, error) {
	iitem := (any)(&item).(idable) //nolint:errcheck // checked in NewMemoryRegistry
	iitem.SetID(uuid.New().String())

	r.lock.Lock()
	r.items.Set(iitem.GetID(), item)
	r.lock.Unlock()

	return &item, nil
}

func (r *MemoryRegistry[T]) Get(id string) (*T, error) {
	r.lock.RLock()
	item, ok := r.items.Get(id)
	r.lock.RUnlock()
	if !ok {
		return nil, ErrNotFound
	}
	return &item, nil
}

func (r *MemoryRegistry[T]) List() ([]T, error) {
	items := make([]T, 0, r.items.Len())
	r.lock.RLock()
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		items = append(items, pair.Value)
	}
	r.lock.RUnlock()
	return items, nil
}

func (r *MemoryRegistry[T]) Update(item T) (*T, error) {
	iitem := (any)(&item).(idable) //nolint:errcheck // checked in NewMemoryRegistry

	r.lock.Lock()
	defer r.lock.Unlock()

	_, ok := r.items.Get(iitem.GetID())
	if !ok {
		return nil, ErrNotFound
	}

	r.items.Set(iitem.GetID(), item)
	return &item, nil
}

func (r *MemoryRegistry[T]) Delete(id string) error {
	r.lock.Lock()
	r.items.Delete(id)
	r.lock.Unlock()
	return nil
}

func (r *MemoryRegistry[T]) Count() (int, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()
	return r.items.Len(), nil
}
