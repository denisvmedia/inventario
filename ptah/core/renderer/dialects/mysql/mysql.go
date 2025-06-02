package mysql

import (
	"github.com/denisvmedia/inventario/ptah/core/ast"
	"github.com/denisvmedia/inventario/ptah/core/renderer/dialects/internal/bufwriter"
	"github.com/denisvmedia/inventario/ptah/core/renderer/dialects/mysqllike"
	"github.com/denisvmedia/inventario/ptah/core/renderer/types"
)

var (
	_ types.RenderVisitor = (*Renderer)(nil)
)

// Renderer provides MySQL-specific SQL rendering
type Renderer struct {
	r *mysqllike.Renderer
	w bufwriter.Writer
}

// New creates a new MySQL renderer
func New() *Renderer {
	var w bufwriter.Writer
	return &Renderer{
		r: mysqllike.New("mysql", &w),
		w: w,
	}
}

func (r *Renderer) VisitDropIndex(node *ast.DropIndexNode) error {
	return r.r.VisitDropIndex(node)
}

func (r *Renderer) VisitCreateType(node *ast.CreateTypeNode) error {
	return r.r.VisitCreateType(node)
}

func (r *Renderer) VisitAlterType(node *ast.AlterTypeNode) error {
	return r.r.VisitAlterType(node)
}

func (r *Renderer) Dialect() string {
	return r.r.Dialect()
}

func (r *Renderer) Reset() {
	r.r.Reset()
}

func (r *Renderer) Output() string {
	return r.r.Output()
}

// Render renders an AST node to SQL and returns the result
func (r *Renderer) Render(node ast.Node) (string, error) {
	return r.r.Render(node)
}

// GetDialect returns the database dialect (alias for Dialect for compatibility)
func (r *Renderer) GetDialect() string {
	return r.r.GetDialect()
}

// GetOutput returns the current generated SQL output (alias for Output for compatibility)
func (r *Renderer) GetOutput() string {
	return r.r.GetOutput()
}

// VisitCreateTable renders MySQL-specific CREATE TABLE statements
func (r *Renderer) VisitCreateTable(node *ast.CreateTableNode) error {
	return r.r.VisitCreateTable(node)
}

// VisitAlterTable renders MySQL-specific ALTER TABLE statements
func (r *Renderer) VisitAlterTable(node *ast.AlterTableNode) error {
	return r.r.VisitAlterTable(node)
}

// VisitColumn is called when visiting individual columns (used by other visitors)
func (r *Renderer) VisitColumn(node *ast.ColumnNode) error {
	return r.r.VisitColumn(node)
}

// VisitConstraint is called when visiting individual constraints (used by other visitors)
func (r *Renderer) VisitConstraint(node *ast.ConstraintNode) error {
	return r.r.VisitConstraint(node)
}

// VisitIndex renders a CREATE INDEX statement for MySQL
func (r *Renderer) VisitIndex(node *ast.IndexNode) error {
	return r.r.VisitIndex(node)
}

// VisitEnum renders enum handling for MySQL (inline ENUM types like MySQL)
func (r *Renderer) VisitEnum(node *ast.EnumNode) error {
	return r.r.VisitEnum(node)
}

// VisitComment renders a comment
func (r *Renderer) VisitComment(node *ast.CommentNode) error {
	return r.r.VisitComment(node)
}

// VisitDropTable renders MySQL-specific DROP TABLE statements
func (r *Renderer) VisitDropTable(node *ast.DropTableNode) error {
	return r.r.VisitDropTable(node)
}

// VisitDropType renders DROP TYPE statements for MySQL
func (r *Renderer) VisitDropType(node *ast.DropTypeNode) error {
	return r.r.VisitDropType(node)
}
