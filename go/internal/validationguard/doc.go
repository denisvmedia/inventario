// Package validationguard holds source-level guards against validation
// mistakes that produce no error and no warning.
//
// jellydator/validation dereferences a field pointer before asking whether
// the value satisfies Validatable, so listing a non-pointer struct field
// whose validator has a pointer receiver validates nothing at all. The
// rules inside are simply skipped. It has been shipped three times: the
// embedded tenant/group identity (#2557), every JSON:API attributes struct
// (#2559), and the non-pointer Date fields alongside them.
//
// A second guard keeps string length limits in characters: validation.Length
// counts a string in UTF-8 bytes, validation.RuneLength in characters, and a
// Length on a string needs a comment that says why its limit is in bytes.
//
// The guards are tests rather than linters because they need no new tooling
// to run in CI, and the shapes they look for are narrow enough to express in
// one AST walk each.
package validationguard
