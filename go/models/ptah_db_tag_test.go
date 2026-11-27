package models_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
)

// ptahFieldName captures the column name of a `//ptah:schema:field` annotation.
var ptahFieldName = regexp.MustCompile(`^//\s*ptah:schema:field\s+.*?\bname="([^"]+)"`)

// TestPtahFieldNameIsTheDBTag fails when a field's `//ptah:schema:field`
// annotation names a column other than the field's `db` tag.
//
// The annotation decides the column the migrations create; the tag decides the
// column sqlx reads into the field. The PostgreSQL registries skip a column no
// field maps, so that a binary keeps working against a schema with columns
// added by a newer release. A field whose tag differs from its annotation would
// therefore read as its zero value instead of failing, and this test is where
// that mismatch fails.
func TestPtahFieldNameIsTheDBTag(t *testing.T) {
	c := qt.New(t)

	fset := token.NewFileSet()
	var mismatches []string
	annotated := 0

	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && path != "." {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return err
		}

		ast.Inspect(file, func(n ast.Node) bool {
			field, ok := n.(*ast.Field)
			if !ok || field.Doc == nil {
				return true
			}
			column, ok := annotatedColumn(field.Doc)
			if !ok {
				return true
			}
			annotated++
			if tag := dbTag(field); tag != column {
				mismatches = append(mismatches,
					fset.Position(field.Pos()).String()+": column "+column+", db tag "+strconv.Quote(tag))
			}
			return true
		})
		return nil
	})
	c.Assert(err, qt.IsNil)

	// The walk must reach the annotations at all, or an empty result proves
	// nothing.
	c.Assert(annotated > 0, qt.IsTrue, qt.Commentf("no //ptah:schema:field annotation found"))
	c.Assert(mismatches, qt.HasLen, 0, qt.Commentf("\n%s", strings.Join(mismatches, "\n")))
}

func annotatedColumn(doc *ast.CommentGroup) (string, bool) {
	for _, comment := range doc.List {
		if m := ptahFieldName.FindStringSubmatch(comment.Text); m != nil {
			return m[1], true
		}
	}
	return "", false
}

func dbTag(field *ast.Field) string {
	if field.Tag == nil {
		return ""
	}
	raw, err := strconv.Unquote(field.Tag.Value)
	if err != nil {
		return ""
	}
	name, _, _ := strings.Cut(reflect.StructTag(raw).Get("db"), ",")
	return name
}
