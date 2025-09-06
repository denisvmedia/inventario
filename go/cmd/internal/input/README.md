# Interactive Input Abstraction

This package provides a comprehensive interactive command input abstraction for Go CLI applications with support for various input types, validation, and user-friendly prompting.

## Features

- **Multiple Input Types**: Support for `bool`, `password`, `string`, and `int` input types
- **Validation Integration**: Seamless integration with `github.com/jellydator/validation` package
- **Fluent API**: Builder pattern for creating input prompts with method chaining
- **Error Handling**: Automatic re-prompting on validation failures with clear error messages
- **Default Values**: Support for optional fields with default values
- **Secure Input**: Hidden password input with confirmation prompts
- **Flexible Formatting**: Multiple boolean prompt formats (y/n, Y/n, y/N, yes/no)
- **Range Validation**: Min/max constraints for numeric and string inputs

## Quick Start

### Simple Usage with Quick API

```go
package main

import (
    "context"
    "fmt"
    "github.com/denisvmedia/inventario/cmd/internal/input"
)

func main() {
    ctx := context.Background()
    quick := input.NewQuick()

    // Simple string input
    name, err := quick.String(ctx, "Your name")
    if err != nil {
        panic(err)
    }

    // String with default
    email, err := quick.StringWithDefault(ctx, "Email", "user@example.com")
    if err != nil {
        panic(err)
    }

    // Boolean input
    confirm, err := quick.Bool(ctx, "Confirm")
    if err != nil {
        panic(err)
    }

    // Password input
    password, err := quick.Password(ctx, "Password")
    if err != nil {
        panic(err)
    }

    fmt.Printf("Name: %s, Email: %s, Confirm: %t\n", name, email, confirm)
}
```

### Advanced Usage with Builder API

```go
package main

import (
    "context"
    "github.com/denisvmedia/inventario/cmd/internal/input"
)

func main() {
    ctx := context.Background()
    
    builder := input.NewBuilder()

    // String field with validation
    builder.String("email").
        Required().
        Validate(input.EmailValidator())

    // Password field with strength validation
    builder.Password("password").
        Validate(input.PasswordStrengthValidator())

    // Boolean field with default
    builder.Bool("newsletter").
        DefaultYes()

    // Integer field with range
    builder.Int("age").
        Range(18, 100)

    // Collect all values
    values, err := builder.Collect(ctx)
    if err != nil {
        panic(err)
    }

    // Access values
    email := values["email"].(string)
    password := values["password"].(string)
    newsletter := values["newsletter"].(bool)
    age := values["age"].(int)
}
```

### Form API with Struct Population

```go
package main

import (
    "context"
    "github.com/denisvmedia/inventario/cmd/internal/input"
)

type User struct {
    Name     string `json:"name"`
    Email    string `json:"email"`
    Age      int    `json:"age"`
    Active   bool   `json:"active"`
}

func main() {
    ctx := context.Background()
    
    form := input.NewForm()

    form.String("name").Required().MinLength(2)
    form.String("email").Required().Validate(input.EmailValidator())
    form.Int("age").Range(18, 100)
    form.Bool("active").DefaultYes()

    var user User
    err := form.CollectInto(ctx, &user)
    if err != nil {
        panic(err)
    }

    // user struct is now populated
}
```

## Input Types

### String Input

```go
field := input.NewStringField("Name", prompter).
    Required().
    MinLength(2).
    MaxLength(50).
    Validate(input.SlugValidator())
```

**Methods:**
- `Required()` / `Optional()` - Set field requirement
- `Default(value)` - Set default value
- `MinLength(min)` - Set minimum length
- `MaxLength(max)` - Set maximum length
- `Length(min, max)` - Set both min and max length
- `Validate(rules...)` - Add validation rules

### Boolean Input

```go
field := input.NewBoolField("Confirm", prompter).
    DefaultYes().
    YesNo()
```

