package postgres

import sqlb "github.com/huandu/go-sqlbuilder"

// Dynamic WHERE and ORDER BY in this package go through go-sqlbuilder: it
// numbers the placeholders, and one WhereClause can serve both the count and
// the data query of a paginated list so the total always describes the page.

// newSelect starts a PostgreSQL SELECT, so the `$N` flavor is chosen in one
// place rather than at each call site.
func newSelect() *sqlb.SelectBuilder {
	return sqlb.PostgreSQL.NewSelectBuilder()
}

// anyStrings widens a slice of string-kinded values for a variadic IN.
func anyStrings[T ~string](in []T) []any {
	out := make([]any, len(in))
	for i, v := range in {
		out[i] = string(v)
	}
	return out
}
