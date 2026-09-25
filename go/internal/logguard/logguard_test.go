package logguard_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
)

// moduleRoot is where the walk starts, relative to this package.
const moduleRoot = "../.."

// dsnName matches an identifier that holds a connection string. Matching the
// name rather than the type is the point: the value is a plain string, so the
// type says nothing, and the name is what a reviewer reads too.
var dsnName = regexp.MustCompile(`(?i)^(db)?(dsn|conn(ection)?str(ing)?|databaseurl|dburl)$`)

type finding struct {
	position string
	expr     string
}

func (f finding) String() string { return fmt.Sprintf("%s\t%s", f.position, f.expr) }

// TestNoRawDSNReachesALogCall fails when a slog call takes a DSN-named value
// directly.
//
// Only bare identifiers and selectors are reported. A call expression has
// passed through something — `shared.RedactDSN(dsn)` masks the password,
// `parsedDSN.String()` renders a URL whose userinfo the caller already
// rewrote — and deciding which of those are safe is a judgement the guard
// cannot make from the name alone. Flagging the unwrapped values catches the
// mistake that has actually been made without inventing a taint analysis.
func TestNoRawDSNReachesALogCall(t *testing.T) {
	c := qt.New(t)

	fset := token.NewFileSet()
	var findings []finding
	scanned := 0

	err := filepath.WalkDir(moduleRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// The root arrives first and its Name() is "..", which the
			// hidden-directory check below would treat as a dotfile and
			// SkipDir — aborting the whole walk and leaving a guard that
			// passes because it looked at nothing.
			if path == moduleRoot {
				return nil
			}
			if name := d.Name(); name == "vendor" || name == "node_modules" || strings.HasPrefix(name, ".") {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		f, perr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if perr != nil {
			return fmt.Errorf("parse %s: %w", path, perr)
		}
		scanned++

		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !isSlogCall(call) {
				return true
			}
			for _, arg := range call.Args {
				name, ok := bareName(arg)
				if !ok || !dsnName.MatchString(name) {
					continue
				}
				findings = append(findings, finding{
					position: fset.Position(arg.Pos()).String(),
					expr:     exprString(arg),
				})
			}
			return true
		})
		return nil
	})
	c.Assert(err, qt.IsNil)

	// A walk that reaches nothing reports no findings, which is
	// indistinguishable from a clean tree. It has already happened here once:
	// the root's own Name() is "..", so the dot check below skipped the entire
	// module and the guard passed without opening a file.
	c.Assert(scanned > 100, qt.IsTrue,
		qt.Commentf("only %d file(s) scanned; the walk is not reaching the tree", scanned))

	sort.Slice(findings, func(i, j int) bool { return findings[i].position < findings[j].position })

	var report strings.Builder
	for _, f := range findings {
		report.WriteString(f.String())
		report.WriteString("\n")
	}
	c.Check(findings, qt.HasLen, 0, qt.Commentf(
		"a DSN carries its password; wrap it in shared.RedactDSN before logging\n%s", report.String()))
}

// TestBareNameSeesThroughParentheses pins the unwrapping, because the shape it
// guards against survives formatting: gofmt leaves a redundant `(dsn)` exactly
// as written, so a parenthesized leak reads as ordinary code in review and the
// guard has to be the thing that notices.
func TestBareNameSeesThroughParentheses(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		want string
	}{
		{name: "identifier", src: "dsn", want: "dsn"},
		{name: "parenthesized", src: "(dsn)", want: "dsn"},
		{name: "doubly parenthesized", src: "((dsn))", want: "dsn"},
		{name: "selector", src: "cfg.DBDSN", want: "DBDSN"},
		{name: "parenthesized selector", src: "(cfg.DBDSN)", want: "DBDSN"},
		// A call has passed through something; deciding whether that something
		// redacts is beyond what a name can tell us.
		{name: "call", src: "shared.RedactDSN(dsn)", want: ""},
		{name: "parenthesized call", src: "(shared.RedactDSN(dsn))", want: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)

			expr, err := parser.ParseExpr(tc.src)
			c.Assert(err, qt.IsNil)

			got, ok := bareName(expr)
			if tc.want == "" {
				c.Check(ok, qt.IsFalse, qt.Commentf("%s should not read as a bare name", tc.src))
				return
			}
			c.Assert(ok, qt.IsTrue, qt.Commentf("%s should read as a bare name", tc.src))
			c.Check(got, qt.Equals, tc.want)
		})
	}
}

// isSlogCall reports whether the call is slog.Error/Warn/Info/Debug, with or
// without the Context suffix.
func isSlogCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || pkg.Name != "slog" {
		return false
	}
	name := strings.TrimSuffix(sel.Sel.Name, "Context")
	switch name {
	case "Error", "Warn", "Info", "Debug":
		return true
	}
	return false
}

// bareName returns the trailing identifier of an expression that carries a
// value straight through — an identifier or a field selector. Anything else,
// a call above all, returns false.
//
// Parentheses are unwrapped first. gofmt keeps a redundant `(dsn)`, so without
// this the guard reads a parenthesized leak as "not a bare name" and passes.
func bareName(e ast.Expr) (string, bool) {
	for {
		paren, ok := e.(*ast.ParenExpr)
		if !ok {
			break
		}
		e = paren.X
	}

	switch v := e.(type) {
	case *ast.Ident:
		return v.Name, true
	case *ast.SelectorExpr:
		return v.Sel.Name, true
	default:
		return "", false
	}
}

func exprString(e ast.Expr) string {
	for {
		paren, ok := e.(*ast.ParenExpr)
		if !ok {
			break
		}
		e = paren.X
	}

	switch v := e.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		if x, ok := v.X.(*ast.Ident); ok {
			return x.Name + "." + v.Sel.Name
		}
		return "." + v.Sel.Name
	default:
		return "?"
	}
}
