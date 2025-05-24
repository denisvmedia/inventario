package boltdb_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/boltdb"
	"github.com/denisvmedia/inventario/registry/boltdb/dbx"
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

func setupTestRegistry(t *testing.T) (*boltdb.Registry[testItem, *testItem], func()) {
	c := qt.New(t)

	// Create a temporary directory for the test database
	tempDir := c.TempDir()

	// Create a new database in the temporary directory
	db, err := dbx.NewDB(tempDir, "test.db").Open()
	c.Assert(err, qt.IsNil)

	// Create a base repository
	base := dbx.NewBaseRepository[testItem, *testItem]("test_items")

	// Create a registry
	r := boltdb.NewRegistry[testItem, *testItem](db, base, "test_item", "")

	// Return the registry and a cleanup function
	cleanup := func() {
		err = db.Close()
		c.Assert(err, qt.IsNil)
	}

	return r, cleanup
}

// noopHook is a no-operation hook function for testing
func noopHook(_tx dbx.TransactionOrBucket, _item *testItem) error {
	return nil
}

func TestRegistry_Create(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of Registry
	r, cleanup := setupTestRegistry(t)
	defer cleanup()

	// Create a test item
	item := testItem{Data: "test"}

	// Create a new item in the registry
	createdItem, err := r.Create(item, noopHook, noopHook)
	c.Assert(err, qt.IsNil)
	c.Assert(createdItem, qt.Not(qt.IsNil))

	// Verify the item is created with a valid ID
	c.Assert(createdItem.GetID(), qt.Not(qt.Equals), "")

	// Verify the count of items in the registry
	count, err := r.Count()
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 1)
}

func TestRegistry_Get(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of Registry
	r, cleanup := setupTestRegistry(t)
	defer cleanup()

	// Create a test item
	item := testItem{Data: "test"}

	// Create a new item in the registry
	createdItem, _ := r.Create(item, noopHook, noopHook)

	// Get the created item from the registry
	getItem, err := r.Get(createdItem.GetID())
	c.Assert(err, qt.IsNil)
	c.Assert(getItem.ID, qt.Equals, createdItem.ID)
	c.Assert(getItem.Data, qt.Equals, createdItem.Data)

	// Get a non-existing item from the registry
	_, err = r.Get("non-existing")
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

func TestRegistry_Update(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of Registry
	r, cleanup := setupTestRegistry(t)
	defer cleanup()

	// Create a test item
	item := testItem{Data: "test"}

	// Create a new item in the registry
	createdItem, _ := r.Create(item, noopHook, noopHook)

	// Update the item in the registry
	updatedItem, err := r.Update(testItem{ID: createdItem.GetID(), Data: "updated"}, noopHook, noopHook)
	c.Assert(err, qt.IsNil)
	c.Assert(updatedItem, qt.Not(qt.IsNil))

	// Verify the updated item's data
	c.Assert(updatedItem.Data, qt.Equals, "updated")

	// Update a non-existing item in the registry
	_, err = r.Update(testItem{ID: "non-existing", Data: "updated"}, noopHook, noopHook)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

func TestRegistry_Delete(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of Registry
	r, cleanup := setupTestRegistry(t)
	defer cleanup()

	// Create a test item
	item := testItem{Data: "test"}

	// Create a new item in the registry
	createdItem, _ := r.Create(item, noopHook, noopHook)

	// Delete the item from the registry
	err := r.Delete(createdItem.GetID(), noopHook, noopHook)
	c.Assert(err, qt.IsNil)

	// Verify that the item is deleted
	_, err = r.Get(createdItem.GetID())
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// Delete a non-existing item from the registry
	err = r.Delete("non-existing", noopHook, noopHook)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

func TestRegistry_Count(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of Registry
	r, cleanup := setupTestRegistry(t)
	defer cleanup()

	// Verify the initial count of items in the registry
	count, err := r.Count()
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)

	// Create a test item
	item := testItem{Data: "test"}

	// Create a new item in the registry
	_, _ = r.Create(item, noopHook, noopHook)

	// Verify the updated count of items in the registry
	count, err = r.Count()
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 1)
}

func TestRegistry_List(t *testing.T) {
	c := qt.New(t)

	// Create a new instance of Registry
	r, cleanup := setupTestRegistry(t)
	defer cleanup()

	// Create test items
	item1 := testItem{Data: "item1"}
	item2 := testItem{Data: "item2"}
	item3 := testItem{Data: "item3"}

	// Create items in the registry
	createdItem1, _ := r.Create(item1, noopHook, noopHook)
	createdItem2, _ := r.Create(item2, noopHook, noopHook)
	createdItem3, _ := r.Create(item3, noopHook, noopHook)

	// Get the list of items from the registry
	items, err := r.List()
	c.Assert(err, qt.IsNil)

	// Verify the length of the list
	c.Assert(len(items), qt.Equals, 3)

	// Create a map of items by ID for easier verification
	itemMap := make(map[string]*testItem)
	for _, item := range items {
		itemMap[item.ID] = item
	}

	// Verify the contents of the list
	c.Assert(itemMap[createdItem1.ID].Data, qt.Equals, "item1")
	c.Assert(itemMap[createdItem2.ID].Data, qt.Equals, "item2")
	c.Assert(itemMap[createdItem3.ID].Data, qt.Equals, "item3")
}
