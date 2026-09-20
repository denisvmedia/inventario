package checkers

import (
	"encoding/json"
	"fmt"
	"reflect"

	qt "github.com/frankban/quicktest"
	"github.com/yalp/jsonpath"
)

// JSONPathEquals creates a custom checker for asserting a JSON value against a JSON path expression using `qt.Equals` checker.
func JSONPathEquals(jsonPath string) qt.Checker {
	return &jsonPathMatchesChecker{jsonPath: jsonPath, checker: qt.DeepEquals}
}

// JSONPathMatches creates a custom checker for asserting a JSON value against a JSON path expression using the provided `checker`.
func JSONPathMatches(jsonPath string, checker qt.Checker) qt.Checker {
	return &jsonPathMatchesChecker{jsonPath: jsonPath, checker: checker}
}

// jsonPathMatchesChecker is a custom checker implementation for JSONPathEqual.
type jsonPathMatchesChecker struct {
	checker  qt.Checker
	jsonPath string
}

// Check checks that the obtained JSON value matches the expected value at the given JSON path expression.
func (c *jsonPathMatchesChecker) Check(got any, args []any, note func(key string, value any)) error {
	switch v := got.(type) {
	case []any:
	case map[string]any:
	case string:
		var gotu any
		err := json.Unmarshal([]byte(v), &gotu)
		if err != nil {
			return fmt.Errorf("failed to unmarshal JSON: %w", err)
		}
		got = gotu
	case []byte:
		var gotu any
		err := json.Unmarshal(v, &gotu)
		if err != nil {
			return fmt.Errorf("failed to unmarshal JSON: %w", err)
		}
		got = gotu
	default:
		data, err := json.Marshal(got)
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}

		var gotu any
		err = json.Unmarshal(data, &gotu)
		if err != nil {
			return fmt.Errorf("failed to unmarshal JSON: %w", err)
		}
		got = gotu
	}

	notes := func() {
		v, _ := json.Marshal(got)

		note("marshalled data", qt.Unquoted(v))
		note("json path", c.jsonPath)
	}

	// Compile before evaluating so the two failure modes stay apart (#2131):
	// a malformed expression is a bug in the TEST and must fail even when the
	// expectation is nil, while a well-formed path that does not resolve in
	// this document is a legitimate way to assert absence. Reading first and
	// then letting `isNil && isNil` swallow the error made a typo'd path pass.
	filter, err := jsonpath.Prepare(c.jsonPath)
	if err != nil {
		notes()
		return fmt.Errorf("invalid JSON path expression %q: %w", c.jsonPath, err)
	}

	jsonPathVal, err := filter(got)

	if isNil(jsonPathVal) && isNil(args[0]) {
		return nil
	}

	if err != nil {
		notes()
		return fmt.Errorf("failed to evaluate JSON path expression: %w", err)
	}

	err = c.checker.Check(jsonPathVal, args, note)
	if err != nil {
		notes()
		return err
	}

	return nil
}

// ArgNames returns the names of all required arguments for the custom checker.
func (*jsonPathMatchesChecker) ArgNames() []string {
	return []string{"got", "want"}
}

func isNil(i any) bool {
	if i == nil {
		return true
	}

	iv := reflect.ValueOf(i)
	if !iv.IsValid() {
		return true
	}

	switch iv.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Map, reflect.Func, reflect.Interface:
		return iv.IsNil()
	default:
		return false
	}
}