**Methods:**
- `Required()` / `Optional()` - Set field requirement
- `Default(value)` - Set default value
- `DefaultYes()` - Set default to true with Y/n format
- `DefaultNo()` - Set default to false with y/N format
- `Format(format)` - Set prompt format
- `YesNo()` - Use full words format

**Formats:**
- `BoolFormatYN` - Equal weight (y/n)
- `BoolFormatYesN` - Default yes (Y/n)
- `BoolFormatYNo` - Default no (y/N)
- `BoolFormatYesNo` - Full words (yes/no)

### Password Input

```go
field := input.NewPasswordField("Password", prompter).
    MinLength(8).
    Validate(input.PasswordStrengthValidator())
```

**Methods:**
- `Required()` / `Optional()` - Set field requirement
- `WithConfirmation()` / `NoConfirmation()` - Enable/disable confirmation
- `MinLength(min)` - Set minimum length
- `MaxLength(max)` - Set maximum length
- `Length(min, max)` - Set both min and max length
- `Validate(rules...)` - Add validation rules

### Integer Input

```go
field := input.NewIntField("Age", prompter).
    Range(18, 100).
    Default(25)
```

**Methods:**
- `Required()` / `Optional()` - Set field requirement
- `Default(value)` - Set default value
- `Min(min)` - Set minimum value
- `Max(max)` - Set maximum value
- `Range(min, max)` - Set both min and max value
- `Positive()` - Constrain to positive values (> 0)
- `NonNegative()` - Constrain to non-negative values (>= 0)
- `Validate(rules...)` - Add validation rules

## Validation Helpers

The package provides common validation helpers:

```go
// Email validation
input.EmailValidator()

// Password strength (8+ chars, upper, lower, digit)
input.PasswordStrengthValidator()

// Slug format (lowercase, alphanumeric, hyphens)
input.SlugValidator()

// Length constraints
input.MinLengthValidator(5)
input.MaxLengthValidator(100)
input.LengthValidator(5, 100)

// Numeric constraints
input.MinValidator(0)
input.MaxValidator(100)
input.RangeValidator(0, 100)

// Custom validation
input.CustomValidator(func(value any) error {
    // Custom validation logic
    return nil
})
```

## Integration with Commands

### Using with Cobra Commands

```go
func (c *Command) collectUserInput() (*UserRequest, error) {
    ctx := context.Background()

    // Create fields with command output
    emailField := input.NewStringField("Email", input.NewDefaultPrompter()).
        Required().
        Validate(input.EmailValidator())
    emailField.BaseField.Prompter.SetOutput(c.Cmd().OutOrStdout())

    passwordField := input.NewPasswordField("Password", input.NewDefaultPrompter()).
        Validate(input.PasswordStrengthValidator())
    passwordField.BaseField.Prompter.SetOutput(c.Cmd().OutOrStdout())

    // Collect input
    email, err := emailField.Prompt(ctx)
    if err != nil {
        return nil, err
    }

    password, err := passwordField.Prompt(ctx)
    if err != nil {
        return nil, err
    }

    return &UserRequest{
        Email:    email.(string),
        Password: password.(string),
    }, nil
}
```

## Error Handling

The input system automatically handles validation errors by re-prompting the user:

```go
// This will keep prompting until valid email is entered
email, err := input.NewStringField("Email", prompter).
    Required().
    Validate(input.EmailValidator()).
    Prompt(ctx)
```

For non-validation errors (EOF, interrupts), the error is returned immediately.

## Testing

The package is designed to be easily testable by allowing custom input/output:

```go
func TestInput(t *testing.T) {
    var output bytes.Buffer
    inputReader := strings.NewReader("test@example.com\n")

    prompter := input.NewDefaultPrompter()
    prompter.SetOutput(&output)
    prompter.SetInput(inputReader)

    field := input.NewStringField("Email", prompter).Required()
    result, err := field.Prompt(context.Background())

    // Assert result and output
}
```
