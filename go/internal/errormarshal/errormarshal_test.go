package errormarshal_test

import (
"encoding/json"
"errors"
"fmt"
"testing"

"github.com/go-extras/errx"
errxtrace "github.com/go-extras/errx/stacktrace"
qt "github.com/frankban/quicktest"
"github.com/jellydator/validation"

"github.com/denisvmedia/inventario/internal/errormarshal"
)

type customError struct {
Code int
Msg  string
}

func (e customError) Error() string {
return fmt.Sprintf("code=%d msg=%s", e.Code, e.Msg)
}

type customErrorWithMarshal struct {
Code int
Msg  string
}

func (e customErrorWithMarshal) Error() string {
return fmt.Sprintf("code=%d msg=%s", e.Code, e.Msg)
}

func (e customErrorWithMarshal) MarshalJSON() ([]byte, error) {
type alias struct {
Code int    `json:"code"`
Msg  string `json:"msg"`
}
return json.Marshal(alias(e))
}

func TestMarshal_NilError(t *testing.T) {
c := qt.New(t)

result := errormarshal.Marshal(nil)
c.Assert(result, qt.Not(qt.IsNil))

// Should produce a valid JSON structure
var decoded map[string]any
err := json.Unmarshal(result, &decoded)
c.Assert(err, qt.IsNil)
c.Assert(decoded["type"], qt.Equals, "<nil>")
}

func TestMarshal_StandardError(t *testing.T) {
c := qt.New(t)

testErr := errors.New("standard error")
result := errormarshal.Marshal(testErr)
c.Assert(result, qt.Not(qt.IsNil))

// Should produce valid JSON
var decoded map[string]any
err := json.Unmarshal(result, &decoded)
c.Assert(err, qt.IsNil)

// Type should be present
typeStr, ok := decoded["type"].(string)
c.Assert(ok, qt.IsTrue)
c.Assert(typeStr, qt.Not(qt.Equals), "")
}

func TestMarshal_CustomErrorWithoutMarshalJSON(t *testing.T) {
c := qt.New(t)

testErr := customError{Code: 42, Msg: "something went wrong"}
result := errormarshal.Marshal(testErr)
c.Assert(result, qt.Not(qt.IsNil))

// Should produce valid JSON
var decoded map[string]any
err := json.Unmarshal(result, &decoded)
c.Assert(err, qt.IsNil)

// Type should be present
typeStr, ok := decoded["type"].(string)
c.Assert(ok, qt.IsTrue)
c.Assert(typeStr, qt.Matches, `.*customError.*`)
}

func TestMarshal_CustomErrorWithMarshalJSON(t *testing.T) {
c := qt.New(t)

testErr := customErrorWithMarshal{Code: 7, Msg: "boom"}
result := errormarshal.Marshal(testErr)
c.Assert(result, qt.Not(qt.IsNil))

// Should produce valid JSON
var decoded map[string]any
err := json.Unmarshal(result, &decoded)
c.Assert(err, qt.IsNil)
c.Assert(decoded["type"], qt.Matches, `.*customErrorWithMarshal.*`)

// Verify the structure is valid - either has error field (json.Marshaler path)
// or msg field (minimal fallback path if errx wraps it)
hasError := decoded["error"] != nil
hasMsg := decoded["msg"] != nil
c.Assert(hasError || hasMsg, qt.IsTrue)
}

func TestMarshal_ErrxError(t *testing.T) {
c := qt.New(t)

baseErr := errors.New("base error")
testErr := errx.Wrap("wrapped error", baseErr, errx.Attrs("key", "value"))
result := errormarshal.Marshal(testErr)
c.Assert(result, qt.Not(qt.IsNil))

// Should produce valid JSON
var decoded map[string]any
err := json.Unmarshal(result, &decoded)
c.Assert(err, qt.IsNil)

// Type and error field should be present
c.Assert(decoded["type"], qt.Not(qt.Equals), "")
c.Assert(decoded["error"], qt.Not(qt.IsNil))
}

