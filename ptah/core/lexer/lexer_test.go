package lexer_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/core/lexer"
)

func TestTokenType_String(t *testing.T) {
	tests := []struct {
		name     string
		token    lexer.TokenType
		expected string
	}{
		{"Unknown", lexer.TokenUnknown, "Unknown"},
		{"String", lexer.TokenString, "String"},
		{"Comment", lexer.TokenComment, "Comment"},
		{"Semicolon", lexer.TokenSemicolon, "Semicolon"},
		{"Whitespace", lexer.TokenWhitespace, "Whitespace"},
		{"Identifier", lexer.TokenIdentifier, "Identifier"},
		{"Operator", lexer.TokenOperator, "Operator"},
		{"EOF", lexer.TokenEOF, "EOF"},
		{"Invalid", lexer.TokenType(999), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			c.Assert(tt.token.String(), qt.Equals, tt.expected)
		})
	}
}

func TestNewLexer(t *testing.T) {
	c := qt.New(t)

	input := "SELECT * FROM users;"
	l := lexer.NewLexer(input)

	c.Assert(l, qt.IsNotNil)
}

func TestLexer_NextToken_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []lexer.Token
	}{
		{
			name:  "simple_select",
			input: "SELECT * FROM users;",
			expected: []lexer.Token{
				{Type: lexer.TokenIdentifier, Value: "SELECT", Start: 0, End: 6},
				{Type: lexer.TokenWhitespace, Value: " ", Start: 6, End: 7},
				{Type: lexer.TokenOperator, Value: "*", Start: 7, End: 8},
				{Type: lexer.TokenWhitespace, Value: " ", Start: 8, End: 9},
				{Type: lexer.TokenIdentifier, Value: "FROM", Start: 9, End: 13},
				{Type: lexer.TokenWhitespace, Value: " ", Start: 13, End: 14},
				{Type: lexer.TokenIdentifier, Value: "users", Start: 14, End: 19},
				{Type: lexer.TokenSemicolon, Value: ";", Start: 19, End: 20},
				{Type: lexer.TokenEOF, Value: "", Start: 20, End: 20},
			},
		},
		{
			name:  "string_literals",
			input: "'hello' \"world\"",
			expected: []lexer.Token{
				{Type: lexer.TokenString, Value: "'hello'", Start: 0, End: 7},
				{Type: lexer.TokenWhitespace, Value: " ", Start: 7, End: 8},
				{Type: lexer.TokenString, Value: "\"world\"", Start: 8, End: 15},
				{Type: lexer.TokenEOF, Value: "", Start: 15, End: 15},
			},
		},
		{
			name:  "line_comment",
			input: "SELECT -- this is a comment\nFROM users",
			expected: []lexer.Token{
				{Type: lexer.TokenIdentifier, Value: "SELECT", Start: 0, End: 6},
				{Type: lexer.TokenWhitespace, Value: " ", Start: 6, End: 7},
				{Type: lexer.TokenComment, Value: "-- this is a comment", Start: 7, End: 27},
				{Type: lexer.TokenWhitespace, Value: "\n", Start: 27, End: 28},
				{Type: lexer.TokenIdentifier, Value: "FROM", Start: 28, End: 32},
				{Type: lexer.TokenWhitespace, Value: " ", Start: 32, End: 33},
				{Type: lexer.TokenIdentifier, Value: "users", Start: 33, End: 38},
				{Type: lexer.TokenEOF, Value: "", Start: 38, End: 38},
			},
		},
		{
			name:  "block_comment",
			input: "SELECT /* multi\nline comment */ FROM users",
			expected: []lexer.Token{
				{Type: lexer.TokenIdentifier, Value: "SELECT", Start: 0, End: 6},
				{Type: lexer.TokenWhitespace, Value: " ", Start: 6, End: 7},
				{Type: lexer.TokenComment, Value: "/* multi\nline comment */", Start: 7, End: 31},
				{Type: lexer.TokenWhitespace, Value: " ", Start: 31, End: 32},
				{Type: lexer.TokenIdentifier, Value: "FROM", Start: 32, End: 36},
				{Type: lexer.TokenWhitespace, Value: " ", Start: 36, End: 37},
				{Type: lexer.TokenIdentifier, Value: "users", Start: 37, End: 42},
				{Type: lexer.TokenEOF, Value: "", Start: 42, End: 42},
			},
		},
		{
			name:  "backticked_identifier",
			input: "`table_name` `column_name`",
			expected: []lexer.Token{
				{Type: lexer.TokenIdentifier, Value: "`table_name`", Start: 0, End: 12},
				{Type: lexer.TokenWhitespace, Value: " ", Start: 12, End: 13},
				{Type: lexer.TokenIdentifier, Value: "`column_name`", Start: 13, End: 26},
				{Type: lexer.TokenEOF, Value: "", Start: 26, End: 26},
			},
		},
		{
			name:  "numbers",
			input: "123 45.67 89",
			expected: []lexer.Token{
				{Type: lexer.TokenIdentifier, Value: "123", Start: 0, End: 3},
				{Type: lexer.TokenWhitespace, Value: " ", Start: 3, End: 4},
				{Type: lexer.TokenIdentifier, Value: "45.67", Start: 4, End: 9},
				{Type: lexer.TokenWhitespace, Value: " ", Start: 9, End: 10},
				{Type: lexer.TokenIdentifier, Value: "89", Start: 10, End: 12},
				{Type: lexer.TokenEOF, Value: "", Start: 12, End: 12},
			},
		},
		{
			name:  "operators",
			input: "= != < > <= >= + - * / ( ) , .",
			expected: []lexer.Token{
				{Type: lexer.TokenOperator, Value: "=", Start: 0, End: 1},
				{Type: lexer.TokenWhitespace, Value: " ", Start: 1, End: 2},
				{Type: lexer.TokenOperator, Value: "!", Start: 2, End: 3},
				{Type: lexer.TokenOperator, Value: "=", Start: 3, End: 4},
				{Type: lexer.TokenWhitespace, Value: " ", Start: 4, End: 5},
				{Type: lexer.TokenOperator, Value: "<", Start: 5, End: 6},
				{Type: lexer.TokenWhitespace, Value: " ", Start: 6, End: 7},
				{Type: lexer.TokenOperator, Value: ">", Start: 7, End: 8},
				{Type: lexer.TokenWhitespace, Value: " ", Start: 8, End: 9},
				{Type: lexer.TokenOperator, Value: "<", Start: 9, End: 10},
				{Type: lexer.TokenOperator, Value: "=", Start: 10, End: 11},
				{Type: lexer.TokenWhitespace, Value: " ", Start: 11, End: 12},
				{Type: lexer.TokenOperator, Value: ">", Start: 12, End: 13},
				{Type: lexer.TokenOperator, Value: "=", Start: 13, End: 14},
				{Type: lexer.TokenWhitespace, Value: " ", Start: 14, End: 15},
				{Type: lexer.TokenOperator, Value: "+", Start: 15, End: 16},
				{Type: lexer.TokenWhitespace, Value: " ", Start: 16, End: 17},
				{Type: lexer.TokenOperator, Value: "-", Start: 17, End: 18},
				{Type: lexer.TokenWhitespace, Value: " ", Start: 18, End: 19},
				{Type: lexer.TokenOperator, Value: "*", Start: 19, End: 20},
				{Type: lexer.TokenWhitespace, Value: " ", Start: 20, End: 21},
				{Type: lexer.TokenOperator, Value: "/", Start: 21, End: 22},
				{Type: lexer.TokenWhitespace, Value: " ", Start: 22, End: 23},
				{Type: lexer.TokenOperator, Value: "(", Start: 23, End: 24},
				{Type: lexer.TokenWhitespace, Value: " ", Start: 24, End: 25},
				{Type: lexer.TokenOperator, Value: ")", Start: 25, End: 26},
				{Type: lexer.TokenWhitespace, Value: " ", Start: 26, End: 27},
				{Type: lexer.TokenOperator, Value: ",", Start: 27, End: 28},
				{Type: lexer.TokenWhitespace, Value: " ", Start: 28, End: 29},
				{Type: lexer.TokenOperator, Value: ".", Start: 29, End: 30},
				{Type: lexer.TokenEOF, Value: "", Start: 30, End: 30},
			},
		},
		{
			name:  "empty_input",
			input: "",
			expected: []lexer.Token{
				{Type: lexer.TokenEOF, Value: "", Start: 0, End: 0},
			},
		},
		{
			name:  "whitespace_only",
			input: "   \t\n\r  ",
			expected: []lexer.Token{
				{Type: lexer.TokenWhitespace, Value: "   \t\n\r  ", Start: 0, End: 8},
				{Type: lexer.TokenEOF, Value: "", Start: 8, End: 8},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			l := lexer.NewLexer(tt.input)

			var tokens []lexer.Token
			for {
				token := l.NextToken()
				tokens = append(tokens, token)
				if token.Type == lexer.TokenEOF {
					break
				}
			}

			c.Assert(tokens, qt.DeepEquals, tt.expected)
		})
	}
}

