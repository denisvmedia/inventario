package sqlsplitter_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/migrator/sqlsplitter"
)

func TestSplitSQLStatements_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "single statement",
			input:    "CREATE TABLE users (id INT PRIMARY KEY)",
			expected: []string{"CREATE TABLE users (id INT PRIMARY KEY)"},
		},
		{
			name:  "multiple simple statements",
			input: "CREATE TABLE users (id INT); CREATE TABLE posts (id INT);",
			expected: []string{
				"CREATE TABLE users (id INT)",
				"CREATE TABLE posts (id INT)",
			},
		},
		{
			name: "statements with whitespace",
			input: `
				CREATE TABLE users (id INT PRIMARY KEY);
				
				CREATE TABLE posts (id INT, user_id INT);
			`,
			expected: []string{
				"CREATE TABLE users (id INT PRIMARY KEY)",
				"CREATE TABLE posts (id INT, user_id INT)",
			},
		},
		{
			name:  "statements with line comments",
			input: "-- Create users table\nCREATE TABLE users (id INT); -- Create posts table\nCREATE TABLE posts (id INT);",
			expected: []string{
				"-- Create users table\nCREATE TABLE users (id INT)",
				"-- Create posts table\nCREATE TABLE posts (id INT)",
			},
		},
		{
			name:  "statements with block comments",
			input: "/* Users table */ CREATE TABLE users (id INT); /* Posts table */ CREATE TABLE posts (id INT);",
			expected: []string{
				"/* Users table */ CREATE TABLE users (id INT)",
				"/* Posts table */ CREATE TABLE posts (id INT)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			result := sqlsplitter.SplitSQLStatements(tt.input)
			c.Assert(result, qt.DeepEquals, tt.expected)
		})
	}
}

func TestSplitSQLStatements_StringLiterals(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "semicolon in single quoted string",
			input:    "INSERT INTO users (name) VALUES ('John; Doe')",
			expected: []string{"INSERT INTO users (name) VALUES ('John; Doe')"},
		},
		{
			name:     "semicolon in double quoted string",
			input:    `INSERT INTO users (name) VALUES ("John; Doe")`,
			expected: []string{`INSERT INTO users (name) VALUES ("John; Doe")`},
		},
		{
			name:  "multiple statements with semicolons in strings",
			input: "INSERT INTO users (name) VALUES ('John; Doe'); INSERT INTO users (name) VALUES ('Jane; Smith');",
			expected: []string{
				"INSERT INTO users (name) VALUES ('John; Doe')",
				"INSERT INTO users (name) VALUES ('Jane; Smith')",
			},
		},
		{
			name:     "escaped quotes in strings",
			input:    `INSERT INTO users (name) VALUES ('John\'s; data')`,
			expected: []string{`INSERT INTO users (name) VALUES ('John\'s; data')`},
		},
		{
			name:  "complex example with strings and comments",
			input: "-- POSTGRES TABLE: products --\nCREATE TABLE products (\n  id SERIAL PRIMARY KEY NOT NULL,\n  name VARCHAR(255) NOT NULL DEFAULT 'default; value',\n  description TEXT\n);\n\n-- POSTGRES TABLE: users --\nCREATE TABLE users (\n  id SERIAL PRIMARY KEY NOT NULL,\n  email VARCHAR(255) UNIQUE NOT NULL,\n  bio TEXT DEFAULT 'No bio; available'\n);",
			expected: []string{
				"-- POSTGRES TABLE: products --\nCREATE TABLE products (\n  id SERIAL PRIMARY KEY NOT NULL,\n  name VARCHAR(255) NOT NULL DEFAULT 'default; value',\n  description TEXT\n)",
				"-- POSTGRES TABLE: users --\nCREATE TABLE users (\n  id SERIAL PRIMARY KEY NOT NULL,\n  email VARCHAR(255) UNIQUE NOT NULL,\n  bio TEXT DEFAULT 'No bio; available'\n)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			result := sqlsplitter.SplitSQLStatements(tt.input)
			c.Assert(result, qt.DeepEquals, tt.expected)
		})
	}
}

