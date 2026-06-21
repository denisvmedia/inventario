# Interactive Input Abstraction

This package provides an interactive command-line input abstraction for the Inventario CLI, with support for several input types, per-field validation, and re-prompting on invalid input.

## Features

- **Multiple Input Types**: `bool`, `string`, `password`, and `int` fields
- **Reader-based I/O**: every field reads from a `*Reader` wrapping an `io.Reader` / `io.Writer`, so input/output are injectable (and easy to test)
- **Fluent API**: chainable field configuration (each method returns a new field copy)
- **Re-prompting**: on a validation error the user is re-prompted; non-validation errors (EOF, interrupts) are returned immediately
- **Default Values**: optional fields can carry a default
- **Secure Input**: hidden password input with an optional confirmation prompt
- **Per-field Validation**: `ValidateEmail` / `ValidateSlug` / `ValidateStrength` / `ValidateCustom`

## Core Types

- **`Reader`** — wraps input/output; created with `NewReader(in io.Reader, out io.Writer)` or `NewDefaultReader()` (stdin/stdout).
- **Field types** — `StringField`, `BoolField`, `PasswordField`, `IntField`, each created with `New<Type>Field(label string, reader *Reader)` and satisfying the `InputField` interface (`Prompt(ctx) (any, error)`).
- **`Quick`** — one-off helpers; created with `NewQuick(in io.Reader, out io.Writer)` or `NewDefaultQuick()`.
- **`Form`** — collects multiple fields at once; created with `NewForm(in io.Reader, out io.Writer)` or `NewDefaultForm()`.

> There is no `Builder` type and no standalone validator constructors — validation lives on the field types as `Validate*` methods.

## Quick Start

### Single fields

This is the pattern the CLI commands use (see `cmd/inventario/users/create`):

```go
import (
    "context"
    "os"

    "github.com/denisvmedia/inventario/cmd/internal/input"
)

ctx := context.Background()
reader := input.NewReader(os.Stdin, os.Stdout)

emailField := input.NewStringField("Email", reader).
    Required().
    ValidateEmail()

value, err := emailField.Prompt(ctx)
if err != nil {
    return err
}
email := value.(string)

passwordField := input.NewPasswordField("Password", reader).
    ValidateStrength()

pwValue, err := passwordField.Prompt(ctx)
if err != nil {
    return err
}
password := pwValue.(string)
```

### Quick API

```go
ctx := context.Background()
quick := input.NewQuick(os.Stdin, os.Stdout) // or input.NewDefaultQuick()

name, err := quick.String(ctx, "Your name")
email, err := quick.Email(ctx, "Email")   // string field with ValidateEmail
slug, err := quick.Slug(ctx, "Slug")      // string field with ValidateSlug
confirm, err := quick.Bool(ctx, "Confirm")
password, err := quick.Password(ctx, "Password")
age, err := quick.Int(ctx, "Age")
```

### Form API

`Form` collects all added fields and returns a `map[string]any` keyed by the field label.

```go
ctx := context.Background()
form := input.NewForm(os.Stdin, os.Stdout) // or input.NewDefaultForm()

form.AddString("name").Required().MinLength(2)
form.AddString("email").Required().ValidateEmail()
form.AddInt("age").Range(18, 100)
form.AddBool("active").DefaultYes()

values, err := form.Collect(ctx)
if err != nil {
    return err
}

name := values["name"].(string)
email := values["email"].(string)
age := values["age"].(int)
active := values["active"].(bool)
```

## Input Types

### String Input (`StringField`)

```go
reader := input.NewReader(os.Stdin, os.Stdout)
field := input.NewStringField("Name", reader).
    Required().
    MinLength(2).
    MaxLength(50).
    ValidateSlug()
```

**Methods:**
- `Required()` / `Optional()` — set field requirement
- `Default(value)` — set default value
- `MinLength(n)` / `MaxLength(n)` — length constraints
- `ValidateEmail()` — require a valid email address
- `ValidateSlug()` — require a valid slug (lowercase letters, numbers, hyphens)
- `ValidateCustom(func(string) error)` — custom validation
- `SetReader(*Reader)` — swap the reader (used in tests)

### Boolean Input (`BoolField`)

```go
field := input.NewBoolField("Confirm", reader).DefaultYes()
```

**Methods:**
- `Required()` / `Optional()` — set field requirement
- `DefaultYes()` — default true (prompt shows `Y/n`)
- `DefaultNo()` — default false (prompt shows `y/N`)
- `SetReader(*Reader)` — swap the reader (used in tests)

Accepted answers: `y`/`Y`/`yes`/`Yes`/`YES` for true, `n`/`N`/`no`/`No`/`NO` for false.

### Password Input (`PasswordField`)

```go
field := input.NewPasswordField("Password", reader).
    MinLength(8).
    ValidateStrength()
```

Passwords are **required and confirmed by default**.

**Methods:**
- `Required()` / `Optional()` — set field requirement
- `WithConfirmation()` / `NoConfirmation()` — enable/disable the confirmation prompt
- `MinLength(n)` — minimum length
- `ValidateStrength()` — require 8+ chars with upper, lower, and a digit
- `ValidateCustom(func(string) error)` — custom validation
- `SetReader(*Reader)` — swap the reader (used in tests)

### Integer Input (`IntField`)

```go
field := input.NewIntField("Age", reader).
    Range(18, 100).
    Default(25)
```

**Methods:**
- `Required()` / `Optional()` — set field requirement
- `Default(value)` — set default value
- `Min(n)` / `Max(n)` — value constraints
- `Range(min, max)` — set both
- `Positive()` — `> 0`
- `NonNegative()` — `>= 0`
- `SetReader(*Reader)` — swap the reader (used in tests)

## Validation

Validation is attached per field via the `Validate*` methods listed above. Under the hood they use the package helpers in `patterns.go`:

- `IsValidEmail(string) bool`
- `IsValidSlug(string) bool`
- `IsValidPassword(string) bool` (8+ chars with upper, lower, and a digit)

A failed validation surfaces as an `AnswerError` (see `errors.go`), which the field's `Prompt` loop catches and re-prompts on. A missing required field surfaces as a `RequiredFieldError`.

## Integration with Cobra Commands

Build a `*Reader` from the command's output writer so prompts go to the right stream:

```go
func (c *Command) collectEmail(ctx context.Context) (string, error) {
    reader := input.NewReader(os.Stdin, c.Cmd().OutOrStdout())
    emailField := input.NewStringField("Email", reader).
        Required().
        ValidateEmail()

    value, err := emailField.Prompt(ctx)
    if err != nil {
        return "", err
    }
    email, ok := value.(string)
    if !ok {
        return "", fmt.Errorf("unexpected type returned from email field")
    }
    return email, nil
}
```

## Testing

The package is easy to test by injecting a `*Reader` over in-memory buffers:

```go
func TestInput(t *testing.T) {
    var output bytes.Buffer
    reader := input.NewReader(strings.NewReader("test@example.com\n"), &output)

    field := input.NewStringField("Email", reader).Required().ValidateEmail()
    result, err := field.Prompt(context.Background())

    // assert on result and output
}
```

`SetReader` can also swap a field's reader after construction, which is handy when wiring fields up before the input source is known.
