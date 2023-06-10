package registry

import (
	"github.com/google/uuid"

	"github.com/denisvmedia/inventario/internal/orderedmap"
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
	items *orderedmap.OrderedMap[T]
}

func NewMemoryRegistry[T any]() *MemoryRegistry[T] {
	return &MemoryRegistry[T]{
		items: orderedmap.New[T](),
	}
}

func (r *MemoryRegistry[T]) Create(item T) (*T, error) {
	iitem, ok := (any)(&item).(idable)
	if !ok {
		return nil, ErrNotIDable
	}
	iitem.SetID(uuid.New().String())
	r.items.Set(iitem.GetID(), item)

	return &item, nil
}

func (r *MemoryRegistry[T]) Get(id string) (*T, error) {
	item, ok := r.items.Get(id)
	if !ok {
		return nil, ErrNotFound
	}
	return &item, nil
}

func (r *MemoryRegistry[T]) List() ([]T, error) {
	items := make([]T, 0, r.items.Len())
	for _, item := range r.items.Iterate() {
		items = append(items, item.Value)
	}
	return items, nil
}

func (r *MemoryRegistry[T]) Update(item T) (*T, error) {
	iitem, ok := (any)(&item).(idable)
	if !ok {
		return nil, ErrNotIDable
	}

	if _, ok := r.items.Get(iitem.GetID()); !ok {
		return nil, ErrNotFound
	}

	r.items.Set(iitem.GetID(), item)
	return &item, nil
}

func (r *MemoryRegistry[T]) Delete(id string) error {
	r.items.Delete(id)
	return nil
}

func (r *MemoryRegistry[T]) Count() (int, error) {
	return r.items.Len(), nil
}
