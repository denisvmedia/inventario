package memory

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"github.com/wk8/go-ordered-map/v2"

	"github.com/denisvmedia/inventario/registry"
)

type Registry[T any, P registry.PIDable[T]] struct {
	items *orderedmap.OrderedMap[string, P]
	lock  sync.RWMutex
}

func NewRegistry[T any, P registry.PIDable[T]]() *Registry[T, P] {
	return &Registry[T, P]{
		items: orderedmap.New[string, P](),
	}
}

func (r *Registry[T, P]) Create(_ context.Context, item T) (P, error) {
	iitem := P(&item)
	iitem.SetID(uuid.New().String())

	r.lock.Lock()
	r.items.Set(iitem.GetID(), iitem)
	r.lock.Unlock()

	return iitem, nil
}

func (r *Registry[_, P]) Get(_ context.Context, id string) (P, error) {
	r.lock.RLock()
	item, ok := r.items.Get(id)
	r.lock.RUnlock()
	if !ok {
		return nil, registry.ErrNotFound
	}
	vitem := *item
	return &vitem, nil
}

func (r *Registry[_, P]) List(_ context.Context) ([]P, error) {
	items := make([]P, 0, r.items.Len())
	r.lock.RLock()
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		v := *pair.Value
		items = append(items, &v)
	}
	r.lock.RUnlock()
	return items, nil
}

func (r *Registry[T, P]) Update(_ context.Context, item T) (P, error) {
	iitem := P(&item)

	r.lock.Lock()
	defer r.lock.Unlock()

	_, ok := r.items.Get(iitem.GetID())
	if !ok {
		return nil, registry.ErrNotFound
	}

	r.items.Set(iitem.GetID(), iitem)
	return &item, nil
}

func (r *Registry[_, _]) Delete(_ context.Context, id string) error {
	r.lock.Lock()
	r.items.Delete(id)
	r.lock.Unlock()
	return nil
}

func (r *Registry[_, _]) Count(_ context.Context) (int, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()
	return r.items.Len(), nil
}