func TestSplitSQLStatements_Comments(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "semicolon in line comment",
			input:    "CREATE TABLE users (id INT) -- comment with; semicolon",
			expected: []string{"CREATE TABLE users (id INT) -- comment with; semicolon"},
		},
		{
			name:     "semicolon in block comment",
			input:    "CREATE TABLE users (id INT) /* comment with; semicolon */",
			expected: []string{"CREATE TABLE users (id INT) /* comment with; semicolon */"},
		},
		{
			name:  "multiple statements with comments containing semicolons",
			input: "CREATE TABLE users (id INT) -- comment; here\n; CREATE TABLE posts (id INT) /* another; comment */;",
			expected: []string{
				"CREATE TABLE users (id INT) -- comment; here",
				"CREATE TABLE posts (id INT) /* another; comment */",
			},
		},
		{
			name:  "multiline block comment with semicolons",
			input: "CREATE TABLE users (id INT) /* \n  multiline comment;\n  with semicolons;\n */; CREATE TABLE posts (id INT);",
			expected: []string{
				"CREATE TABLE users (id INT) /* \n  multiline comment;\n  with semicolons;\n */",
				"CREATE TABLE posts (id INT)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			result := sqlsplitter.SplitSQLStatements(tt.input)
			c.Assert(result, qt.DeepEquals, tt.expected)
		})
	}
}

func TestSplitSQLStatements_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: []string{},
		},
		{
			name:     "only whitespace",
			input:    "   \n\t  ",
			expected: []string{},
		},
		{
			name:     "only semicolons",
			input:    ";;;",
			expected: []string{},
		},
		{
			name:     "only comments",
			input:    "-- just a comment\n/* another comment */",
			expected: []string{"-- just a comment\n/* another comment */"},
		},
		{
			name:     "unterminated string",
			input:    "INSERT INTO users (name) VALUES ('unterminated",
			expected: []string{"INSERT INTO users (name) VALUES ('unterminated"},
		},
		{
			name:     "unterminated block comment",
			input:    "CREATE TABLE users (id INT) /* unterminated comment",
			expected: []string{"CREATE TABLE users (id INT) /* unterminated comment"},
		},
		{
			name:  "trailing semicolon",
			input: "CREATE TABLE users (id INT);",
			expected: []string{
				"CREATE TABLE users (id INT)",
			},
		},
		{
			name:  "multiple trailing semicolons",
			input: "CREATE TABLE users (id INT);;;",
			expected: []string{
				"CREATE TABLE users (id INT)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			result := sqlsplitter.SplitSQLStatements(tt.input)
			c.Assert(result, qt.DeepEquals, tt.expected)
		})
	}
}

func TestSplitSQLStatements_RealWorldExample(t *testing.T) {
	c := qt.New(t)

	// This is the exact example from the user's issue
	input := ` -- POSTGRES TABLE: products --
CREATE TABLE products (
  id SERIAL PRIMARY KEY NOT NULL,
  active BOOLEAN NOT NULL DEFAULT 'true',
  name VARCHAR(255) NOT NULL,
  price DECIMAL(10,2) NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  category VARCHAR(100),
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  description TEXT
);


-- POSTGRES TABLE: users --
CREATE TABLE users (
  id SERIAL PRIMARY KEY NOT NULL,
  active BOOLEAN NOT NULL DEFAULT 'true',
  name VARCHAR(255) NOT NULL,
  age INTEGER,
  email VARCHAR(255) UNIQUE NOT NULL,
  bio TEXT,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);`

	expected := []string{
		`-- POSTGRES TABLE: products --
CREATE TABLE products (
  id SERIAL PRIMARY KEY NOT NULL,
  active BOOLEAN NOT NULL DEFAULT 'true',
  name VARCHAR(255) NOT NULL,
  price DECIMAL(10,2) NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  category VARCHAR(100),
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  description TEXT
)`,
		`-- POSTGRES TABLE: users --
CREATE TABLE users (
  id SERIAL PRIMARY KEY NOT NULL,
  active BOOLEAN NOT NULL DEFAULT 'true',
  name VARCHAR(255) NOT NULL,
  age INTEGER,
  email VARCHAR(255) UNIQUE NOT NULL,
  bio TEXT,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
)`,
	}

	result := sqlsplitter.SplitSQLStatements(input)
	c.Assert(result, qt.DeepEquals, expected)
}

