package memory_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

type testItem struct {
	ID   string
	Data string
}

func (ti *testItem) GetID() string {
	return ti.ID
}

func (ti *testItem) SetID(id string) {
	ti.ID = id
}

func TestRegistry_Create(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Create a new instance of Registry
	r := memory.NewRegistry[testItem, *testItem]()

	// Create a test item
	item := &testItem{Data: "test"}

	// Create a new item in the registry
	createdItem, err := r.Create(ctx, *item)
	c.Assert(err, qt.IsNil)
	c.Assert(createdItem, qt.Not(qt.IsNil))

	// Verify the item is created with a valid ID
	c.Assert(createdItem.GetID(), qt.Not(qt.Equals), "")

	// Verify the count of items in the registry
	count, err := r.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 1)
}

func TestRegistry_Get(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Create a new instance of Registry
	r := memory.NewRegistry[testItem, *testItem]()

	// Create a test item
	item := &testItem{Data: "test"}

	// Create a new item in the registry
	createdItem, err := r.Create(ctx, *item)
	c.Assert(err, qt.IsNil)

	// Get the created item from the registry
	getItem, err := r.Get(ctx, createdItem.GetID())
	c.Assert(err, qt.IsNil)
	c.Assert(getItem, qt.DeepEquals, createdItem)

	// Get a non-existing item from the registry
	_, err = r.Get(ctx, "non-existing")
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

func TestRegistry_Update(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Create a new instance of Registry
	r := memory.NewRegistry[testItem, *testItem]()

	// Create a test item
	item := &testItem{Data: "test"}

	// Create a new item in the registry
	createdItem, err := r.Create(ctx, *item)
	c.Assert(err, qt.IsNil)

	// Update the item in the registry
	updatedItem, err := r.Update(ctx, testItem{ID: createdItem.GetID(), Data: "updated"})
	c.Assert(err, qt.IsNil)
	c.Assert(updatedItem, qt.Not(qt.IsNil))

	// Verify the updated item's data
	c.Assert(updatedItem.Data, qt.Equals, "updated")

	// Update a non-existing item in the registry
	_, err = r.Update(ctx, testItem{ID: "non-existing", Data: "updated"})
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

func TestRegistry_Delete(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Create a new instance of Registry
	r := memory.NewRegistry[testItem, *testItem]()

	// Create a test item
	item := &testItem{Data: "test"}

	// Create a new item in the registry
	createdItem, err := r.Create(ctx, *item)
	c.Assert(err, qt.IsNil)

	// Delete the item from the registry
	err = r.Delete(ctx, createdItem.GetID())
	c.Assert(err, qt.IsNil)

	// Verify that the item is deleted
	_, err = r.Get(ctx, createdItem.GetID())
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// Delete a non-existing item from the registry
	err = r.Delete(ctx, "non-existing")
	c.Assert(err, qt.IsNil)
}

func TestRegistry_Count(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Create a new instance of Registry
	r := memory.NewRegistry[testItem, *testItem]()

	// Verify the initial count of items in the registry
	count, err := r.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)

	// Create a test item
	item := &testItem{Data: "test"}

	// Create a new item in the registry
	_, err = r.Create(ctx, *item)
	c.Assert(err, qt.IsNil)

	// Verify the updated count of items in the registry
	count, err = r.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 1)
}

func TestRegistry_List(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Create a new instance of Registry
	r := memory.NewRegistry[testItem, *testItem]()

	// Create test items
	item1 := &testItem{Data: "item1"}
	item2 := &testItem{Data: "item2"}
	item3 := &testItem{Data: "item3"}

	// Create items in the registry
	_, err := r.Create(ctx, *item1)
	c.Assert(err, qt.IsNil)
	_, err = r.Create(ctx, *item2)
	c.Assert(err, qt.IsNil)
	_, err = r.Create(ctx, *item3)
	c.Assert(err, qt.IsNil)

	// Get the list of items from the registry
	items, err := r.List(ctx)
	c.Assert(err, qt.IsNil)

	// Verify the length and contents of the list
	c.Assert(len(items), qt.Equals, 3)
	c.Assert(items[0].Data, qt.Equals, "item1")
	c.Assert(items[1].Data, qt.Equals, "item2")
	c.Assert(items[2].Data, qt.Equals, "item3")
}