func TestLexer_NextToken_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []lexer.Token
	}{
		{
			name:  "unterminated_string_single_quote",
			input: "'unterminated string",
			expected: []lexer.Token{
				{Type: lexer.TokenString, Value: "'unterminated string", Start: 0, End: 20},
				{Type: lexer.TokenEOF, Value: "", Start: 20, End: 20},
			},
		},
		{
			name:  "unterminated_string_double_quote",
			input: "\"unterminated string",
			expected: []lexer.Token{
				{Type: lexer.TokenString, Value: "\"unterminated string", Start: 0, End: 20},
				{Type: lexer.TokenEOF, Value: "", Start: 20, End: 20},
			},
		},
		{
			name:  "unterminated_backticked_identifier",
			input: "`unterminated_identifier",
			expected: []lexer.Token{
				{Type: lexer.TokenIdentifier, Value: "`unterminated_identifier", Start: 0, End: 24},
				{Type: lexer.TokenEOF, Value: "", Start: 24, End: 24},
			},
		},
		{
			name:  "unterminated_block_comment",
			input: "/* unterminated comment",
			expected: []lexer.Token{
				{Type: lexer.TokenComment, Value: "/* unterminated comment", Start: 0, End: 23},
				{Type: lexer.TokenEOF, Value: "", Start: 23, End: 23},
			},
		},
		{
			name:  "escaped_characters_in_string",
			input: "'escaped \\'quote\\' and \\\\backslash'",
			expected: []lexer.Token{
				{Type: lexer.TokenString, Value: "'escaped \\'quote\\' and \\\\backslash'", Start: 0, End: 35},
				{Type: lexer.TokenEOF, Value: "", Start: 35, End: 35},
			},
		},
		{
			name:  "escaped_characters_in_backticked_identifier",
			input: "`escaped \\`backtick\\` and \\\\backslash`",
			expected: []lexer.Token{
				{Type: lexer.TokenIdentifier, Value: "`escaped \\`backtick\\` and \\\\backslash`", Start: 0, End: 38},
				{Type: lexer.TokenEOF, Value: "", Start: 38, End: 38},
			},
		},
		{
			name:  "escape_at_end_of_input",
			input: "'string with escape at end\\",
			expected: []lexer.Token{
				{Type: lexer.TokenString, Value: "'string with escape at end\\", Start: 0, End: 27},
				{Type: lexer.TokenEOF, Value: "", Start: 27, End: 27},
			},
		},
		{
			name:  "line_comment_with_carriage_return",
			input: "-- comment\rSELECT",
			expected: []lexer.Token{
				{Type: lexer.TokenComment, Value: "-- comment", Start: 0, End: 10},
				{Type: lexer.TokenWhitespace, Value: "\r", Start: 10, End: 11},
				{Type: lexer.TokenIdentifier, Value: "SELECT", Start: 11, End: 17},
				{Type: lexer.TokenEOF, Value: "", Start: 17, End: 17},
			},
		},
		{
			name:  "identifier_with_underscores_and_numbers",
			input: "_table_name_123 column_456_",
			expected: []lexer.Token{
				{Type: lexer.TokenIdentifier, Value: "_table_name_123", Start: 0, End: 15},
				{Type: lexer.TokenWhitespace, Value: " ", Start: 15, End: 16},
				{Type: lexer.TokenIdentifier, Value: "column_456_", Start: 16, End: 27},
				{Type: lexer.TokenEOF, Value: "", Start: 27, End: 27},
			},
		},
		{
			name:  "single_dash_not_comment",
			input: "- not a comment",
			expected: []lexer.Token{
				{Type: lexer.TokenOperator, Value: "-", Start: 0, End: 1},
				{Type: lexer.TokenWhitespace, Value: " ", Start: 1, End: 2},
				{Type: lexer.TokenIdentifier, Value: "not", Start: 2, End: 5},
				{Type: lexer.TokenWhitespace, Value: " ", Start: 5, End: 6},
				{Type: lexer.TokenIdentifier, Value: "a", Start: 6, End: 7},
				{Type: lexer.TokenWhitespace, Value: " ", Start: 7, End: 8},
				{Type: lexer.TokenIdentifier, Value: "comment", Start: 8, End: 15},
				{Type: lexer.TokenEOF, Value: "", Start: 15, End: 15},
			},
		},
		{
			name:  "single_slash_not_comment",
			input: "/ not a comment",
			expected: []lexer.Token{
				{Type: lexer.TokenOperator, Value: "/", Start: 0, End: 1},
				{Type: lexer.TokenWhitespace, Value: " ", Start: 1, End: 2},
				{Type: lexer.TokenIdentifier, Value: "not", Start: 2, End: 5},
				{Type: lexer.TokenWhitespace, Value: " ", Start: 5, End: 6},
				{Type: lexer.TokenIdentifier, Value: "a", Start: 6, End: 7},
				{Type: lexer.TokenWhitespace, Value: " ", Start: 7, End: 8},
				{Type: lexer.TokenIdentifier, Value: "comment", Start: 8, End: 15},
				{Type: lexer.TokenEOF, Value: "", Start: 15, End: 15},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			l := lexer.NewLexer(tt.input)

			var tokens []lexer.Token
			for {
				token := l.NextToken()
				tokens = append(tokens, token)
				if token.Type == lexer.TokenEOF {
					break
				}
			}

			c.Assert(tokens, qt.DeepEquals, tt.expected)
		})
	}
}