func TestRemoveComments_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no comments",
			input:    "SELECT * FROM users WHERE id = 1",
			expected: "SELECT * FROM users WHERE id = 1",
		},
		{
			name:     "single line comment at end",
			input:    "SELECT * FROM users -- get all users",
			expected: "SELECT * FROM users ",
		},
		{
			name:     "single line comment at beginning",
			input:    "-- get all users\nSELECT * FROM users",
			expected: "\nSELECT * FROM users",
		},
		{
			name:     "single line comment in middle",
			input:    "SELECT * FROM users -- get all users\nWHERE id = 1",
			expected: "SELECT * FROM users \nWHERE id = 1",
		},
		{
			name:     "multiple line comments",
			input:    "-- First comment\nSELECT * FROM users -- Second comment\nWHERE id = 1 -- Third comment",
			expected: "\nSELECT * FROM users \nWHERE id = 1 ",
		},
		{
			name:     "simple block comment",
			input:    "SELECT * FROM users /* get all users */ WHERE id = 1",
			expected: "SELECT * FROM users  WHERE id = 1",
		},
		{
			name:     "block comment at beginning",
			input:    "/* get all users */ SELECT * FROM users",
			expected: " SELECT * FROM users",
		},
		{
			name:     "block comment at end",
			input:    "SELECT * FROM users /* get all users */",
			expected: "SELECT * FROM users ",
		},
		{
			name:     "multiple block comments",
			input:    "/* First */ SELECT * FROM users /* Second */ WHERE id = 1 /* Third */",
			expected: " SELECT * FROM users  WHERE id = 1 ",
		},
		{
			name:     "mixed line and block comments",
			input:    "-- Line comment\nSELECT * FROM users /* block comment */ WHERE id = 1 -- another line",
			expected: "\nSELECT * FROM users  WHERE id = 1 ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			result := sqlsplitter.RemoveComments(tt.input)
			c.Assert(result, qt.Equals, tt.expected)
		})
	}
}

func TestRemoveComments_StringLiterals(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "line comment syntax in single quoted string",
			input:    "INSERT INTO users (name) VALUES ('John -- not a comment')",
			expected: "INSERT INTO users (name) VALUES ('John -- not a comment')",
		},
		{
			name:     "line comment syntax in double quoted string",
			input:    `INSERT INTO users (name) VALUES ("John -- not a comment")`,
			expected: `INSERT INTO users (name) VALUES ("John -- not a comment")`,
		},
		{
			name:     "block comment syntax in single quoted string",
			input:    "INSERT INTO users (name) VALUES ('John /* not a comment */ Doe')",
			expected: "INSERT INTO users (name) VALUES ('John /* not a comment */ Doe')",
		},
		{
			name:     "block comment syntax in double quoted string",
			input:    `INSERT INTO users (name) VALUES ("John /* not a comment */ Doe")`,
			expected: `INSERT INTO users (name) VALUES ("John /* not a comment */ Doe")`,
		},
		{
			name:     "real comment after string with comment syntax",
			input:    "INSERT INTO users (name) VALUES ('John -- fake') -- real comment",
			expected: "INSERT INTO users (name) VALUES ('John -- fake') ",
		},
		{
			name:     "escaped quotes with comment syntax",
			input:    `INSERT INTO users (name) VALUES ('John\'s -- data') -- real comment`,
			expected: `INSERT INTO users (name) VALUES ('John\'s -- data') `,
		},
		{
			name:     "complex string with both comment types",
			input:    `INSERT INTO users (bio) VALUES ('Bio: -- line /* block */ comment') /* real comment */`,
			expected: `INSERT INTO users (bio) VALUES ('Bio: -- line /* block */ comment') `,
		},
		{
			name:     "multiple strings with comments between",
			input:    "INSERT INTO users (first, last) VALUES ('John' -- comment\n, 'Doe')",
			expected: "INSERT INTO users (first, last) VALUES ('John' \n, 'Doe')",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			result := sqlsplitter.RemoveComments(tt.input)
			c.Assert(result, qt.Equals, tt.expected)
		})
	}
}

