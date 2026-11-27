package registrytest

import (
	"context"
	"sync"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// InterleaveCommodityWrite wraps factory so that the first successful Get
// of item id through a user registry it creates runs write before Get
// returns. Get then returns the item one write behind, as a read is when a
// bulk change or a tag rename commits between a caller's read and its
// write. write gets the read and the unwrapped registry; an error from it
// is returned from Get. Service registries are not wrapped.
func InterleaveCommodityWrite(
	factory registry.CommodityRegistryFactory,
	id string,
	write func(ctx context.Context, reg registry.CommodityRegistry, read models.Commodity) error,
) registry.CommodityRegistryFactory {
	return &interleavingFactory{CommodityRegistryFactory: factory, id: id, write: write}
}

type interleavingFactory struct {
	registry.CommodityRegistryFactory

	id    string
	write func(ctx context.Context, reg registry.CommodityRegistry, read models.Commodity) error
	once  sync.Once
}

func (f *interleavingFactory) CreateUserRegistry(ctx context.Context) (registry.CommodityRegistry, error) {
	reg, err := f.CommodityRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, err
	}
	return &interleavingRegistry{CommodityRegistry: reg, factory: f}, nil
}

func (f *interleavingFactory) MustCreateUserRegistry(ctx context.Context) registry.CommodityRegistry {
	reg, err := f.CreateUserRegistry(ctx)
	if err != nil {
		panic(err)
	}
	return reg
}

type interleavingRegistry struct {
	registry.CommodityRegistry

	factory *interleavingFactory
}

func (r *interleavingRegistry) Get(ctx context.Context, id string) (*models.Commodity, error) {
	read, err := r.CommodityRegistry.Get(ctx, id)
	if err != nil || id != r.factory.id {
		return read, err
	}
	var writeErr error
	r.factory.once.Do(func() {
		writeErr = r.factory.write(ctx, r.CommodityRegistry, *read)
	})
	if writeErr != nil {
		return nil, writeErr
	}
	return read, nil
}