func TestLexer_NextToken_ComplexSQL(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedTypes []lexer.TokenType
	}{
		{
			name: "create_table_statement",
			input: `CREATE TABLE users (
				id INTEGER PRIMARY KEY,
				name VARCHAR(255) NOT NULL,
				email VARCHAR(255) UNIQUE
			);`,
			expectedTypes: []lexer.TokenType{
				lexer.TokenIdentifier, // CREATE
				lexer.TokenWhitespace,
				lexer.TokenIdentifier, // TABLE
				lexer.TokenWhitespace,
				lexer.TokenIdentifier, // users
				lexer.TokenWhitespace,
				lexer.TokenOperator, // (
				lexer.TokenWhitespace,
				lexer.TokenIdentifier, // id
				lexer.TokenWhitespace,
				lexer.TokenIdentifier, // INTEGER
				lexer.TokenWhitespace,
				lexer.TokenIdentifier, // PRIMARY
				lexer.TokenWhitespace,
				lexer.TokenIdentifier, // KEY
				lexer.TokenOperator,   // ,
				lexer.TokenWhitespace,
				lexer.TokenIdentifier, // name
				lexer.TokenWhitespace,
				lexer.TokenIdentifier, // VARCHAR
				lexer.TokenOperator,   // (
				lexer.TokenIdentifier, // 255
				lexer.TokenOperator,   // )
				lexer.TokenWhitespace,
				lexer.TokenIdentifier, // NOT
				lexer.TokenWhitespace,
				lexer.TokenIdentifier, // NULL
				lexer.TokenOperator,   // ,
				lexer.TokenWhitespace,
				lexer.TokenIdentifier, // email
				lexer.TokenWhitespace,
				lexer.TokenIdentifier, // VARCHAR
				lexer.TokenOperator,   // (
				lexer.TokenIdentifier, // 255
				lexer.TokenOperator,   // )
				lexer.TokenWhitespace,
				lexer.TokenIdentifier, // UNIQUE
				lexer.TokenWhitespace,
				lexer.TokenOperator, // )
				lexer.TokenSemicolon,
				lexer.TokenEOF,
			},
		},
		{
			name: "complex_select_with_comments",
			input: `-- Get user information
			SELECT u.id, u.name, /* inline comment */ u.email
			FROM users u
			WHERE u.active = 'true' -- only active users
			ORDER BY u.name;`,
			expectedTypes: []lexer.TokenType{
				lexer.TokenComment,    // -- Get user information
				lexer.TokenWhitespace,
				lexer.TokenIdentifier, // SELECT
				lexer.TokenWhitespace,
				lexer.TokenIdentifier, // u
				lexer.TokenOperator,   // .
				lexer.TokenIdentifier, // id
				lexer.TokenOperator,   // ,
				lexer.TokenWhitespace,
				lexer.TokenIdentifier, // u
				lexer.TokenOperator,   // .
				lexer.TokenIdentifier, // name
				lexer.TokenOperator,   // ,
				lexer.TokenWhitespace,
				lexer.TokenComment, // /* inline comment */
				lexer.TokenWhitespace,
				lexer.TokenIdentifier, // u
				lexer.TokenOperator,   // .
				lexer.TokenIdentifier, // email
				lexer.TokenWhitespace,
				lexer.TokenIdentifier, // FROM
				lexer.TokenWhitespace,
				lexer.TokenIdentifier, // users
				lexer.TokenWhitespace,
				lexer.TokenIdentifier, // u
				lexer.TokenWhitespace,
				lexer.TokenIdentifier, // WHERE
				lexer.TokenWhitespace,
				lexer.TokenIdentifier, // u
				lexer.TokenOperator,   // .
				lexer.TokenIdentifier, // active
				lexer.TokenWhitespace,
				lexer.TokenOperator, // =
				lexer.TokenWhitespace,
				lexer.TokenString, // 'true'
				lexer.TokenWhitespace,
				lexer.TokenComment, // -- only active users
				lexer.TokenWhitespace,
				lexer.TokenIdentifier, // ORDER
				lexer.TokenWhitespace,
				lexer.TokenIdentifier, // BY
				lexer.TokenWhitespace,
				lexer.TokenIdentifier, // u
				lexer.TokenOperator,   // .
				lexer.TokenIdentifier, // name
				lexer.TokenSemicolon,
				lexer.TokenEOF,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			l := lexer.NewLexer(tt.input)

			var tokenTypes []lexer.TokenType
			for {
				token := l.NextToken()
				tokenTypes = append(tokenTypes, token.Type)
				if token.Type == lexer.TokenEOF {
					break
				}
			}

			c.Assert(tokenTypes, qt.DeepEquals, tt.expectedTypes)
		})
	}
}

