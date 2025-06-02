package sqlutil

import (
	"strings"

	"github.com/denisvmedia/inventario/ptah/core/lexer"
)

// StripComments removes all SQL comments from the input string using lexer-based parsing.
// This properly handles comments within string literals and preserves the structure of the SQL.
// Both line comments (-- comment) and block comments (/* comment */) are removed.
func StripComments(sql string) string {
	if strings.TrimSpace(sql) == "" {
		return sql
	}

	lexr := lexer.NewLexer(sql)
	var result strings.Builder

	for {
		token := lexr.NextToken()

		if token.Type == lexer.TokenEOF {
			break
		}

		// Skip comment tokens, include everything else
		if token.Type != lexer.TokenComment {
			result.WriteString(token.Value)
		}
	}

	return result.String()
}

// SplitSQLStatements splits a SQL string into individual statements using AST-based parsing.
// This properly handles semicolons within string literals and comments, unlike simple string splitting.
func SplitSQLStatements(sql string) []string {
	if strings.TrimSpace(sql) == "" {
		return []string{}
	}

	lexr := lexer.NewLexer(sql)
	var statements []string
	var currentStatement strings.Builder

	for {
		token := lexr.NextToken()

		if token.Type == lexer.TokenEOF {
			break
		}

		if token.Type == lexer.TokenSemicolon {
			// Found a statement terminator - add current statement if not empty
			stmt := strings.TrimSpace(currentStatement.String())
			if stmt != "" {
				statements = append(statements, stmt)
			}
			currentStatement.Reset()
		} else {
			// Add token to current statement
			currentStatement.WriteString(token.Value)
		}
	}

	// Add any remaining statement
	stmt := strings.TrimSpace(currentStatement.String())
	if stmt != "" {
		statements = append(statements, stmt)
	}

	// Ensure we always return a non-nil slice
	if statements == nil {
		return []string{}
	}

	return statements
}