func TestMarshal_ErrxWithStacktrace(t *testing.T) {
c := qt.New(t)

baseErr := errors.New("base error")
testErr := errxtrace.Wrap("wrapped with trace", baseErr)
result := errormarshal.Marshal(testErr)
c.Assert(result, qt.Not(qt.IsNil))

// Should produce valid JSON
var decoded map[string]any
err := json.Unmarshal(result, &decoded)
c.Assert(err, qt.IsNil)
}

func TestMarshal_ValidationError(t *testing.T) {
c := qt.New(t)

// validation.Errors implements json.Marshaler
testErr := validation.Errors{
"field1": errors.New("required"),
"field2": errors.New("invalid format"),
}
result := errormarshal.Marshal(testErr)
c.Assert(result, qt.Not(qt.IsNil))

// Should handle validation.Errors properly (it implements MarshalJSON)
var decoded map[string]any
err := json.Unmarshal(result, &decoded)
c.Assert(err, qt.IsNil)

// The error field should contain the validation errors
errorField, ok := decoded["error"].(map[string]any)
c.Assert(ok, qt.IsTrue)
c.Assert(len(errorField) > 0, qt.IsTrue)
}

func TestMarshal_ErrxWrappedValidationError(t *testing.T) {
c := qt.New(t)

validationErr := validation.Errors{
"email": errors.New("invalid email format"),
}
// Wrap validation error with errx
testErr := errx.Wrap("validation failed", validationErr)

result := errormarshal.Marshal(testErr)
c.Assert(result, qt.Not(qt.IsNil))

// Should handle the wrapped validation error
var decoded map[string]any
err := json.Unmarshal(result, &decoded)
c.Assert(err, qt.IsNil)
}

func TestMarshal_ReturnsValidJSON(t *testing.T) {
testCases := []struct {
name string
err  error
}{
{"nil error", nil},
{"standard error", errors.New("test")},
{"custom error", customError{Code: 1, Msg: "test"}},
{"custom with marshal", customErrorWithMarshal{Code: 2, Msg: "test"}},
{"errx error", errx.Wrap("test", errors.New("base"))},
{"validation error", validation.Errors{"field": errors.New("error")}},
}

for _, tc := range testCases {
t.Run(tc.name, func(t *testing.T) {
c := qt.New(t)

result := errormarshal.Marshal(tc.err)
c.Assert(result, qt.Not(qt.IsNil))

// Must be valid JSON
var decoded any
err := json.Unmarshal(result, &decoded)
c.Assert(err, qt.IsNil, qt.Commentf("result: %s", string(result)))
})
}
}

func TestMustMarshal_Success(t *testing.T) {
c := qt.New(t)

testErr := errors.New("test error")
result := errormarshal.MustMarshal(testErr)
c.Assert(result, qt.Not(qt.IsNil))

// Should be valid JSON
var decoded any
err := json.Unmarshal(result, &decoded)
c.Assert(err, qt.IsNil)
}

func TestMarshal_JSONStructureFormat(t *testing.T) {
c := qt.New(t)

testErr := customErrorWithMarshal{Code: 123, Msg: "test message"}
result := errormarshal.Marshal(testErr)

// Verify the structure has type field
var decoded struct {
Error json.RawMessage `json:"error"`
Msg   string          `json:"msg"`
Type  string          `json:"type"`
}
err := json.Unmarshal(result, &decoded)
c.Assert(err, qt.IsNil)
c.Assert(decoded.Type, qt.Not(qt.Equals), "")
// Either error or msg should be present
hasError := len(decoded.Error) > 0
hasMsg := decoded.Msg != ""
c.Assert(hasError || hasMsg, qt.IsTrue)
}

func TestMarshal_StandardErrorsFallbackToMinimal(t *testing.T) {
c := qt.New(t)

testErr := errors.New("simple error")
result := errormarshal.Marshal(testErr)

// Verify we get a valid structure
var decoded map[string]any
err := json.Unmarshal(result, &decoded)
c.Assert(err, qt.IsNil)
c.Assert(decoded["type"], qt.Not(qt.Equals), "")

// Either msg field (minimal) or error field (errx wrapped) should be present
hasMsg := decoded["msg"] != nil
hasError := decoded["error"] != nil
c.Assert(hasMsg || hasError, qt.IsTrue)
}