func TestRemoveComments_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only whitespace",
			input:    "   \n\t  ",
			expected: "   \n\t  ",
		},
		{
			name:     "only line comment",
			input:    "-- just a comment",
			expected: "",
		},
		{
			name:     "only block comment",
			input:    "/* just a comment */",
			expected: "",
		},
		{
			name:     "multiple only comments",
			input:    "-- first\n/* second */ -- third",
			expected: "\n ",
		},
		{
			name:     "unterminated line comment at EOF",
			input:    "SELECT * FROM users -- comment without newline",
			expected: "SELECT * FROM users ",
		},
		{
			name:     "unterminated block comment",
			input:    "SELECT * FROM users /* unterminated comment",
			expected: "SELECT * FROM users ",
		},
		{
			name:     "nested block comment syntax",
			input:    "SELECT * FROM users /* outer /* inner */ still in comment */",
			expected: "SELECT * FROM users  still in comment */",
		},
		{
			name:     "line comment with carriage return",
			input:    "SELECT * FROM users -- comment\r\nWHERE id = 1",
			expected: "SELECT * FROM users \r\nWHERE id = 1",
		},
		{
			name:     "block comment spanning multiple lines",
			input:    "SELECT * FROM users /*\n  multiline\n  comment\n*/ WHERE id = 1",
			expected: "SELECT * FROM users  WHERE id = 1",
		},
		{
			name:     "comment with special characters",
			input:    "SELECT * FROM users -- comment with émojis 🚀 and unicode ñ",
			expected: "SELECT * FROM users ",
		},
		{
			name:     "empty block comment",
			input:    "SELECT * FROM users /**/ WHERE id = 1",
			expected: "SELECT * FROM users  WHERE id = 1",
		},
		{
			name:     "comment with SQL keywords",
			input:    "SELECT * FROM users -- SELECT * FROM posts WHERE id = 1",
			expected: "SELECT * FROM users ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			result := sqlsplitter.RemoveComments(tt.input)
			c.Assert(result, qt.Equals, tt.expected)
		})
	}
}

