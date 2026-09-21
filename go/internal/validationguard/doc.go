// Package validationguard holds a source-level guard against a validation
// mistake that produces no error and no warning.
//
// jellydator/validation dereferences a field pointer before asking whether
// the value satisfies Validatable, so listing a non-pointer struct field
// whose validator has a pointer receiver validates nothing at all. The
// rules inside are simply skipped. It has been shipped three times: the
// embedded tenant/group identity (#2557), every JSON:API attributes struct
// (#2559), and the non-pointer Date fields alongside them.
//
// The guard is a test rather than a linter because it needs no new tooling
// to run in CI, and the shape it looks for is narrow enough to express in
// one AST walk.
package validationguard
