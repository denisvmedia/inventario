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
	"strconv"
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

// forbiddenKeys are the log keys devdocs/logging.md rules out, with what to
// write instead.
var forbiddenKeys = map[string]string{
	"err":        `use "error"`,
	"request_id": "the handler adds it from the context; pass the request's context instead",
}

// TestNoForbiddenKeyReachesALogCall fails when a slog call names a key the
// guideline rules out, as a literal key or as the key of a slog.Attr built in
// the call. Keys collected in a slice first (`attrs := []any{...}`) are out of
// its reach.
func TestNoForbiddenKeyReachesALogCall(t *testing.T) {
	c := qt.New(t)

	fset := token.NewFileSet()
	var findings []finding
	scanned := 0

	err := filepath.WalkDir(moduleRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
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
			for _, key := range logKeys(call) {
				if fix, bad := forbiddenKeys[key.name]; bad {
					findings = append(findings, finding{
						position: fset.Position(key.pos).String(),
						expr:     fmt.Sprintf("%q: %s", key.name, fix),
					})
				}
			}
			return true
		})
		return nil
	})
	c.Assert(err, qt.IsNil)
	c.Assert(scanned > 100, qt.IsTrue,
		qt.Commentf("only %d file(s) scanned; the walk is not reaching the tree", scanned))

	sort.Slice(findings, func(i, j int) bool { return findings[i].position < findings[j].position })

	var report strings.Builder
	for _, f := range findings {
		report.WriteString(f.String())
		report.WriteString("\n")
	}
	c.Check(findings, qt.HasLen, 0, qt.Commentf(
		"a log key devdocs/logging.md rules out\n%s", report.String()))
}

// TestLogKeysReadsKeysOnly pins which arguments count as keys: the message, a
// value and a key built outside the call do not.
func TestLogKeysReadsKeysOnly(t *testing.T) {
	for _, tc := range []struct {
		src  string
		want []string
	}{
		{src: `slog.Info("err")`, want: nil},
		{src: `slog.Info("x", "err", err)`, want: []string{"err"}},
		{src: `slog.ErrorContext(ctx, "x", "error", err, "user_id", id)`, want: []string{"error", "user_id"}},
		{src: `slog.Warn("x", "field", "err")`, want: []string{"field"}},
		{src: `slog.Warn("x", slog.Any("err", err), "k", v)`, want: []string{"err", "k"}},
		{src: `slog.Log(ctx, slog.LevelWarn, "x", "request_id", id)`, want: []string{"request_id"}},
		{src: `slog.LogAttrs(ctx, slog.LevelInfo, "x", slog.String("request_id", id))`, want: []string{"request_id"}},
		{src: `m.logger.WarnContext(ctx, "x", "err", err)`, want: []string{"err"}},
		{src: `slog.Info("x", attrs...)`, want: nil},
		{src: `slog.Info("x", attr, "k", v)`, want: []string{"k"}},
	} {
		t.Run(tc.src, func(t *testing.T) {
			c := qt.New(t)

			expr, err := parser.ParseExpr(tc.src)
			c.Assert(err, qt.IsNil)
			call, ok := expr.(*ast.CallExpr)
			c.Assert(ok, qt.IsTrue)
			c.Assert(isSlogCall(call), qt.IsTrue)

			var got []string
			for _, k := range logKeys(call) {
				got = append(got, k.name)
			}
			c.Assert(got, qt.DeepEquals, tc.want)
		})
	}
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

// The migrator and a few other components log through a *slog.Logger field,
// so a logger kept in a value is a log call too.
func TestIsSlogCallSeesLoggerFields(t *testing.T) {
	for _, tc := range []struct {
		src  string
		want bool
	}{
		{src: `slog.Info("x")`, want: true},
		{src: `slog.ErrorContext(ctx, "x")`, want: true},
		{src: `m.logger.InfoContext(ctx, "x")`, want: true},
		{src: `logger.Warn("x")`, want: true},
		{src: `slog.Log(ctx, slog.LevelWarn, "x")`, want: true},
		{src: `slog.LogAttrs(ctx, slog.LevelWarn, "x")`, want: true},
		{src: `fmt.Println("x")`, want: false},
		{src: `m.logger.With("k", v)`, want: false},
	} {
		t.Run(tc.src, func(t *testing.T) {
			c := qt.New(t)

			expr, err := parser.ParseExpr(tc.src)
			c.Assert(err, qt.IsNil)
			call, ok := expr.(*ast.CallExpr)
			c.Assert(ok, qt.IsTrue)
			c.Assert(isSlogCall(call), qt.Equals, tc.want)
		})
	}
}

// isSlogCall reports whether the call is slog.Error/Warn/Info/Debug, with or
// without the Context suffix, slog.Log or slog.LogAttrs, or the same method on
// a logger kept in a value named logger (`m.logger.InfoContext`), which writes
// to the same log.
func isSlogCall(call *ast.CallExpr) bool {
	return messageIndex(call) >= 0
}

// messageIndex returns the position of the message argument of a slog call,
// or -1 when the call is not one.
func messageIndex(call *ast.CallExpr) int {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return -1
	}
	recv, ok := bareName(sel.X)
	if !ok || (recv != "slog" && !strings.EqualFold(recv, "logger")) {
		return -1
	}
	switch sel.Sel.Name {
	case "Error", "Warn", "Info", "Debug":
		return 0
	case "ErrorContext", "WarnContext", "InfoContext", "DebugContext":
		return 1
	case "Log", "LogAttrs":
		return 2
	default:
		return -1
	}
}

type logKey struct {
	name string
	pos  token.Pos
}

// logKeys returns the keys a slog call names: a string literal in a key
// position, and the literal first argument of a slog.Attr constructor
// (`slog.Any("error", err)`). Any other argument in a key position is an
// Attr built elsewhere and takes no value after it.
func logKeys(call *ast.CallExpr) []logKey {
	start := messageIndex(call)
	if start < 0 {
		return nil
	}
	var keys []logKey
	for i := start + 1; i < len(call.Args); i++ {
		switch arg := call.Args[i].(type) {
		case *ast.BasicLit:
			if name, ok := stringLit(arg); ok {
				keys = append(keys, logKey{name: name, pos: arg.Pos()})
				i++ // the value
			}
		case *ast.CallExpr:
			if sel, ok := arg.Fun.(*ast.SelectorExpr); ok && len(arg.Args) > 0 {
				if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "slog" {
					if lit, ok := arg.Args[0].(*ast.BasicLit); ok {
						if name, ok := stringLit(lit); ok {
							keys = append(keys, logKey{name: name, pos: lit.Pos()})
						}
					}
				}
			}
		}
	}
	return keys
}

func stringLit(lit *ast.BasicLit) (string, bool) {
	if lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	return s, err == nil
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