func TestRemoveComments_ComplexScenarios(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "real world CREATE TABLE with comments",
			input: `-- Create users table
CREATE TABLE users (
  id SERIAL PRIMARY KEY NOT NULL, -- Primary key
  name VARCHAR(255) NOT NULL, /* User's full name */
  email VARCHAR(255) UNIQUE NOT NULL -- Must be unique
); -- End of table`,
			expected: `
CREATE TABLE users (
  id SERIAL PRIMARY KEY NOT NULL, 
  name VARCHAR(255) NOT NULL, 
  email VARCHAR(255) UNIQUE NOT NULL 
); `,
		},
		{
			name: "mixed comments with string literals",
			input: `-- Insert test data
INSERT INTO users (name, bio) VALUES
  ('John Doe', 'Bio: -- not a comment'), -- First user
  ('Jane Smith', 'Bio: /* also not a comment */'); /* End insert */`,
			expected: `
INSERT INTO users (name, bio) VALUES
  ('John Doe', 'Bio: -- not a comment'), 
  ('Jane Smith', 'Bio: /* also not a comment */'); `,
		},
		{
			name: "complex query with multiple comment types",
			input: `-- Get user statistics
SELECT
  u.name, -- User name
  COUNT(p.id) as post_count /* Total posts */
FROM users u /* Users table */
LEFT JOIN posts p ON u.id = p.user_id -- Join condition
WHERE u.active = true -- Only active users
GROUP BY u.id, u.name /* Group by user */
ORDER BY post_count DESC; -- Sort by post count`,
			expected: `
SELECT
  u.name, 
  COUNT(p.id) as post_count 
FROM users u 
LEFT JOIN posts p ON u.id = p.user_id 
WHERE u.active = true 
GROUP BY u.id, u.name 
ORDER BY post_count DESC; `,
		},
		{
			name:     "comments with escaped characters in strings",
			input:    `INSERT INTO logs (message) VALUES ('Error: can\'t connect -- not a comment'); -- Real comment`,
			expected: `INSERT INTO logs (message) VALUES ('Error: can\'t connect -- not a comment'); `,
		},
		{
			name:     "block comment with asterisks inside",
			input:    `SELECT * FROM users /* comment with * asterisks * inside */ WHERE id = 1`,
			expected: `SELECT * FROM users  WHERE id = 1`,
		},
		{
			name: "line comment followed immediately by block comment",
			input: `SELECT * FROM users -- line comment
/* block comment */ WHERE id = 1`,
			expected: `SELECT * FROM users 
 WHERE id = 1`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			result := sqlsplitter.RemoveComments(tt.input)
			c.Assert(result, qt.Equals, tt.expected)
		})
	}
}

func TestRemoveComments_RealWorldExample(t *testing.T) {
	c := qt.New(t)

	// Same example as used in SplitSQLStatements test
	input := ` -- POSTGRES TABLE: products --
CREATE TABLE products (
  id SERIAL PRIMARY KEY NOT NULL,
  active BOOLEAN NOT NULL DEFAULT 'true',
  name VARCHAR(255) NOT NULL,
  price DECIMAL(10,2) NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  category VARCHAR(100),
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  description TEXT
);


-- POSTGRES TABLE: users --
CREATE TABLE users (
  id SERIAL PRIMARY KEY NOT NULL,
  active BOOLEAN NOT NULL DEFAULT 'true',
  name VARCHAR(255) NOT NULL,
  age INTEGER,
  email VARCHAR(255) UNIQUE NOT NULL,
  bio TEXT,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);`

	expected := ` 
CREATE TABLE products (
  id SERIAL PRIMARY KEY NOT NULL,
  active BOOLEAN NOT NULL DEFAULT 'true',
  name VARCHAR(255) NOT NULL,
  price DECIMAL(10,2) NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  category VARCHAR(100),
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  description TEXT
);



CREATE TABLE users (
  id SERIAL PRIMARY KEY NOT NULL,
  active BOOLEAN NOT NULL DEFAULT 'true',
  name VARCHAR(255) NOT NULL,
  age INTEGER,
  email VARCHAR(255) UNIQUE NOT NULL,
  bio TEXT,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);`

	result := sqlsplitter.RemoveComments(input)
	c.Assert(result, qt.Equals, expected)
}

func TestRemoveComments_Integration(t *testing.T) {
	c := qt.New(t)

	// Test that RemoveComments + SplitSQLStatements works correctly together
	input := `-- First statement
SELECT * FROM users -- get users
WHERE active = true; -- only active

-- Second statement
INSERT INTO logs (message) VALUES ('Test -- not a comment'); /* Real comment */`

	// First remove comments
	withoutComments := sqlsplitter.RemoveComments(input)
	expected := `
SELECT * FROM users 
WHERE active = true; 


INSERT INTO logs (message) VALUES ('Test -- not a comment'); `

	c.Assert(withoutComments, qt.Equals, expected)

	// Then split into statements
	statements := sqlsplitter.SplitSQLStatements(withoutComments)
	expectedStatements := []string{
		"SELECT * FROM users \nWHERE active = true",
		"INSERT INTO logs (message) VALUES ('Test -- not a comment')",
	}

	c.Assert(statements, qt.DeepEquals, expectedStatements)
}
