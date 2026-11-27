package validationguard_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
)

// TestStringLengthRulesCountCharacters fails when `validation.Length` limits
// a string without a comment that says the limit is in bytes.
//
// validation.Length measures a string with len(), in UTF-8 bytes, while the
// frontend and the people typing count characters: a Czech or Russian text
// takes two bytes for most letters and hits a byte limit at half the length.
// validation.RuneLength counts characters, which is also what a PostgreSQL
// VARCHAR(n) counts. Length stays right where the limit really is in bytes
// (an index entry, a request size), and a comment on the same line or the
// line above says so with the word "bytes".
//
// Length on a slice or a map counts elements and needs nothing. The guard
// knows the value is a collection when it is a field of the method's
// receiver, `&r.Field` or `*r.Field`, declared as a slice, a map, an array or
// a named type over one. Anything it cannot tell is treated as a string.
func TestStringLengthRulesCountCharacters(t *testing.T) {
	c := qt.New(t)

	fset := token.NewFileSet()
	files := parseTreeWithComments(c, fset)

	collectionTypes := collectCollectionTypeNames(files)
	structFields := collectStructFields(files)

	var findings []string
	for _, f := range files {
		onCollections := lengthCallsOnCollections(f, structFields, collectionTypes)
		bytesLines := linesWithBytesComment(fset, f)

		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !isValidationCall(call, "Length") || onCollections[call] {
				return true
			}
			pos := fset.Position(call.Pos())
			if bytesLines[pos.Line] || bytesLines[pos.Line-1] {
				return true
			}
			findings = append(findings, strings.TrimPrefix(pos.String(), moduleRoot+"/"))
			return true
		})
	}
	sort.Strings(findings)

	c.Assert(findings, qt.HasLen, 0, qt.Commentf(
		"these validation.Length rules count a string in bytes; use validation.RuneLength, "+
			"or say on the line or the line above why the limit is in bytes:\n%s",
		strings.Join(findings, "\n")))
}

// parseTreeWithComments is parseTree with the comments kept, which the
// bytes marker needs.
func parseTreeWithComments(c *qt.C, fset *token.FileSet) []*ast.File {
	c.Helper()

	var files []*ast.File
	err := filepath.WalkDir(moduleRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path == moduleRoot {
				return nil
			}
			if name := d.Name(); name == "vendor" || name == "node_modules" || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		parsed, perr := parser.ParseFile(fset, path, nil, parser.ParseComments|parser.SkipObjectResolution)
		if perr != nil {
			return fmt.Errorf("parse %s: %w", path, perr)
		}
		files = append(files, parsed)
		return nil
	})
	c.Assert(err, qt.IsNil)
	c.Assert(len(files) > 0, qt.IsTrue, qt.Commentf("walked %s and found no Go files", moduleRoot))
	return files
}

// isValidationCall reports whether call is validation.<name>(...).
func isValidationCall(call *ast.CallExpr, name string) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "validation"
}

// collectCollectionTypeNames returns the bare names of the declared types
// whose underlying type is a slice, an array or a map, such as
// `type ValuerSlice[T any] []T`.
func collectCollectionTypeNames(files []*ast.File) map[string]bool {
	out := map[string]bool{}
	for _, f := range files {
		for _, decl := range f.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				switch ts.Type.(type) {
				case *ast.ArrayType, *ast.MapType:
					out[ts.Name.Name] = true
				}
			}
		}
	}
	return out
}

func isCollectionType(expr ast.Expr, collectionTypes map[string]bool) bool {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return isCollectionType(t.X, collectionTypes)
	case *ast.ArrayType, *ast.MapType:
		return true
	case *ast.IndexExpr:
		return isCollectionType(t.X, collectionTypes)
	case *ast.IndexListExpr:
		return isCollectionType(t.X, collectionTypes)
	case *ast.Ident:
		return collectionTypes[t.Name]
	case *ast.SelectorExpr:
		return collectionTypes[t.Sel.Name]
	default:
		return false
	}
}

// lengthCallsOnCollections returns the validation.Length calls in f that are
// a rule of a validation.Field or validation.Validate call on a collection
// field of the method's receiver, directly or inside validation.When.
func lengthCallsOnCollections(
	f *ast.File,
	structFields map[string]map[string]ast.Expr,
	collectionTypes map[string]bool,
) map[*ast.CallExpr]bool {
	out := map[*ast.CallExpr]bool{}
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil || fn.Recv == nil || len(fn.Recv.List[0].Names) == 0 {
			continue
		}
		recvName := fn.Recv.List[0].Names[0].Name
		owner, ok := receiverTypeName(fn)
		if !ok {
			continue
		}
		fields := structFields[owner]

		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) < 2 {
				return true
			}
			if !isValidationCall(call, "Field") && !isValidationCall(call, "Validate") {
				return true
			}
			field, ok := receiverField(call.Args[0], recvName)
			if !ok {
				return true
			}
			declared, ok := fields[field]
			if !ok || !isCollectionType(declared, collectionTypes) {
				return true
			}
			for _, rule := range lengthRules(call.Args[1:]) {
				out[rule] = true
			}
			return true
		})
	}
	return out
}

// receiverField returns F for `&recv.F`, `recv.F` or `*recv.F`.
func receiverField(expr ast.Expr, recvName string) (string, bool) {
	switch t := expr.(type) {
	case *ast.UnaryExpr:
		return receiverField(t.X, recvName)
	case *ast.StarExpr:
		return receiverField(t.X, recvName)
	case *ast.SelectorExpr:
		x, ok := t.X.(*ast.Ident)
		if !ok || x.Name != recvName {
			return "", false
		}
		return t.Sel.Name, true
	default:
		return "", false
	}
}

func lengthRules(rules []ast.Expr) []*ast.CallExpr {
	var out []*ast.CallExpr
	for _, rule := range rules {
		call, ok := rule.(*ast.CallExpr)
		if !ok {
			continue
		}
		switch {
		case isValidationCall(call, "Length"):
			out = append(out, call)
		case isValidationCall(call, "When") && len(call.Args) > 1:
			out = append(out, lengthRules(call.Args[1:])...)
		}
	}
	return out
}

// linesWithBytesComment returns the lines of f that carry a comment
// mentioning bytes.
func linesWithBytesComment(fset *token.FileSet, f *ast.File) map[int]bool {
	out := map[int]bool{}
	for _, group := range f.Comments {
		for _, comment := range group.List {
			if strings.Contains(strings.ToLower(comment.Text), "bytes") {
				out[fset.Position(comment.Pos()).Line] = true
			}
		}
	}
	return out
}
