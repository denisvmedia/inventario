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
		c.Assert(err, qt.ErrorIs, typekit.ErrNoFieldWithTag)
		c.Assert(err, qt.ErrorMatches, `cannot set field "nonexistent": .*`)
	})

	// Test with unexported field
	c.Run("unexported field", func(c *qt.C) {
		config := &TestConfig{}
		err := typekit.SetFieldByConfigfieldTag(config, "hidden", "value")
		c.Assert(err, qt.ErrorIs, typekit.ErrUnsettableField)
		c.Assert(err, qt.ErrorMatches, `cannot set field "unexported": .*`)
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

func TestGetFieldByConfigfieldTag(t *testing.T) {
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
		NoTag      string
	}

	// Test with valid struct and tag - happy path
	c.Run("valid struct and tag", func(c *qt.C) {
		desc := "test description"
		count := 42
		config := &TestConfig{
			Name:        "John Doe",
			Age:         30,
			IsActive:    true,
			Description: &desc,
			Count:       &count,
		}

		// Test string field
		value, err := typekit.GetFieldByConfigfieldTag(config, "name")
		c.Assert(err, qt.IsNil)
		c.Assert(value, qt.Equals, "John Doe")

		// Test int field
		value, err = typekit.GetFieldByConfigfieldTag(config, "age")
		c.Assert(err, qt.IsNil)
		c.Assert(value, qt.Equals, 30)

		// Test bool field
		value, err = typekit.GetFieldByConfigfieldTag(config, "active")
		c.Assert(err, qt.IsNil)
		c.Assert(value, qt.Equals, true)

		// Test pointer field
		value, err = typekit.GetFieldByConfigfieldTag(config, "desc")
		c.Assert(err, qt.IsNil)
		c.Assert(value, qt.DeepEquals, &desc)

		// Test pointer field with int
		value, err = typekit.GetFieldByConfigfieldTag(config, "count")
		c.Assert(err, qt.IsNil)
		c.Assert(value, qt.DeepEquals, &count)
	})

	// Test with zero values
	c.Run("zero values", func(c *qt.C) {
		config := &TestConfig{}

		// Test zero string
		value, err := typekit.GetFieldByConfigfieldTag(config, "name")
		c.Assert(err, qt.IsNil)
		c.Assert(value, qt.Equals, "")

		// Test zero int
		value, err = typekit.GetFieldByConfigfieldTag(config, "age")
		c.Assert(err, qt.IsNil)
		c.Assert(value, qt.Equals, 0)

		// Test zero bool
		value, err = typekit.GetFieldByConfigfieldTag(config, "active")
		c.Assert(err, qt.IsNil)
		c.Assert(value, qt.Equals, false)

		// Test nil pointer
		value, err = typekit.GetFieldByConfigfieldTag(config, "desc")
		c.Assert(err, qt.IsNil)
		c.Assert(value, qt.IsNil)
	})

	// Test with unexported field - should return error instead of panicking
	c.Run("unexported field", func(c *qt.C) {
		config := &TestConfig{}
		value, err := typekit.GetFieldByConfigfieldTag(config, "hidden")
		c.Assert(err, qt.ErrorMatches, "cannot access field unexported")
		c.Assert(value, qt.IsNil)
	})
}

func TestGetFieldByConfigfieldTag_ErrorCases(t *testing.T) {
	c := qt.New(t)

	type TestConfig struct {
		Name string `configfield:"name"`
		Age  int    `configfield:"age"`
	}

	// Test with nil pointer
	c.Run("nil pointer", func(c *qt.C) {
		var config *TestConfig
		value, err := typekit.GetFieldByConfigfieldTag(config, "name")
		c.Assert(err, qt.ErrorMatches, "ptr must be a non-nil pointer to struct")
		c.Assert(value, qt.IsNil)
	})

	// Test with non-pointer
	c.Run("non-pointer", func(c *qt.C) {
		config := TestConfig{Name: "test"}
		value, err := typekit.GetFieldByConfigfieldTag(config, "name")
		c.Assert(err, qt.ErrorMatches, "ptr must be a non-nil pointer to struct")
		c.Assert(value, qt.IsNil)
	})

	// Test with pointer to non-struct
	c.Run("pointer to non-struct", func(c *qt.C) {
		num := 42
		value, err := typekit.GetFieldByConfigfieldTag(&num, "name")
		c.Assert(err, qt.ErrorMatches, "ptr must point to a struct")
		c.Assert(value, qt.IsNil)
	})

	// Test with non-existent tag
	c.Run("non-existent tag", func(c *qt.C) {
		config := &TestConfig{Name: "test", Age: 30}
		value, err := typekit.GetFieldByConfigfieldTag(config, "nonexistent")
		c.Assert(err, qt.ErrorMatches, `no field with configfield tag "nonexistent" found`)
		c.Assert(value, qt.IsNil)
	})

	// Test with empty tag
	c.Run("empty tag", func(c *qt.C) {
		config := &TestConfig{Name: "test", Age: 30}
		value, err := typekit.GetFieldByConfigfieldTag(config, "")
		c.Assert(err, qt.ErrorMatches, `no field with configfield tag "" found`)
		c.Assert(value, qt.IsNil)
	})

	// Test with unexported field
	c.Run("unexported field access", func(c *qt.C) {
		type TestConfigWithUnexported struct {
			Name       string `configfield:"name"`
			unexported string `configfield:"hidden"` //nolint:unused // it's a test
		}
		config := &TestConfigWithUnexported{Name: "test"}
		value, err := typekit.GetFieldByConfigfieldTag(config, "hidden")
		c.Assert(err, qt.ErrorMatches, "cannot access field unexported")
		c.Assert(value, qt.IsNil)
	})
}

func TestGetFieldByConfigfieldTag_AdditionalTypes(t *testing.T) {
	c := qt.New(t)

	// Test with more complex types
	type ComplexConfig struct {
		Float64Value float64         `configfield:"float"`
		SliceValue   []string        `configfield:"slice"`
		MapValue     map[string]int  `configfield:"map"`
		StructValue  struct{ X int } `configfield:"struct"`
		InterfaceVal interface{}     `configfield:"interface"`
	}

	c.Run("complex types", func(c *qt.C) {
		config := &ComplexConfig{
			Float64Value: 3.14,
			SliceValue:   []string{"a", "b", "c"},
			MapValue:     map[string]int{"key": 42},
			StructValue:  struct{ X int }{X: 100},
			InterfaceVal: "interface value",
		}

		// Test float64
		value, err := typekit.GetFieldByConfigfieldTag(config, "float")
		c.Assert(err, qt.IsNil)
		c.Assert(value, qt.Equals, 3.14)

		// Test slice
		value, err = typekit.GetFieldByConfigfieldTag(config, "slice")
		c.Assert(err, qt.IsNil)
		c.Assert(value, qt.DeepEquals, []string{"a", "b", "c"})

		// Test map
		value, err = typekit.GetFieldByConfigfieldTag(config, "map")
		c.Assert(err, qt.IsNil)
		c.Assert(value, qt.DeepEquals, map[string]int{"key": 42})

		// Test struct
		value, err = typekit.GetFieldByConfigfieldTag(config, "struct")
		c.Assert(err, qt.IsNil)
		c.Assert(value, qt.DeepEquals, struct{ X int }{X: 100})

		// Test interface
		value, err = typekit.GetFieldByConfigfieldTag(config, "interface")
		c.Assert(err, qt.IsNil)
		c.Assert(value, qt.Equals, "interface value")
	})

	c.Run("zero values for complex types", func(c *qt.C) {
		config := &ComplexConfig{}

		// Test zero float64
		value, err := typekit.GetFieldByConfigfieldTag(config, "float")
		c.Assert(err, qt.IsNil)
		c.Assert(value, qt.Equals, 0.0)

		// Test nil slice
		value, err = typekit.GetFieldByConfigfieldTag(config, "slice")
		c.Assert(err, qt.IsNil)
		c.Assert(value, qt.IsNil)

		// Test nil map
		value, err = typekit.GetFieldByConfigfieldTag(config, "map")
		c.Assert(err, qt.IsNil)
		c.Assert(value, qt.IsNil)

		// Test zero struct
		value, err = typekit.GetFieldByConfigfieldTag(config, "struct")
		c.Assert(err, qt.IsNil)
		c.Assert(value, qt.DeepEquals, struct{ X int }{})

		// Test nil interface
		value, err = typekit.GetFieldByConfigfieldTag(config, "interface")
		c.Assert(err, qt.IsNil)
		c.Assert(value, qt.IsNil)
	})
}

func TestStructToMap(t *testing.T) {
	c := qt.New(t)

	// Define test struct with various field types
	type TestStruct struct {
		Name        string  `configfield:"name"`
		Age         int     `configfield:"age"`
		IsActive    bool    `configfield:"active"`
		Description *string `configfield:"desc"`
		Count       *int    `configfield:"count"`
		NoTag       string  // Field without configfield tag
		//revive:disable-next-line:struct-tag
		unexported string `configfield:"hidden"` //nolint:unused // it's a test
	}

	// Test with valid struct - happy path
	c.Run("valid struct with values", func(c *qt.C) {
		desc := "test description"
		count := 42
		config := &TestStruct{
			Name:        "John Doe",
			Age:         30,
			IsActive:    true,
			Description: &desc,
			Count:       &count,
			NoTag:       "no tag value",
		}

		result, err := typekit.StructToMap(config)
		c.Assert(err, qt.IsNil)
		c.Assert(result, qt.IsNotNil)

		// Check that fields with configfield tags are included
		c.Assert(result["name"], qt.Equals, "John Doe")
		c.Assert(result["age"], qt.Equals, 30)
		c.Assert(result["active"], qt.Equals, true)
		c.Assert(result["desc"], qt.DeepEquals, &desc)
		c.Assert(result["count"], qt.DeepEquals, &count)

		// Check that field without configfield tag gets empty string key
		c.Assert(result[""], qt.Equals, "no tag value")

		// Check that unexported field is not included (due to PkgPath check)
		_, exists := result["hidden"]
		c.Assert(exists, qt.IsFalse)
	})

	// Test with zero values
	c.Run("struct with zero values", func(c *qt.C) {
		config := &TestStruct{}

		result, err := typekit.StructToMap(config)
		c.Assert(err, qt.IsNil)
		c.Assert(result, qt.IsNotNil)

		// Check zero values
		c.Assert(result["name"], qt.Equals, "")
		c.Assert(result["age"], qt.Equals, 0)
		c.Assert(result["active"], qt.Equals, false)
		c.Assert(result["desc"], qt.IsNil)
		c.Assert(result["count"], qt.IsNil)
		c.Assert(result[""], qt.Equals, "")
	})
}

func TestStructToMap_NonPointerStructs(t *testing.T) {
	c := qt.New(t)

	// Define test struct with various field types
	type TestStruct struct {
		Name        string  `configfield:"name"`
		Age         int     `configfield:"age"`
		IsActive    bool    `configfield:"active"`
		Description *string `configfield:"desc"`
		Count       *int    `configfield:"count"`
		NoTag       string  // Field without configfield tag
		//revive:disable-next-line:struct-tag
		unexported string `configfield:"hidden"` //nolint:unused // it's a test
	}

	// Test with non-pointer struct with values
	c.Run("non-pointer struct with values", func(c *qt.C) {
		desc := "test description"
		count := 42
		config := TestStruct{
			Name:        "John Doe",
			Age:         30,
			IsActive:    true,
			Description: &desc,
			Count:       &count,
			NoTag:       "no tag value",
		}

		result, err := typekit.StructToMap(config)
		c.Assert(err, qt.IsNil)
		c.Assert(result, qt.IsNotNil)

		// Check that fields with configfield tags are included
		c.Assert(result["name"], qt.Equals, "John Doe")
		c.Assert(result["age"], qt.Equals, 30)
		c.Assert(result["active"], qt.Equals, true)
		c.Assert(result["desc"], qt.DeepEquals, &desc)
		c.Assert(result["count"], qt.DeepEquals, &count)

		// Check that field without configfield tag gets empty string key
		c.Assert(result[""], qt.Equals, "no tag value")

		// Check that unexported field is not included (due to PkgPath check)
		_, exists := result["hidden"]
		c.Assert(exists, qt.IsFalse)
	})

	// Test with non-pointer struct with zero values
	c.Run("non-pointer struct with zero values", func(c *qt.C) {
		config := TestStruct{}

		result, err := typekit.StructToMap(config)
		c.Assert(err, qt.IsNil)
		c.Assert(result, qt.IsNotNil)

		// Check zero values
		c.Assert(result["name"], qt.Equals, "")
		c.Assert(result["age"], qt.Equals, 0)
		c.Assert(result["active"], qt.Equals, false)
		c.Assert(result["desc"], qt.IsNil)
		c.Assert(result["count"], qt.IsNil)
		c.Assert(result[""], qt.Equals, "")
	})

	// Test with anonymous struct (non-pointer)
	c.Run("anonymous struct", func(c *qt.C) {
		config := struct {
			Name string `configfield:"name"`
			Age  int    `configfield:"age"`
		}{
			Name: "Anonymous",
			Age:  25,
		}

		result, err := typekit.StructToMap(config)
		c.Assert(err, qt.IsNil)
		c.Assert(result, qt.IsNotNil)

		c.Assert(result["name"], qt.Equals, "Anonymous")
		c.Assert(result["age"], qt.Equals, 25)
		c.Assert(len(result), qt.Equals, 2)
	})
}

func TestStructToMap_ErrorCases(t *testing.T) {
	c := qt.New(t)

	// Test with nil pointer
	c.Run("nil pointer", func(c *qt.C) {
		var config *struct {
			Name string `configfield:"name"`
		}
		result, err := typekit.StructToMap(config)
		c.Assert(err, qt.ErrorMatches, "ptr must be a non-nil pointer to struct")
		c.Assert(result, qt.IsNil)
	})

	// Test with pointer to non-struct
	c.Run("pointer to non-struct", func(c *qt.C) {
		num := 42
		result, err := typekit.StructToMap(&num)
		c.Assert(err, qt.ErrorMatches, "ptr must point to a struct")
		c.Assert(result, qt.IsNil)
	})

	// Test with non-struct, non-pointer types
	c.Run("non-struct types", func(c *qt.C) {
		// Test with int
		result, err := typekit.StructToMap(42)
		c.Assert(err, qt.ErrorMatches, "ptr must be a pointer to struct")
		c.Assert(result, qt.IsNil)

		// Test with string
		result, err = typekit.StructToMap("hello")
		c.Assert(err, qt.ErrorMatches, "ptr must be a pointer to struct")
		c.Assert(result, qt.IsNil)

		// Test with slice
		result, err = typekit.StructToMap([]int{1, 2, 3})
		c.Assert(err, qt.ErrorMatches, "ptr must be a pointer to struct")
		c.Assert(result, qt.IsNil)

		// Test with map
		result, err = typekit.StructToMap(map[string]int{"key": 42})
		c.Assert(err, qt.ErrorMatches, "ptr must be a pointer to struct")
		c.Assert(result, qt.IsNil)
	})
}

func TestStructToMap_ComplexTypes(t *testing.T) {
	c := qt.New(t)

	// Test with complex field types
	type ComplexStruct struct {
		Float64Value float64           `configfield:"float"`
		SliceValue   []string          `configfield:"slice"`
		MapValue     map[string]int    `configfield:"map"`
		StructValue  struct{ X int }   `configfield:"struct"`
		InterfaceVal interface{}       `configfield:"interface"`
		ChannelVal   chan int          `configfield:"channel"`
		FuncVal      func() string     `configfield:"func"`
		ArrayVal     [3]int            `configfield:"array"`
		PtrToStruct  *struct{ Y bool } `configfield:"ptr_struct"`
	}

	c.Run("complex types with values", func(c *qt.C) {
		ch := make(chan int, 1)
		fn := func() string { return "test" }
		ptrStruct := &struct{ Y bool }{Y: true}

		config := &ComplexStruct{
			Float64Value: 3.14,
			SliceValue:   []string{"a", "b", "c"},
			MapValue:     map[string]int{"key": 42},
			StructValue:  struct{ X int }{X: 100},
			InterfaceVal: "interface value",
			ChannelVal:   ch,
			FuncVal:      fn,
			ArrayVal:     [3]int{1, 2, 3},
			PtrToStruct:  ptrStruct,
		}

		result, err := typekit.StructToMap(config)
		c.Assert(err, qt.IsNil)
		c.Assert(result, qt.IsNotNil)

		// Check complex types
		c.Assert(result["float"], qt.Equals, 3.14)
		c.Assert(result["slice"], qt.DeepEquals, []string{"a", "b", "c"})
		c.Assert(result["map"], qt.DeepEquals, map[string]int{"key": 42})
		c.Assert(result["struct"], qt.DeepEquals, struct{ X int }{X: 100})
		c.Assert(result["interface"], qt.Equals, "interface value")
		c.Assert(result["channel"], qt.Equals, ch)
		c.Assert(result["array"], qt.DeepEquals, [3]int{1, 2, 3})
		c.Assert(result["ptr_struct"], qt.DeepEquals, ptrStruct)

		// Function comparison is tricky, just check it exists and is not nil
		funcVal, exists := result["func"]
		c.Assert(exists, qt.IsTrue)
		c.Assert(funcVal, qt.IsNotNil)
	})

	c.Run("complex types with zero values", func(c *qt.C) {
		config := &ComplexStruct{}

		result, err := typekit.StructToMap(config)
		c.Assert(err, qt.IsNil)
		c.Assert(result, qt.IsNotNil)

		// Check zero values for complex types
		c.Assert(result["float"], qt.Equals, 0.0)
		c.Assert(result["slice"], qt.IsNil)
		c.Assert(result["map"], qt.IsNil)
		c.Assert(result["struct"], qt.DeepEquals, struct{ X int }{})
		c.Assert(result["interface"], qt.IsNil)
		c.Assert(result["channel"], qt.IsNil)
		c.Assert(result["func"], qt.IsNil)
		c.Assert(result["array"], qt.DeepEquals, [3]int{})
		c.Assert(result["ptr_struct"], qt.IsNil)
	})

	// Test with non-pointer complex struct
	c.Run("non-pointer complex types", func(c *qt.C) {
		ch := make(chan int, 1)
		fn := func() string { return "test" }
		ptrStruct := &struct{ Y bool }{Y: true}

		config := ComplexStruct{
			Float64Value: 2.71,
			SliceValue:   []string{"x", "y"},
			MapValue:     map[string]int{"test": 123},
			StructValue:  struct{ X int }{X: 200},
			InterfaceVal: 42,
			ChannelVal:   ch,
			FuncVal:      fn,
			ArrayVal:     [3]int{4, 5, 6},
			PtrToStruct:  ptrStruct,
		}

		result, err := typekit.StructToMap(config)
		c.Assert(err, qt.IsNil)
		c.Assert(result, qt.IsNotNil)

		// Check complex types work with non-pointer structs
		c.Assert(result["float"], qt.Equals, 2.71)
		c.Assert(result["slice"], qt.DeepEquals, []string{"x", "y"})
		c.Assert(result["map"], qt.DeepEquals, map[string]int{"test": 123})
		c.Assert(result["struct"], qt.DeepEquals, struct{ X int }{X: 200})
		c.Assert(result["interface"], qt.Equals, 42)
		c.Assert(result["channel"], qt.Equals, ch)
		c.Assert(result["array"], qt.DeepEquals, [3]int{4, 5, 6})
		c.Assert(result["ptr_struct"], qt.DeepEquals, ptrStruct)

		// Function comparison is tricky, just check it exists and is not nil
		funcVal, exists := result["func"]
		c.Assert(exists, qt.IsTrue)
		c.Assert(funcVal, qt.IsNotNil)
	})
}

func TestStructToMap_EdgeCases(t *testing.T) {
	c := qt.New(t)

	// Test with empty struct
	c.Run("empty struct", func(c *qt.C) {
		type EmptyStruct struct{}
		config := &EmptyStruct{}

		result, err := typekit.StructToMap(config)
		c.Assert(err, qt.IsNil)
		c.Assert(result, qt.IsNotNil)
		c.Assert(len(result), qt.Equals, 0)
	})

	// Test with struct containing only unexported fields
	c.Run("only unexported fields", func(c *qt.C) {
		type UnexportedStruct struct {
			//revive:disable-next-line:struct-tag
			name string `configfield:"name"` //nolint:unused // it's a test
			//revive:disable-next-line:struct-tag
			age int `configfield:"age"` //nolint:unused // it's a test
		}
		config := &UnexportedStruct{}

		result, err := typekit.StructToMap(config)
		c.Assert(err, qt.IsNil)
		c.Assert(result, qt.IsNotNil)
		c.Assert(len(result), qt.Equals, 0)
	})

	// Test with struct containing fields with duplicate configfield tags
	c.Run("duplicate configfield tags", func(c *qt.C) {
		type DuplicateTagStruct struct {
			Name1 string `configfield:"name"`
			Name2 string `configfield:"name"`
		}
		config := &DuplicateTagStruct{
			Name1: "first",
			Name2: "second",
		}

		result, err := typekit.StructToMap(config)
		c.Assert(err, qt.IsNil)
		c.Assert(result, qt.IsNotNil)

		// The second field should overwrite the first due to map behavior
		c.Assert(result["name"], qt.Equals, "second")
		c.Assert(len(result), qt.Equals, 1)
	})

	// Test with struct containing fields with empty configfield tags
	c.Run("empty configfield tags", func(c *qt.C) {
		type EmptyTagStruct struct {
			Name1 string `configfield:""`
			Name2 string `configfield:""`
			Name3 string `configfield:"valid"`
		}
		config := &EmptyTagStruct{
			Name1: "first",
			Name2: "second",
			Name3: "third",
		}

		result, err := typekit.StructToMap(config)
		c.Assert(err, qt.IsNil)
		c.Assert(result, qt.IsNotNil)

		// Both empty tag fields should map to empty string key, second overwrites first
		c.Assert(result[""], qt.Equals, "second")
		c.Assert(result["valid"], qt.Equals, "third")
		c.Assert(len(result), qt.Equals, 2)
	})

	// Test with struct containing mixed exported/unexported fields
	c.Run("mixed exported and unexported fields", func(c *qt.C) {
		type MixedStruct struct {
			ExportedField   string `configfield:"exported"`
			//revive:disable-next-line:struct-tag
			unexportedField string `configfield:"unexported"` //nolint:unused // it's a test
			AnotherExported int    `configfield:"another"`
		}
		config := &MixedStruct{
			ExportedField:   "exported value",
			AnotherExported: 42,
		}

		result, err := typekit.StructToMap(config)
		c.Assert(err, qt.IsNil)
		c.Assert(result, qt.IsNotNil)

		// Only exported fields should be included
		c.Assert(result["exported"], qt.Equals, "exported value")
		c.Assert(result["another"], qt.Equals, 42)
		c.Assert(len(result), qt.Equals, 2)

		// Unexported field should not be included
		_, exists := result["unexported"]
		c.Assert(exists, qt.IsFalse)
	})

	// Test with non-pointer empty struct
	c.Run("non-pointer empty struct", func(c *qt.C) {
		type EmptyStruct struct{}
		config := EmptyStruct{}

		result, err := typekit.StructToMap(config)
		c.Assert(err, qt.IsNil)
		c.Assert(result, qt.IsNotNil)
		c.Assert(len(result), qt.Equals, 0)
	})

	// Test with non-pointer struct with only unexported fields
	c.Run("non-pointer only unexported fields", func(c *qt.C) {
		type UnexportedStruct struct {
			//revive:disable-next-line:struct-tag
			name string `configfield:"name"` //nolint:unused // it's a test
			//revive:disable-next-line:struct-tag
			age int `configfield:"age"` //nolint:unused // it's a test
		}
		config := UnexportedStruct{}

		result, err := typekit.StructToMap(config)
		c.Assert(err, qt.IsNil)
		c.Assert(result, qt.IsNotNil)
		c.Assert(len(result), qt.Equals, 0)
	})
}
