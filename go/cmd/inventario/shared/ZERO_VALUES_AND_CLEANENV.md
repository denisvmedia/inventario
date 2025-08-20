# Problem

cleanenv does not support pointer types. This is a problem because we use pointer types for our configuration structs to indicate that the values are optional.

# Option 1: Fork cleanenv and Addd Pointer Support

Add pointer handling to the parseValue function in cleanenv:

```go
switch valueType.Kind() {
// parse pointer value
case reflect.Ptr:
    if field.IsNil() {
		// Create a new instance of the pointed-to type
        field.Set(reflect.New(valueType.Elem()))
    }
	// Recursively parse the dereferenced value
    return parseValue(value.Elem(), value, sep, layout)
	
	// parse string value
	case reflect.String:
		
    // ...rest of the case statements...
}
```

# Option 2: Use Custom Setter Interface

Implement the Setter interface for your pointer types:

```go
type CustomType string


func (ct *CustomType) SetValue(s string) error {
    *ct = CustomType(s)
    return nil
}
```