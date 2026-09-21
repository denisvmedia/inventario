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

// moduleRoot is where the walk starts, relative to this package.
const moduleRoot = "../.."

// validationFieldCall is one `validation.Field(&recv.Name, rules...)` whose
// field type can never be validated by that call.
type validationFieldCall struct {
	position  string
	owner     string
	field     string
	fieldType string
}

func (f validationFieldCall) String() string {
	return fmt.Sprintf("%s\t%s.%s is %s", f.position, f.owner, f.field, f.fieldType)
}

// TestValidationFieldReachesTheFieldsValidator fails when a
// `validation.Field` call names a field whose own validator can never run.
//
// The shape: the field's declared type is a named struct (not a pointer),
// its Validate/ValidateWithContext has a pointer receiver, and the call
// carries no rule of its own. The library hands the *value* to the
// interface check, the value type does not implement it, and every rule
// inside the field's validator is skipped in silence.
//
// Two ways to satisfy the guard. Give the field's type value receivers,
// which is right when nothing calls its validator on a possibly-nil
// pointer. Or pass an explicit rule that does the check, which is what
// models.Date needs — it returns nil for a nil *Date and a value receiver
// would turn that into a panic.
func TestValidationFieldReachesTheFieldsValidator(t *testing.T) {
	c := qt.New(t)

	fset := token.NewFileSet()
	files := parseTree(c, fset)

	pointerReceiverValidators := collectPointerReceiverValidators(files)
	structFields := collectStructFields(files)

	var findings []validationFieldCall
	for _, f := range files {
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			owner, ok := receiverTypeName(fn)
			if !ok {
				continue
			}
			fields := structFields[owner]
			if fields == nil {
				continue
			}
			findings = append(findings,
				inspectBody(fset, fn.Body, owner, fields, pointerReceiverValidators)...)
		}
	}

	sort.Slice(findings, func(i, j int) bool { return findings[i].position < findings[j].position })

	report := make([]string, 0, len(findings))
	for _, f := range findings {
		report = append(report, f.String())
	}
	c.Assert(report, qt.HasLen, 0, qt.Commentf(
		"these validation.Field calls validate nothing — the field's validator has a pointer "+
			"receiver, so the value the library hands it does not implement the interface:\n%s",
		strings.Join(report, "\n")))
}

func parseTree(c *qt.C, fset *token.FileSet) []*ast.File {
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
			// Vendored and generated trees are not ours to police.
			// The dot check also keeps the walk out of .git.
			if name := d.Name(); name == "vendor" || name == "node_modules" || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		parsed, perr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
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

// collectPointerReceiverValidators records, per type name, whether its
// validators are reachable from a value. A type keeps its entry only while
// every validator it declares takes a pointer.
func collectPointerReceiverValidators(files []*ast.File) map[string]bool {
	pointerOnly := map[string]bool{}
	for _, f := range files {
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if fn.Name.Name != "Validate" && fn.Name.Name != "ValidateWithContext" {
				continue
			}
			name, pointer, ok := receiverType(fn)
			if !ok {
				continue
			}
			if prev, seen := pointerOnly[name]; seen && !prev {
				continue
			}
			pointerOnly[name] = pointer
		}
	}
	for name, pointer := range pointerOnly {
		if !pointer {
			delete(pointerOnly, name)
		}
	}
	return pointerOnly
}

func collectStructFields(files []*ast.File) map[string]map[string]ast.Expr {
	out := map[string]map[string]ast.Expr{}
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
				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					continue
				}
				fields := map[string]ast.Expr{}
				for _, field := range st.Fields.List {
					if len(field.Names) == 0 {
						if name, ok := typeName(field.Type); ok {
							fields[name] = field.Type
						}
						continue
					}
					for _, n := range field.Names {
						fields[n.Name] = field.Type
					}
				}
				out[ts.Name.Name] = fields
			}
		}
	}
	return out
}

func inspectBody(
	fset *token.FileSet,
	body *ast.BlockStmt,
	owner string,
	fields map[string]ast.Expr,
	pointerOnly map[string]bool,
) []validationFieldCall {
	var found []validationFieldCall
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || !isValidationField(call) {
			return true
		}
		// A rule of the caller's own is an explicit choice to validate
		// here instead of through the field's type.
		if len(call.Args) > 1 && carriesOwnRule(call.Args[1:]) {
			return true
		}
		field, ok := fieldSelector(call)
		if !ok {
			return true
		}
		declared, ok := fields[field]
		if !ok {
			return true
		}
		name, ok := typeName(declared)
		if !ok || !pointerOnly[name] {
			return true
		}
		found = append(found, validationFieldCall{
			position:  strings.TrimPrefix(fset.Position(call.Pos()).String(), moduleRoot+"/"),
			owner:     owner,
			field:     field,
			fieldType: name,
		})
		return true
	})
	return found
}

func isValidationField(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Field" || len(call.Args) == 0 {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "validation"
}

// carriesOwnRule reports whether any rule beyond validation.Required is
// present. Required only checks that the value is non-zero; it says nothing
// about whether the value is well formed.
func carriesOwnRule(rules []ast.Expr) bool {
	for _, rule := range rules {
		sel, ok := rule.(*ast.SelectorExpr)
		if ok && sel.Sel.Name == "Required" {
			continue
		}
		return true
	}
	return false
}

func fieldSelector(call *ast.CallExpr) (string, bool) {
	unary, ok := call.Args[0].(*ast.UnaryExpr)
	if !ok || unary.Op != token.AND {
		return "", false
	}
	sel, ok := unary.X.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	return sel.Sel.Name, true
}

func receiverTypeName(fn *ast.FuncDecl) (string, bool) {
	name, _, ok := receiverType(fn)
	return name, ok
}

// receiverType returns the receiver's type name and whether it is taken by
// pointer.
func receiverType(fn *ast.FuncDecl) (name string, pointer bool, ok bool) {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return "", false, false
	}
	expr := fn.Recv.List[0].Type
	if star, isStar := expr.(*ast.StarExpr); isStar {
		pointer = true
		expr = star.X
	}
	// A generic receiver is written Type[T]; the name is the base.
	if idx, isIdx := expr.(*ast.IndexExpr); isIdx {
		expr = idx.X
	}
	ident, isIdent := expr.(*ast.Ident)
	if !isIdent {
		return "", false, false
	}
	return ident.Name, pointer, true
}

// typeName returns the bare name of a named, non-pointer type: `Date` for
// both `Date` and `models.Date`. Pointers, slices, maps and everything else
// return false — the library reaches a pointer field's validator, and the
// element rules cover the collections.
//
// Keying on the bare name across packages is deliberate: two packages with
// a same-named type would be conflated, which can only report a finding
// that is not one, and the repo has no such pair today.
func typeName(expr ast.Expr) (string, bool) {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name, true
	case *ast.SelectorExpr:
		return t.Sel.Name, true
	default:
		return "", false
	}
}