func TestLexer_MultipleTokenCalls(t *testing.T) {
	c := qt.New(t)

	input := "abc"
	l := lexer.NewLexer(input)

	// Test that multiple calls to NextToken work correctly
	token1 := l.NextToken()
	c.Assert(token1.Type, qt.Equals, lexer.TokenIdentifier)
	c.Assert(token1.Value, qt.Equals, "abc")
	c.Assert(token1.Start, qt.Equals, 0)
	c.Assert(token1.End, qt.Equals, 3)

	// Second call should return EOF
	token2 := l.NextToken()
	c.Assert(token2.Type, qt.Equals, lexer.TokenEOF)
	c.Assert(token2.Value, qt.Equals, "")
	c.Assert(token2.Start, qt.Equals, 3)
	c.Assert(token2.End, qt.Equals, 3)

	// Subsequent calls should continue to return EOF
	token3 := l.NextToken()
	c.Assert(token3.Type, qt.Equals, lexer.TokenEOF)
	c.Assert(token3.Value, qt.Equals, "")
}

func TestLexer_TokenPositions(t *testing.T) {
	c := qt.New(t)

	input := "SELECT id FROM users;"
	l := lexer.NewLexer(input)

	// Test that token positions are correct
	token1 := l.NextToken() // SELECT
	c.Assert(token1.Type, qt.Equals, lexer.TokenIdentifier)
	c.Assert(token1.Value, qt.Equals, "SELECT")
	c.Assert(token1.Start, qt.Equals, 0)
	c.Assert(token1.End, qt.Equals, 6)

	token2 := l.NextToken() // whitespace
	c.Assert(token2.Type, qt.Equals, lexer.TokenWhitespace)
	c.Assert(token2.Start, qt.Equals, 6)
	c.Assert(token2.End, qt.Equals, 7)

	token3 := l.NextToken() // id
	c.Assert(token3.Type, qt.Equals, lexer.TokenIdentifier)
	c.Assert(token3.Value, qt.Equals, "id")
	c.Assert(token3.Start, qt.Equals, 7)
	c.Assert(token3.End, qt.Equals, 9)
}

