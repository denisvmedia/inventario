package postgres

import sqlb "github.com/huandu/go-sqlbuilder"

// Dynamic WHERE and ORDER BY in this package go through go-sqlbuilder: it
// numbers the placeholders, and one WhereClause can serve both the count and
// the data query of a paginated list so the total always describes the page.
//
// Values reach the database only as arguments. Column names, table names and
// sort directions are written into the SQL text, so they must come from code:
// a constant, a TableNames method or a switch over a validated enum, never a
// request parameter.

// newSelect starts a PostgreSQL SELECT, so the `$N` flavor is chosen in one
// place rather than at each call site.
func newSelect() *sqlb.SelectBuilder {
	return sqlb.PostgreSQL.NewSelectBuilder()
}

// andWhere joins the non-empty exprs with AND into one WHERE clause, whose
// arguments are held by cond. It returns nil when no expr is left, see
// addWhere.
func andWhere(cond *sqlb.Cond, exprs ...string) *sqlb.WhereClause {
	wc := sqlb.NewWhereClause()
	empty := true
	for _, expr := range exprs {
		if expr == "" {
			continue
		}
		wc.AddWhereExpr(cond.Args, expr)
		empty = false
	}
	if empty {
		return nil
	}
	return wc
}

// addWhere attaches wc to sb. A nil wc leaves sb without a WHERE: attaching an
// empty clause would leave a stray space where WHERE would have been.
func addWhere(sb *sqlb.SelectBuilder, wc *sqlb.WhereClause) *sqlb.SelectBuilder {
	if wc != nil {
		sb.AddWhereClause(wc)
	}
	return sb
}

// paginate sets LIMIT and OFFSET with negative values clamped to zero.
// go-sqlbuilder drops a negative LIMIT altogether, which would turn a bad
// limit into every row of the table.
func paginate(sb *sqlb.SelectBuilder, offset, limit int) *sqlb.SelectBuilder {
	return sb.Limit(max(limit, 0)).Offset(max(offset, 0))
}

// anyStrings widens a slice of string-kinded values for a variadic IN.
func anyStrings[T ~string](in []T) []any {
	out := make([]any, len(in))
	for i, v := range in {
		out[i] = string(v)
	}
	return out
}
