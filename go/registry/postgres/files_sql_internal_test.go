package postgres

// White-box: fileWhere is unexported, and what this pins is the SQL it emits.

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
)

// One clause serves Search, ListPaginated and CountByCategory. A filter that
// silently stops applying answers with the wrong set rather than an error, and
// the caller reads that as "nothing matched".
func TestFileWhereSQL(t *testing.T) {
	fileType := models.FileTypeDocument
	category := models.FileCategoryDocuments
	linkedType := "commodity"
	linkedID := "com-1"

	for _, tc := range []struct {
		name     string
		query    string
		fileType *models.FileType
		category *models.FileCategory
		tags     []string
		linkType *string
		linkID   *string
		want     string
		args     []any
	}{
		{
			name: "no filter yields no clause",
			want: "SELECT * FROM files",
		},
		{
			name:     "type and category",
			fileType: &fileType,
			category: &category,
			want:     "SELECT * FROM files WHERE type = $1 AND category = $2",
			args:     []any{fileType, category},
		},
		{
			// The pair is all-or-nothing; a half pair applies no filter.
			name:     "the linked-entity pair",
			linkType: &linkedType,
			linkID:   &linkedID,
			want:     "SELECT * FROM files WHERE linked_entity_type = $1 AND linked_entity_id = $2",
			args:     []any{linkedType, linkedID},
		},
		{
			name:     "half a linked-entity pair applies nothing",
			linkType: &linkedType,
			want:     "SELECT * FROM files",
		},
		{
			// Containment, so every requested tag has to be present rather
			// than any of them.
			name: "tags use containment",
			tags: []string{"invoice", "tax"},
			want: "SELECT * FROM files WHERE tags @> $1",
			args: []any{[]byte(`["invoice","tax"]`)},
		},
		{
			name:  "search spans four columns",
			query: "receipt",
			want: "SELECT * FROM files WHERE (title ILIKE $1 OR description ILIKE $2 OR " +
				"path ILIKE $3 OR original_path ILIKE $4)",
			args: []any{"%receipt%", "%receipt%", "%receipt%", "%receipt%"},
		},
		{
			name:     "everything at once, in a stable order",
			query:    "receipt",
			fileType: &fileType,
			category: &category,
			tags:     []string{"invoice"},
			linkType: &linkedType,
			linkID:   &linkedID,
			want: "SELECT * FROM files WHERE type = $1 AND category = $2 AND " +
				"linked_entity_type = $3 AND linked_entity_id = $4 AND tags @> $5 AND " +
				"(title ILIKE $6 OR description ILIKE $7 OR path ILIKE $8 OR original_path ILIKE $9)",
			args: []any{
				fileType, category, linkedType, linkedID, []byte(`["invoice"]`),
				"%receipt%", "%receipt%", "%receipt%", "%receipt%",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)

			sb := newSelect().Select("*").From("files")
			if where := fileWhere(tc.query, tc.fileType, tc.category, tc.tags, tc.linkType, tc.linkID); where != nil {
				sb.AddWhereClause(where)
			}

			got, args := sb.Build()
			c.Check(got, qt.Equals, tc.want)
			c.Check(args, qt.DeepEquals, tc.args)
		})
	}
}

// ListPaginated counts and pages off the same clause; a total describing a
// different set than the page is what the shared clause prevents.
func TestFileListPaginatedSQLSharesThePredicate(t *testing.T) {
	c := qt.New(t)

	linkedType := "commodity"
	linkedID := "com-1"
	where := fileWhere("", nil, nil, nil, &linkedType, &linkedID)
	c.Assert(where, qt.IsNotNil)

	countSB := newSelect().Select("COUNT(*)").From("files")
	countSB.AddWhereClause(where)
	countSQL, countArgs := countSB.Build()
	c.Check(countSQL, qt.Equals,
		"SELECT COUNT(*) FROM files WHERE linked_entity_type = $1 AND linked_entity_id = $2")
	c.Check(countArgs, qt.DeepEquals, []any{linkedType, linkedID})

	dataSB := newSelect().Select("*").From("files").OrderByDesc("created_at").Limit(24).Offset(48)
	dataSB.AddWhereClause(where)
	dataSQL, dataArgs := dataSB.Build()
	c.Check(dataSQL, qt.Equals,
		"SELECT * FROM files WHERE linked_entity_type = $1 AND linked_entity_id = $2 "+
			"ORDER BY created_at DESC LIMIT $3 OFFSET $4")
	c.Check(dataArgs, qt.DeepEquals, []any{linkedType, linkedID, 24, 48})
}