func BenchmarkLexer_SimpleSQL(b *testing.B) {
	input := "SELECT id, name, email FROM users WHERE active = 'true' ORDER BY name;"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l := lexer.NewLexer(input)
		for {
			token := l.NextToken()
			if token.Type == lexer.TokenEOF {
				break
			}
		}
	}
}

func BenchmarkLexer_ComplexSQL(b *testing.B) {
	input := `-- Complex query with comments
	SELECT u.id, u.name, p.title, /* inline comment */ c.name as category
	FROM users u
	JOIN posts p ON u.id = p.user_id
	JOIN categories c ON p.category_id = c.id
	WHERE u.active = 'true' AND p.published = 'true'
	ORDER BY u.name, p.created_at DESC;`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l := lexer.NewLexer(input)
		for {
			token := l.NextToken()
			if token.Type == lexer.TokenEOF {
				break
			}
		}
	}
}

func BenchmarkLexer_LargeSQL(b *testing.B) {
	// Generate a large SQL statement
	input := "CREATE TABLE large_table ("
	for i := 0; i < 100; i++ {
		if i > 0 {
			input += ", "
		}
		input += "column_" + string(rune('0'+i%10)) + " VARCHAR(255)"
	}
	input += ");"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l := lexer.NewLexer(input)
		for {
			token := l.NextToken()
			if token.Type == lexer.TokenEOF {
				break
			}
		}
	}
}
