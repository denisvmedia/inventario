package typekit_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/internal/typekit"
)

func TestZeroOfType(t *testing.T) {
	c := qt.New(t)

	// Test with basic types
	c.Run("int", func(c *qt.C) {
		var i = 42
		zero := typekit.ZeroOfType(i)
		c.Assert(zero, qt.Equals, 0)
	})

	c.Run("string", func(c *qt.C) {
		var s = "hello"
		zero := typekit.ZeroOfType(s)
		c.Assert(zero, qt.Equals, "")
	})

	// Test with struct
	type testStruct struct {
		Name string
		Age  int
	}

	c.Run("struct", func(c *qt.C) {
		ts := testStruct{Name: "Test", Age: 30}
		zero := typekit.ZeroOfType(ts)
		c.Assert(zero, qt.DeepEquals, testStruct{})
	})

	// Test with nil pointer
	c.Run("nil pointer", func(c *qt.C) {
		var ptr *testStruct
		zero := typekit.ZeroOfType(ptr)
		c.Assert(zero, qt.Not(qt.IsNil))
		c.Assert(zero, qt.DeepEquals, &testStruct{})
	})

	// Test with non-nil pointer
	c.Run("non-nil pointer", func(c *qt.C) {
		ptr := &testStruct{Name: "Test", Age: 30}
		zero := typekit.ZeroOfType(ptr)
		// For non-nil pointers, ZeroOfType returns a nil pointer (zero value of pointer type)
		c.Assert(zero, qt.IsNil)
	})
}

func TestSetFieldByConfigfieldTag(t *testing.T) {
	c := qt.New(t)

	// Define test struct
	type TestConfig struct {
		Name        string  `configfield:"name"`
		Age         int     `configfield:"age"`
		IsActive    bool    `configfield:"active"`
		Description *string `configfield:"desc"`
		Count       *int    `configfield:"count"`
		//revive:disable-next-line:struct-tag
		unexported string `configfield:"hidden"` //nolint:unused // it's a test
	}

	// Test with valid struct and tag
	c.Run("valid struct and tag", func(c *qt.C) {
		config := &TestConfig{}
		err := typekit.SetFieldByConfigfieldTag(config, "name", "John Doe")
		c.Assert(err, qt.IsNil)
		c.Assert(config.Name, qt.Equals, "John Doe")

		err = typekit.SetFieldByConfigfieldTag(config, "age", 30)
		c.Assert(err, qt.IsNil)
		c.Assert(config.Age, qt.Equals, 30)

		err = typekit.SetFieldByConfigfieldTag(config, "active", true)
		c.Assert(err, qt.IsNil)
		c.Assert(config.IsActive, qt.Equals, true)
	})

	// Test with nil pointer
	c.Run("nil pointer", func(c *qt.C) {
		var config *TestConfig
		err := typekit.SetFieldByConfigfieldTag(config, "name", "John Doe")
		c.Assert(err, qt.ErrorMatches, "ptr must be a non-nil pointer to struct")
	})

	// Test with non-struct pointer
	c.Run("non-struct pointer", func(c *qt.C) {
		var num = 42
		err := typekit.SetFieldByConfigfieldTag(&num, "name", "John Doe")
		c.Assert(err, qt.ErrorMatches, "ptr must point to a struct")
	})

	// Test with non-existent tag
	c.Run("non-existent tag", func(c *qt.C) {
		config := &TestConfig{}
		err := typekit.SetFieldByConfigfieldTag(config, "nonexistent", "value")
		c.Assert(err, qt.ErrorMatches, `no field with configfield tag "nonexistent" found`)
	})

	// Test with unexported field
	c.Run("unexported field", func(c *qt.C) {
		config := &TestConfig{}
		err := typekit.SetFieldByConfigfieldTag(config, "hidden", "value")
		c.Assert(err, qt.ErrorMatches, "cannot set field unexported")
	})

	// Test with pointer to non-pointer conversion
	c.Run("pointer to non-pointer", func(c *qt.C) {
		config := &TestConfig{}
		count := 42
		err := typekit.SetFieldByConfigfieldTag(config, "age", &count)
		c.Assert(err, qt.IsNil)
		c.Assert(config.Age, qt.Equals, 42)
	})

	// Test with non-pointer to pointer conversion
	c.Run("non-pointer to pointer", func(c *qt.C) {
		config := &TestConfig{}
		err := typekit.SetFieldByConfigfieldTag(config, "desc", "description")
		c.Assert(err, qt.IsNil)
		c.Assert(*config.Description, qt.Equals, "description")

		err = typekit.SetFieldByConfigfieldTag(config, "count", 10)
		c.Assert(err, qt.IsNil)
		c.Assert(*config.Count, qt.Equals, 10)
	})

	// Test with same type
	c.Run("same type", func(c *qt.C) {
		config := &TestConfig{}
		desc := "description"
		err := typekit.SetFieldByConfigfieldTag(config, "desc", &desc)
		c.Assert(err, qt.IsNil)
		c.Assert(*config.Description, qt.Equals, "description")

		count := 10
		err = typekit.SetFieldByConfigfieldTag(config, "count", &count)
		c.Assert(err, qt.IsNil)
		c.Assert(*config.Count, qt.Equals, 10)
	})
}
