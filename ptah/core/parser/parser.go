// Package parser provides token-to-AST parsing logic.
package parser

import (
	"fmt"
	"strings"
	"time"

	"github.com/denisvmedia/inventario/ptah/core/ast"
	"github.com/denisvmedia/inventario/ptah/core/lexer"
)

// Parser converts SQL tokens into AST nodes.
//
// The parser takes a stream of tokens from the lexer and builds an Abstract Syntax Tree
// representation of SQL DDL statements. It supports CREATE TABLE, ALTER TABLE, CREATE INDEX,
// and other DDL operations.
type Parser struct {
	lexer     *lexer.Lexer
	current   lexer.Token
	previous  lexer.Token
	startTime time.Time
	timeout   time.Duration
}

// NewParser creates a new parser with the given SQL input.
//
// The parser initializes with a lexer and advances to the first token.
//
// Example:
//
//	parser := NewParser("CREATE TABLE users (id INTEGER PRIMARY KEY);")
func NewParser(input string) *Parser {
	l := lexer.NewLexer(input)
	p := &Parser{
		lexer:     l,
		startTime: time.Now(),
		timeout:   30 * time.Second, // 30 second timeout to prevent infinite loops
	}
	p.advance() // Load the first token
	return p
}

// Parse parses the input SQL and returns a list of AST statements.
//
// This method parses multiple SQL statements separated by semicolons and returns
// them as a StatementList. Each statement is parsed according to its type
// (CREATE TABLE, ALTER TABLE, etc.).
//
// Returns an error if the SQL syntax is invalid or unsupported.
func (p *Parser) Parse() (*ast.StatementList, error) {
	statements := &ast.StatementList{
		Statements: make([]ast.Node, 0),
	}

	for !p.isAtEnd() {
		// Check for timeout to prevent infinite loops
		if err := p.checkTimeout(); err != nil {
			return nil, err
		}

		// Skip whitespace and comments
		if p.current.Type == lexer.TokenWhitespace || p.current.Type == lexer.TokenComment {
			p.advance()
			continue
		}

		// Skip empty statements (just semicolons)
		if p.current.Type == lexer.TokenSemicolon {
			p.advance()
			continue
		}

		stmt, err := p.parseStatement()
		if err != nil {
			return nil, err
		}

		if stmt != nil {
			statements.Statements = append(statements.Statements, stmt)
		}

		// Consume optional semicolon
		if p.current.Type == lexer.TokenSemicolon {
			p.advance()
		}
	}

	return statements, nil
}

// parseStatement parses a single SQL statement based on the current token.
func (p *Parser) parseStatement() (ast.Node, error) {
	if p.current.Type != lexer.TokenIdentifier {
		return nil, fmt.Errorf("expected SQL keyword, got %s at position %d", p.current.Type, p.current.Start)
	}

	keyword := strings.ToUpper(p.current.Value)
	switch keyword {
	case "CREATE":
		return p.parseCreateStatement()
	case "ALTER":
		return p.parseAlterStatement()
	case "COMMENT":
		return p.parseCommentStatement()
	default:
		return nil, fmt.Errorf("unsupported SQL statement: %s at position %d", keyword, p.current.Start)
	}
}

// parseCreateStatement parses CREATE statements (TABLE, INDEX, TYPE).
func (p *Parser) parseCreateStatement() (ast.Node, error) {
	if err := p.expect(lexer.TokenIdentifier, "CREATE"); err != nil {
		return nil, err
	}

	p.skipWhitespace()

	if p.current.Type != lexer.TokenIdentifier {
		return nil, fmt.Errorf("expected CREATE target (TABLE, INDEX, TYPE), got %s at position %d", p.current.Type, p.current.Start)
	}

	target := strings.ToUpper(p.current.Value)
	switch target {
	case "TABLE":
		return p.parseCreateTable()
	case "INDEX":
		return p.parseCreateIndex()
	case "UNIQUE":
		// Handle CREATE UNIQUE INDEX
		p.advance()
		p.skipWhitespace()
		if err := p.expect(lexer.TokenIdentifier, "INDEX"); err != nil {
			return nil, err
		}
		return p.parseCreateUniqueIndex()
	case "TYPE":
		return p.parseCreateType()
	case "DOMAIN":
		return p.parseCreateDomain()
	default:
		return nil, fmt.Errorf("unsupported CREATE target: %s at position %d", target, p.current.Start)
	}
}

// advance moves to the next token.
func (p *Parser) advance() {
	p.previous = p.current
	p.current = p.lexer.NextToken()
}

// isAtEnd checks if we've reached the end of the input.
func (p *Parser) isAtEnd() bool {
	return p.current.Type == lexer.TokenEOF
}

// checkTimeout checks if parsing has exceeded the timeout limit.
func (p *Parser) checkTimeout() error {
	if time.Since(p.startTime) > p.timeout {
		return fmt.Errorf("parsing timeout exceeded (%v) - possible infinite loop at position %d", p.timeout, p.current.Start)
	}
	return nil
}

// expect consumes a token of the expected type and value, returning an error if it doesn't match.
func (p *Parser) expect(tokenType lexer.TokenType, value string) error {
	p.skipWhitespace()
	if p.current.Type != tokenType {
		return fmt.Errorf("expected %s, got %s at position %d", tokenType, p.current.Type, p.current.Start)
	}
	if value != "" && strings.ToUpper(p.current.Value) != strings.ToUpper(value) {
		return fmt.Errorf("expected '%s', got '%s' at position %d", value, p.current.Value, p.current.Start)
	}
	p.advance()
	return nil
}

// expectIdentifier consumes an identifier token and returns its value.
func (p *Parser) expectIdentifier() (string, error) {
	if p.current.Type != lexer.TokenIdentifier {
		return "", fmt.Errorf("expected identifier, got %s at position %d", p.current.Type, p.current.Start)
	}
	value := p.current.Value
	p.advance()
	return value, nil
}

// skipWhitespace skips whitespace and comment tokens.
func (p *Parser) skipWhitespace() {
	for p.current.Type == lexer.TokenWhitespace || p.current.Type == lexer.TokenComment {
		p.advance()
	}
}

// isNumeric checks if a string represents a numeric value.
func isNumeric(s string) bool {
	if s == "" {
		return false
	}

	for i, r := range s {
		if i == 0 && (r == '-' || r == '+') {
			continue
		}
		if r < '0' || r > '9' {
			if r == '.' {
				continue // Allow decimal points
			}
			return false
		}
	}
	return true
}

// parseCreateTable parses CREATE TABLE statements.
func (p *Parser) parseCreateTable() (*ast.CreateTableNode, error) {
	if err := p.expect(lexer.TokenIdentifier, "TABLE"); err != nil {
		return nil, err
	}

	p.skipWhitespace()

	// Get table name (could be schema.table)
	var tableName strings.Builder
	tableName.WriteString(p.current.Value)
	p.advance()

	// Check for schema.table notation
	if p.current.Type == lexer.TokenOperator && p.current.Value == "." {
		tableName.WriteString(".")
		p.advance()
		p.skipWhitespace()
		if p.current.Type != lexer.TokenIdentifier {
			return nil, fmt.Errorf("expected table name after schema: got %s at position %d", p.current.Type, p.current.Start)
		}
		tableName.WriteString(p.current.Value)
		p.advance()
	}

	p.skipWhitespace()

	// Expect opening parenthesis
	if err := p.expect(lexer.TokenOperator, "("); err != nil {
		return nil, fmt.Errorf("expected '(' after table name: %w", err)
	}

	table := ast.NewCreateTable(tableName.String())

	// Parse column definitions and constraints
	for {
		// Check for timeout to prevent infinite loops
		if err := p.checkTimeout(); err != nil {
			return nil, err
		}

		p.skipWhitespace()

		// Check for closing parenthesis
		if p.current.Type == lexer.TokenOperator && p.current.Value == ")" {
			break
		}

		// Parse column or constraint
		if err := p.parseTableElement(table); err != nil {
			return nil, err
		}

		p.skipWhitespace()

		// Check for comma or closing parenthesis
		if p.current.Type == lexer.TokenOperator && p.current.Value == "," {
			p.advance()
			continue
		} else if p.current.Type == lexer.TokenOperator && p.current.Value == ")" {
			break
		} else if p.current.Type == lexer.TokenWhitespace {
			// Skip whitespace and try again
			p.skipWhitespace()
			if p.current.Type == lexer.TokenOperator && p.current.Value == "," {
				p.advance()
				continue
			} else if p.current.Type == lexer.TokenOperator && p.current.Value == ")" {
				break
			}
		}

		// If we get here and it's an identifier, it might be another table element
		if p.current.Type == lexer.TokenIdentifier {
			continue
		}

		return nil, fmt.Errorf("expected ',' or ')' after table element at position %d", p.current.Start)
	}

	// Consume closing parenthesis
	if err := p.expect(lexer.TokenOperator, ")"); err != nil {
		return nil, err
	}

	// Parse optional table options (ENGINE, etc.)
	if err := p.parseTableOptions(table); err != nil {
		return nil, err
	}

	return table, nil
}

// parseTableElement parses a column definition or table constraint.
func (p *Parser) parseTableElement(table *ast.CreateTableNode) error {
	p.skipWhitespace()

	// Check if this is a constraint (starts with CONSTRAINT, PRIMARY, UNIQUE, FOREIGN, CHECK, SPATIAL, INDEX, KEY)
	if p.current.Type == lexer.TokenIdentifier {
		keyword := strings.ToUpper(p.current.Value)
		switch keyword {
		case "CONSTRAINT", "PRIMARY", "UNIQUE", "FOREIGN", "CHECK", "SPATIAL", "INDEX", "KEY":
			constraint, err := p.parseTableConstraint()
			if err != nil {
				return err
			}
			table.AddConstraint(constraint)
			return nil
		}
	}

	// Otherwise, parse as column definition
	column, err := p.parseColumnDefinition()
	if err != nil {
		return err
	}
	table.AddColumn(column)
	return nil
}

// parseColumnDefinition parses a column definition.
func (p *Parser) parseColumnDefinition() (*ast.ColumnNode, error) {
	p.skipWhitespace()

	// Get column name
	columnName, err := p.expectIdentifier()
	if err != nil {
		return nil, fmt.Errorf("expected column name: %w", err)
	}

	p.skipWhitespace()

	// Get column type
	columnType, err := p.parseColumnType()
	if err != nil {
		return nil, fmt.Errorf("expected column type: %w", err)
	}

	column := ast.NewColumn(columnName, columnType)

	// Parse column constraints and attributes
	for {
		// Check for timeout to prevent infinite loops
		if err := p.checkTimeout(); err != nil {
			return nil, err
		}

		p.skipWhitespace()

		if p.current.Type != lexer.TokenIdentifier {
			break
		}

		keyword := strings.ToUpper(p.current.Value)
		switch keyword {
		case "NOT":
			// Handle NOT NULL
			p.advance()
			p.skipWhitespace()
			if err := p.expect(lexer.TokenIdentifier, "NULL"); err != nil {
				return nil, fmt.Errorf("expected NULL after NOT: %w", err)
			}
			column.SetNotNull()

		case "NULL":
			// Explicit NULL (default behavior)
			p.advance()
			column.Nullable = true

		case "PRIMARY":
			// Handle PRIMARY KEY
			p.advance()
			p.skipWhitespace()
			if err := p.expect(lexer.TokenIdentifier, "KEY"); err != nil {
				return nil, fmt.Errorf("expected KEY after PRIMARY: %w", err)
			}
			column.SetPrimary()

		case "UNIQUE":
			p.advance()
			column.SetUnique()

		case "AUTO_INCREMENT", "AUTOINCREMENT":
			p.advance()
			column.SetAutoIncrement()

		case "DEFAULT":
			p.advance()
			p.skipWhitespace()
			defaultValue, err := p.parseDefaultValue()
			if err != nil {
				return nil, fmt.Errorf("expected default value: %w", err)
			}
			if defaultValue.Expression != "" {
				column.SetDefaultExpression(defaultValue.Expression)
			} else {
				column.SetDefault(defaultValue.Value)
			}

		case "CHECK":
			p.advance()
			p.skipWhitespace()
			checkExpr, err := p.parseCheckExpression()
			if err != nil {
				return nil, fmt.Errorf("expected check expression: %w", err)
			}
			column.SetCheck(checkExpr)

		case "REFERENCES":
			// Handle foreign key reference
			p.advance()
			fkRef, err := p.parseForeignKeyReference()
			if err != nil {
				return nil, fmt.Errorf("expected foreign key reference: %w", err)
			}
			column.ForeignKey = fkRef

		case "AS":
			// Handle MySQL/MariaDB virtual columns (AS (expression) STORED)
			p.advance()
			p.skipWhitespace()

			// Parse the generation expression
			if err := p.expect(lexer.TokenOperator, "("); err != nil {
				return nil, fmt.Errorf("expected '(' for generated expression: %w", err)
			}

			// Collect the expression until closing parenthesis
			var expr strings.Builder
			parenCount := 1
			for parenCount > 0 && !p.isAtEnd() {
				if p.current.Type == lexer.TokenOperator {
					if p.current.Value == "(" {
						parenCount++
					} else if p.current.Value == ")" {
						parenCount--
					}
				}
				if parenCount > 0 {
					expr.WriteString(p.current.Value)
				}
				p.advance()
			}

			// Parse STORED/VIRTUAL keyword
			p.skipWhitespace()
			if p.current.Type == lexer.TokenIdentifier {
				storageType := strings.ToUpper(p.current.Value)
				if storageType == "STORED" || storageType == "VIRTUAL" {
					p.advance()
				}
			}

			// Store as a check constraint for now (in a full implementation, add Generated field to ColumnNode)
			column.SetCheck("AS (" + expr.String() + ") STORED")

		case "GENERATED":
			// Handle PostgreSQL GENERATED columns
			p.advance()
			p.skipWhitespace()
			if err := p.expect(lexer.TokenIdentifier, "ALWAYS"); err != nil {
				return nil, fmt.Errorf("expected ALWAYS after GENERATED: %w", err)
			}
			p.skipWhitespace()
			if err := p.expect(lexer.TokenIdentifier, "AS"); err != nil {
				return nil, fmt.Errorf("expected AS after ALWAYS: %w", err)
			}
			p.skipWhitespace()

			// Parse the generation expression
			if err := p.expect(lexer.TokenOperator, "("); err != nil {
				return nil, fmt.Errorf("expected '(' for generated expression: %w", err)
			}

			// Collect the expression until closing parenthesis
			var expr strings.Builder
			parenCount := 1
			for parenCount > 0 && !p.isAtEnd() {
				if p.current.Type == lexer.TokenOperator {
					if p.current.Value == "(" {
						parenCount++
					} else if p.current.Value == ")" {
						parenCount--
					}
				}
				if parenCount > 0 {
					expr.WriteString(p.current.Value)
				}
				p.advance()
			}

			// Parse STORED keyword
			p.skipWhitespace()
			if p.current.Type == lexer.TokenIdentifier && strings.ToUpper(p.current.Value) == "STORED" {
				p.advance()
			}

			// Store as a check constraint for now (in a full implementation, add Generated field to ColumnNode)
			column.SetCheck("GENERATED ALWAYS AS (" + expr.String() + ") STORED")

		case "CHARACTER":
			// Handle MySQL/MariaDB CHARACTER SET
			p.advance()
			p.skipWhitespace()
			if p.current.Type == lexer.TokenIdentifier && strings.ToUpper(p.current.Value) == "SET" {
				p.advance()
				p.skipWhitespace()
				if p.current.Type == lexer.TokenIdentifier {
					// Store charset as comment for now
					column.SetComment("CHARACTER SET " + p.current.Value)
					p.advance()
				}
			}

		case "COLLATE":
			// Handle PostgreSQL/MySQL COLLATE
			p.advance()
			p.skipWhitespace()

			var collation string
			if p.current.Type == lexer.TokenString {
				// Quoted collation name like "C"
				collation = p.current.Value
				p.advance()
			} else if p.current.Type == lexer.TokenIdentifier {
				// Unquoted collation name
				collation = p.current.Value
				p.advance()
			} else {
				return nil, fmt.Errorf("expected collation name: got %s at position %d", p.current.Type, p.current.Start)
			}

			// Store as comment for now (in a full implementation, add Collation field to ColumnNode)
			column.SetComment("COLLATE " + collation)

		case "ON":
			// Handle MySQL/MariaDB ON UPDATE syntax
			p.advance()
			p.skipWhitespace()
			if p.current.Type == lexer.TokenIdentifier && strings.ToUpper(p.current.Value) == "UPDATE" {
				p.advance()
				p.skipWhitespace()
				// Parse the update expression (usually CURRENT_TIMESTAMP)
				if p.current.Type == lexer.TokenIdentifier {
					updateExpr := p.current.Value
					p.advance()
					// Handle function calls like CURRENT_TIMESTAMP()
					if p.current.Type == lexer.TokenOperator && p.current.Value == "(" {
						p.advance()
						if p.current.Type == lexer.TokenOperator && p.current.Value == ")" {
							updateExpr += "()"
							p.advance()
						}
					}
					// Store as comment for now
					column.SetComment("ON UPDATE " + updateExpr)
				}
			}

		default:
			// Unknown keyword, stop parsing column attributes
			break
		}
	}

	return column, nil
}

// parseColumnType parses a column data type (e.g., INTEGER, VARCHAR(255), DECIMAL(10,2), DOUBLE PRECISION).
func (p *Parser) parseColumnType() (string, error) {
	if p.current.Type != lexer.TokenIdentifier {
		return "", fmt.Errorf("expected column type, got %s at position %d", p.current.Type, p.current.Start)
	}

	typeName := p.current.Value
	p.advance()

	// Handle multi-word types like DOUBLE PRECISION, CHARACTER VARYING, etc.
	p.skipWhitespace()
	if p.current.Type == lexer.TokenIdentifier {
		firstWord := strings.ToUpper(typeName)
		secondWord := strings.ToUpper(p.current.Value)

		// Check for known multi-word type combinations
		switch firstWord {
		case "DOUBLE":
			if secondWord == "PRECISION" {
				typeName += " " + p.current.Value
				p.advance()
			}
		case "CHARACTER":
			if secondWord == "VARYING" {
				typeName += " " + p.current.Value
				p.advance()
			}
		case "TIME":
			if secondWord == "WITH" || secondWord == "WITHOUT" {
				typeName = p.current.Value + " " + typeName
				p.advance()
				p.skipWhitespace()
				if p.current.Type == lexer.TokenIdentifier && strings.ToUpper(p.current.Value) == "TIME" {
					typeName += " " + p.current.Value
					p.advance()
					p.skipWhitespace()
					if p.current.Type == lexer.TokenIdentifier && strings.ToUpper(p.current.Value) == "ZONE" {
						typeName += " " + p.current.Value
						p.advance()
					}
				}
			}
		case "TIMESTAMP":
			if secondWord == "WITH" || secondWord == "WITHOUT" {
				typeName = p.current.Value + " " + typeName
				p.advance()
				p.skipWhitespace()
				if p.current.Type == lexer.TokenIdentifier && strings.ToUpper(p.current.Value) == "TIME" {
					typeName += " " + p.current.Value
					p.advance()
					p.skipWhitespace()
					if p.current.Type == lexer.TokenIdentifier && strings.ToUpper(p.current.Value) == "ZONE" {
						typeName += " " + p.current.Value
						p.advance()
					}
				}
			}
		}
	}

	// Check for type parameters (e.g., VARCHAR(255), NUMERIC(10,2))
	p.skipWhitespace()
	if p.current.Type == lexer.TokenOperator && p.current.Value == "(" {
		typeName += "("
		p.advance()

		// Collect everything inside parentheses
		parenCount := 1
		for parenCount > 0 && p.current.Type != lexer.TokenEOF {
			if p.current.Type == lexer.TokenOperator && p.current.Value == "(" {
				parenCount++
			} else if p.current.Type == lexer.TokenOperator && p.current.Value == ")" {
				parenCount--
			}
			typeName += p.current.Value
			p.advance()
		}
	}

	// Check for MySQL/MariaDB type modifiers (UNSIGNED, ZEROFILL, etc.)
	p.skipWhitespace()
	for p.current.Type == lexer.TokenIdentifier {
		modifier := strings.ToUpper(p.current.Value)
		switch modifier {
		case "UNSIGNED", "SIGNED", "ZEROFILL":
			typeName += " " + p.current.Value
			p.advance()
			p.skipWhitespace()
		default:
			// Not a type modifier, stop processing
			goto arrayCheck
		}
	}

arrayCheck:
	// Check for array notation (PostgreSQL) - must come after type parameters
	p.skipWhitespace()
	if p.current.Type == lexer.TokenOperator && p.current.Value == "[" {
		typeName += "["
		p.advance()

		// Handle multi-dimensional arrays like INT[][] or NUMERIC(5,2)[]
		for p.current.Type == lexer.TokenOperator && p.current.Value == "]" {
			typeName += "]"
			p.advance()
			if p.current.Type == lexer.TokenOperator && p.current.Value == "[" {
				typeName += "["
				p.advance()
			} else {
				break
			}
		}
	}

	return typeName, nil
}

// parseDefaultValue parses a default value (literal or function call).
func (p *Parser) parseDefaultValue() (*ast.DefaultValue, error) {
	p.skipWhitespace()

	switch p.current.Type {
	case lexer.TokenString:
		// String literal
		value := p.current.Value
		p.advance()

		// Check for PostgreSQL type casting like '{}'::jsonb
		if p.current.Type == lexer.TokenOperator && p.current.Value == ":" {
			p.advance()
			if p.current.Type == lexer.TokenOperator && p.current.Value == ":" {
				p.advance()
				if p.current.Type == lexer.TokenIdentifier {
					value += "::" + p.current.Value
					p.advance()
				}
			}
		}

		return &ast.DefaultValue{Value: value}, nil

	case lexer.TokenIdentifier:
		// Could be a function call or keyword like NULL, TRUE, FALSE
		value := p.current.Value
		p.advance()

		// Check if it's a function call
		if p.current.Type == lexer.TokenOperator && p.current.Value == "(" {
			// Parse function call
			p.advance()
			p.skipWhitespace()

			// Consume closing parenthesis
			if err := p.expect(lexer.TokenOperator, ")"); err != nil {
				return nil, err
			}

			return &ast.DefaultValue{Expression: value + "()"}, nil
		}

		// Handle MySQL/PostgreSQL functions that can be used without parentheses
		upperValue := strings.ToUpper(value)
		if upperValue == "CURRENT_TIMESTAMP" || upperValue == "NOW" || upperValue == "CURRENT_DATE" || upperValue == "CURRENT_TIME" {
			return &ast.DefaultValue{Expression: value + "()"}, nil
		}

		// Handle PostgreSQL-specific functions
		if upperValue == "GEN_RANDOM_UUID" {
			return &ast.DefaultValue{Expression: value + "()"}, nil
		}

		// Handle PostgreSQL array literals like ARRAY[]::TEXT[]
		if upperValue == "ARRAY" {
			p.skipWhitespace()
			if p.current.Type == lexer.TokenOperator && p.current.Value == "[" {
				// Parse array literal
				arrayLiteral := value + "["
				p.advance()

				// Collect array elements
				for {
					if p.current.Type == lexer.TokenOperator && p.current.Value == "]" {
						arrayLiteral += "]"
						p.advance()
						break
					}
					arrayLiteral += p.current.Value
					p.advance()
				}

				// Handle type cast like ::TEXT[]
				if p.current.Type == lexer.TokenOperator && p.current.Value == ":" {
					p.advance()
					if p.current.Type == lexer.TokenOperator && p.current.Value == ":" {
						p.advance()
						// Get the cast type
						if p.current.Type == lexer.TokenIdentifier {
							arrayLiteral += "::" + p.current.Value
							p.advance()
							// Handle array brackets in cast
							if p.current.Type == lexer.TokenOperator && p.current.Value == "[" {
								arrayLiteral += "["
								p.advance()
								if p.current.Type == lexer.TokenOperator && p.current.Value == "]" {
									arrayLiteral += "]"
									p.advance()
								}
							}
						}
					}
				}

				return &ast.DefaultValue{Expression: arrayLiteral}, nil
			}
		}

		// Regular identifier/keyword
		return &ast.DefaultValue{Value: value}, nil

	case lexer.TokenOperator:
		// Could be a number (positive or negative) or just a number
		if p.current.Value == "-" || p.current.Value == "+" {
			sign := p.current.Value
			p.advance()
			p.skipWhitespace()
			if p.current.Type == lexer.TokenIdentifier || p.current.Type == lexer.TokenOperator {
				value := sign + p.current.Value
				p.advance()
				return &ast.DefaultValue{Value: value}, nil
			}
		}
		// Check if it's a number that the lexer tokenized as an operator (like "0", "1", etc.)
		// Numbers might be tokenized as operators by the simple lexer
		value := p.current.Value
		// Check if this looks like a number
		if isNumeric(value) {
			p.advance()
			return &ast.DefaultValue{Value: value}, nil
		}

		return nil, fmt.Errorf("unexpected token for default value: %s at position %d", p.current.Value, p.current.Start)

	default:
		return nil, fmt.Errorf("expected default value, got %s at position %d", p.current.Type, p.current.Start)
	}
}

// parseCheckExpression parses a CHECK constraint expression.
func (p *Parser) parseCheckExpression() (string, error) {
	p.skipWhitespace()

	// Expect opening parenthesis
	if err := p.expect(lexer.TokenOperator, "("); err != nil {
		return "", fmt.Errorf("expected '(' for check expression: %w", err)
	}

	// Collect everything until closing parenthesis
	var expr strings.Builder
	parenCount := 1

	for parenCount > 0 && !p.isAtEnd() {
		if p.current.Type == lexer.TokenOperator {
			if p.current.Value == "(" {
				parenCount++
			} else if p.current.Value == ")" {
				parenCount--
			}
		}

		if parenCount > 0 {
			expr.WriteString(p.current.Value)
		}
		p.advance()
	}

	return expr.String(), nil
}

// parseForeignKeyReference parses a REFERENCES clause.
func (p *Parser) parseForeignKeyReference() (*ast.ForeignKeyRef, error) {
	p.skipWhitespace()

	// Get referenced table name
	tableName, err := p.expectIdentifier()
	if err != nil {
		return nil, fmt.Errorf("expected table name in REFERENCES: %w", err)
	}

	p.skipWhitespace()

	// Expect opening parenthesis
	if err := p.expect(lexer.TokenOperator, "("); err != nil {
		return nil, fmt.Errorf("expected '(' after table name in REFERENCES: %w", err)
	}

	p.skipWhitespace()

	// Get referenced column name
	columnName, err := p.expectIdentifier()
	if err != nil {
		return nil, fmt.Errorf("expected column name in REFERENCES: %w", err)
	}

	p.skipWhitespace()

	// Expect closing parenthesis
	if err := p.expect(lexer.TokenOperator, ")"); err != nil {
		return nil, fmt.Errorf("expected ')' after column name in REFERENCES: %w", err)
	}

	fkRef := &ast.ForeignKeyRef{
		Table:  tableName,
		Column: columnName,
	}

	// Parse optional ON DELETE/UPDATE actions
	for {
		p.skipWhitespace()

		if p.current.Type != lexer.TokenIdentifier || strings.ToUpper(p.current.Value) != "ON" {
			break
		}

		p.advance() // consume ON
		p.skipWhitespace()

		if p.current.Type != lexer.TokenIdentifier {
			break
		}

		action := strings.ToUpper(p.current.Value)
		p.advance()
		p.skipWhitespace()

		// Get the action value (CASCADE, SET NULL, etc.)
		var actionValue string
		if p.current.Type == lexer.TokenIdentifier {
			actionValue = strings.ToUpper(p.current.Value)
			p.advance()

			// Handle multi-word actions like "SET NULL"
			if actionValue == "SET" {
				p.skipWhitespace()
				if p.current.Type == lexer.TokenIdentifier {
					actionValue += " " + strings.ToUpper(p.current.Value)
					p.advance()
				}
			}
		}

		switch action {
		case "DELETE":
			fkRef.OnDelete = actionValue
		case "UPDATE":
			fkRef.OnUpdate = actionValue
		}
	}

	return fkRef, nil
}

// parseTableConstraint parses table-level constraints.
func (p *Parser) parseTableConstraint() (*ast.ConstraintNode, error) {
	p.skipWhitespace()

	constraint := &ast.ConstraintNode{}

	// Check for CONSTRAINT name
	if p.current.Type == lexer.TokenIdentifier && strings.ToUpper(p.current.Value) == "CONSTRAINT" {
		p.advance()
		p.skipWhitespace()

		// Get constraint name
		name, err := p.expectIdentifier()
		if err != nil {
			return nil, fmt.Errorf("expected constraint name: %w", err)
		}
		constraint.Name = name
		p.skipWhitespace()
	}

	// Parse constraint type
	if p.current.Type != lexer.TokenIdentifier {
		return nil, fmt.Errorf("expected constraint type, got %s at position %d", p.current.Type, p.current.Start)
	}

	constraintType := strings.ToUpper(p.current.Value)
	switch constraintType {
	case "PRIMARY":
		p.advance()
		p.skipWhitespace()
		if err := p.expect(lexer.TokenIdentifier, "KEY"); err != nil {
			return nil, fmt.Errorf("expected KEY after PRIMARY: %w", err)
		}
		constraint.Type = ast.PrimaryKeyConstraint

	case "UNIQUE":
		p.advance()
		p.skipWhitespace()
		// Optional KEY or INDEX keyword
		if p.current.Type == lexer.TokenIdentifier {
			keyword := strings.ToUpper(p.current.Value)
			if keyword == "KEY" || keyword == "INDEX" {
				p.advance()
				p.skipWhitespace()
				// Check for optional constraint name after UNIQUE KEY
				if p.current.Type == lexer.TokenIdentifier && p.current.Value != "(" {
					constraint.Name = p.current.Value
					p.advance()
					p.skipWhitespace()
				}
			}
		}
		constraint.Type = ast.UniqueConstraint

	case "FOREIGN":
		p.advance()
		p.skipWhitespace()
		if err := p.expect(lexer.TokenIdentifier, "KEY"); err != nil {
			return nil, fmt.Errorf("expected KEY after FOREIGN: %w", err)
		}
		constraint.Type = ast.ForeignKeyConstraint

	case "CHECK":
		p.advance()
		constraint.Type = ast.CheckConstraint

	case "SPATIAL":
		p.advance()
		p.skipWhitespace()
		// Expect INDEX keyword
		if err := p.expect(lexer.TokenIdentifier, "INDEX"); err != nil {
			return nil, fmt.Errorf("expected INDEX after SPATIAL: %w", err)
		}
		// Treat as a special unique constraint for now
		constraint.Type = ast.UniqueConstraint
		constraint.Name = "SPATIAL_INDEX"

	case "INDEX", "KEY":
		p.advance()
		p.skipWhitespace()
		// Check for optional constraint name after INDEX/KEY
		if p.current.Type == lexer.TokenIdentifier && p.current.Value != "(" {
			constraint.Name = p.current.Value
			p.advance()
			p.skipWhitespace()
		}
		// Treat as a unique constraint for now
		constraint.Type = ast.UniqueConstraint

	default:
		return nil, fmt.Errorf("unsupported constraint type: %s at position %d", constraintType, p.current.Start)
	}

	p.skipWhitespace()

	// Parse column list for PRIMARY KEY, UNIQUE, FOREIGN KEY
	if constraint.Type != ast.CheckConstraint {
		if err := p.expect(lexer.TokenOperator, "("); err != nil {
			return nil, fmt.Errorf("expected '(' for constraint columns: %w", err)
		}

		p.skipWhitespace()

		// Parse column names
		for {
			columnName, err := p.expectIdentifier()
			if err != nil {
				return nil, fmt.Errorf("expected column name: %w", err)
			}
			constraint.Columns = append(constraint.Columns, columnName)

			p.skipWhitespace()

			if p.current.Type == lexer.TokenOperator && p.current.Value == "," {
				p.advance()
				p.skipWhitespace()
				continue
			} else if p.current.Type == lexer.TokenOperator && p.current.Value == ")" {
				break
			} else {
				return nil, fmt.Errorf("expected ',' or ')' in column list at position %d", p.current.Start)
			}
		}

		if err := p.expect(lexer.TokenOperator, ")"); err != nil {
			return nil, err
		}
	}

	// Handle FOREIGN KEY REFERENCES
	if constraint.Type == ast.ForeignKeyConstraint {
		p.skipWhitespace()
		if err := p.expect(lexer.TokenIdentifier, "REFERENCES"); err != nil {
			return nil, fmt.Errorf("expected REFERENCES after FOREIGN KEY: %w", err)
		}

		fkRef, err := p.parseForeignKeyReference()
		if err != nil {
			return nil, err
		}
		constraint.Reference = fkRef
	}

	// Handle CHECK expression
	if constraint.Type == ast.CheckConstraint {
		expr, err := p.parseCheckExpression()
		if err != nil {
			return nil, err
		}
		constraint.Expression = expr
	}

	return constraint, nil
}

// parseTableOptions parses table options like ENGINE, CHARSET, etc.
func (p *Parser) parseTableOptions(table *ast.CreateTableNode) error {
	for {
		// Check for timeout to prevent infinite loops
		if err := p.checkTimeout(); err != nil {
			return err
		}

		p.skipWhitespace()

		if p.current.Type != lexer.TokenIdentifier {
			break
		}

		option := strings.ToUpper(p.current.Value)
		switch option {
		case "ENGINE":
			p.advance()
			p.skipWhitespace()
			if err := p.expect(lexer.TokenOperator, "="); err != nil {
				return fmt.Errorf("expected '=' after ENGINE: %w", err)
			}
			p.skipWhitespace()
			value, err := p.expectIdentifier()
			if err != nil {
				return fmt.Errorf("expected engine value: %w", err)
			}
			table.SetOption("ENGINE", value)

		case "CHARSET", "CHARACTER":
			p.advance()
			p.skipWhitespace()
			if option == "CHARACTER" {
				if err := p.expect(lexer.TokenIdentifier, "SET"); err != nil {
					return fmt.Errorf("expected SET after CHARACTER: %w", err)
				}
				p.skipWhitespace()
			}
			if err := p.expect(lexer.TokenOperator, "="); err != nil {
				return fmt.Errorf("expected '=' after CHARSET: %w", err)
			}
			p.skipWhitespace()
			value, err := p.expectIdentifier()
			if err != nil {
				return fmt.Errorf("expected charset value: %w", err)
			}
			table.SetOption("CHARSET", value)

		case "COLLATE":
			p.advance()
			p.skipWhitespace()
			if err := p.expect(lexer.TokenOperator, "="); err != nil {
				return fmt.Errorf("expected '=' after COLLATE: %w", err)
			}
			p.skipWhitespace()
			value, err := p.expectIdentifier()
			if err != nil {
				return fmt.Errorf("expected collate value: %w", err)
			}
			table.SetOption("COLLATE", value)

		case "COMMENT":
			p.advance()
			p.skipWhitespace()
			if err := p.expect(lexer.TokenOperator, "="); err != nil {
				return fmt.Errorf("expected '=' after COMMENT: %w", err)
			}
			p.skipWhitespace()
			if p.current.Type != lexer.TokenString {
				return fmt.Errorf("expected string for comment value at position %d", p.current.Start)
			}
			table.Comment = p.current.Value
			p.advance()

		case "AUTO_INCREMENT":
			p.advance()
			p.skipWhitespace()
			if err := p.expect(lexer.TokenOperator, "="); err != nil {
				return fmt.Errorf("expected '=' after AUTO_INCREMENT: %w", err)
			}
			p.skipWhitespace()
			// Handle numeric values which might be tokenized as operators
			var value string
			if p.current.Type == lexer.TokenIdentifier {
				value = p.current.Value
				p.advance()
			} else if p.current.Type == lexer.TokenOperator && isNumeric(p.current.Value) {
				value = p.current.Value
				p.advance()
			} else {
				return fmt.Errorf("expected auto increment value: got %s at position %d", p.current.Type, p.current.Start)
			}
			table.SetOption("AUTO_INCREMENT", value)

		case "DEFAULT":
			// Handle DEFAULT CHARSET syntax
			p.advance()
			p.skipWhitespace()
			if p.current.Type == lexer.TokenIdentifier && strings.ToUpper(p.current.Value) == "CHARSET" {
				p.advance()
				p.skipWhitespace()
				if err := p.expect(lexer.TokenOperator, "="); err != nil {
					return fmt.Errorf("expected '=' after DEFAULT CHARSET: %w", err)
				}
				p.skipWhitespace()
				value, err := p.expectIdentifier()
				if err != nil {
					return fmt.Errorf("expected charset value: %w", err)
				}
				table.SetOption("CHARSET", value)
			} else {
				// Unknown DEFAULT option, stop parsing
				break
			}

		case "WITH":
			// Handle PostgreSQL WITH clause
			if err := p.parsePostgreSQLWithClause(table); err != nil {
				return err
			}

		case "ROW_FORMAT":
			p.advance()
			p.skipWhitespace()
			if err := p.expect(lexer.TokenOperator, "="); err != nil {
				return fmt.Errorf("expected '=' after ROW_FORMAT: %w", err)
			}
			p.skipWhitespace()
			value, err := p.expectIdentifier()
			if err != nil {
				return fmt.Errorf("expected row format value: %w", err)
			}
			table.SetOption("ROW_FORMAT", value)

		case "TABLESPACE":
			// Handle PostgreSQL TABLESPACE
			p.advance()
			p.skipWhitespace()
			value, err := p.expectIdentifier()
			if err != nil {
				return fmt.Errorf("expected tablespace name: %w", err)
			}
			table.SetOption("TABLESPACE", value)

		default:
			// Unknown option, stop parsing
			break
		}
	}

	return nil
}

// parsePostgreSQLWithClause parses PostgreSQL WITH clause for table options.
func (p *Parser) parsePostgreSQLWithClause(table *ast.CreateTableNode) error {
	if err := p.expect(lexer.TokenIdentifier, "WITH"); err != nil {
		return err
	}

	p.skipWhitespace()

	// Expect opening parenthesis
	if err := p.expect(lexer.TokenOperator, "("); err != nil {
		return fmt.Errorf("expected '(' after WITH: %w", err)
	}

	// Parse key-value pairs
	for {
		p.skipWhitespace()

		// Check for closing parenthesis
		if p.current.Type == lexer.TokenOperator && p.current.Value == ")" {
			break
		}

		// Get option name
		if p.current.Type != lexer.TokenIdentifier {
			return fmt.Errorf("expected option name in WITH clause, got %s at position %d", p.current.Type, p.current.Start)
		}
		optionName := p.current.Value
		p.advance()

		p.skipWhitespace()

		// Expect equals sign
		if err := p.expect(lexer.TokenOperator, "="); err != nil {
			return fmt.Errorf("expected '=' after option name '%s': %w", optionName, err)
		}

		p.skipWhitespace()

		// Get option value (can be identifier, number, or boolean)
		var optionValue string
		switch p.current.Type {
		case lexer.TokenIdentifier:
			optionValue = p.current.Value
			p.advance()
		case lexer.TokenString:
			optionValue = p.current.Value
			p.advance()
		default:
			// Handle numeric values and other tokens
			optionValue = p.current.Value
			p.advance()
		}

		// Store the option
		table.SetOption(optionName, optionValue)

		p.skipWhitespace()

		// Check for comma (more options) or closing parenthesis
		if p.current.Type == lexer.TokenOperator && p.current.Value == "," {
			p.advance()
			continue
		} else if p.current.Type == lexer.TokenOperator && p.current.Value == ")" {
			break
		} else {
			return fmt.Errorf("expected ',' or ')' in WITH clause at position %d", p.current.Start)
		}
	}

	// Consume closing parenthesis
	if err := p.expect(lexer.TokenOperator, ")"); err != nil {
		return err
	}

	return nil
}

// parseAlterStatement parses ALTER TABLE statements.
func (p *Parser) parseAlterStatement() (*ast.AlterTableNode, error) {
	if err := p.expect(lexer.TokenIdentifier, "ALTER"); err != nil {
		return nil, err
	}

	p.skipWhitespace()

	if err := p.expect(lexer.TokenIdentifier, "TABLE"); err != nil {
		return nil, fmt.Errorf("expected TABLE after ALTER: %w", err)
	}

	p.skipWhitespace()

	// Get table name
	tableName, err := p.expectIdentifier()
	if err != nil {
		return nil, fmt.Errorf("expected table name: %w", err)
	}

	alterNode := &ast.AlterTableNode{
		Name:       tableName,
		Operations: make([]ast.AlterOperation, 0),
	}

	// Parse alter operations
	for {
		p.skipWhitespace()

		if p.isAtEnd() || p.current.Type == lexer.TokenSemicolon {
			break
		}

		operation, err := p.parseAlterOperation()
		if err != nil {
			return nil, err
		}

		if operation != nil {
			alterNode.Operations = append(alterNode.Operations, operation)
		}

		p.skipWhitespace()

		// Check for comma (multiple operations)
		if p.current.Type == lexer.TokenOperator && p.current.Value == "," {
			p.advance()
			continue
		} else {
			break
		}
	}

	return alterNode, nil
}

// parseAlterOperation parses individual ALTER TABLE operations.
func (p *Parser) parseAlterOperation() (ast.AlterOperation, error) {
	p.skipWhitespace()

	if p.current.Type != lexer.TokenIdentifier {
		return nil, fmt.Errorf("expected ALTER operation, got %s at position %d", p.current.Type, p.current.Start)
	}

	operation := strings.ToUpper(p.current.Value)
	switch operation {
	case "ADD":
		return p.parseAddOperation()
	case "DROP":
		return p.parseDropOperation()
	case "MODIFY", "ALTER":
		return p.parseModifyOperation()
	default:
		return nil, fmt.Errorf("unsupported ALTER operation: %s at position %d", operation, p.current.Start)
	}
}

// parseAddOperation parses ADD COLUMN operations.
func (p *Parser) parseAddOperation() (*ast.AddColumnOperation, error) {
	if err := p.expect(lexer.TokenIdentifier, "ADD"); err != nil {
		return nil, err
	}

	p.skipWhitespace()

	// Optional COLUMN keyword
	if p.current.Type == lexer.TokenIdentifier && strings.ToUpper(p.current.Value) == "COLUMN" {
		p.advance()
		p.skipWhitespace()
	}

	// Parse column definition
	column, err := p.parseColumnDefinition()
	if err != nil {
		return nil, err
	}

	return &ast.AddColumnOperation{Column: column}, nil
}

// parseDropOperation parses DROP COLUMN operations.
func (p *Parser) parseDropOperation() (*ast.DropColumnOperation, error) {
	if err := p.expect(lexer.TokenIdentifier, "DROP"); err != nil {
		return nil, err
	}

	p.skipWhitespace()

	// Optional COLUMN keyword
	if p.current.Type == lexer.TokenIdentifier && strings.ToUpper(p.current.Value) == "COLUMN" {
		p.advance()
		p.skipWhitespace()
	}

	// Get column name
	columnName, err := p.expectIdentifier()
	if err != nil {
		return nil, fmt.Errorf("expected column name: %w", err)
	}

	return &ast.DropColumnOperation{ColumnName: columnName}, nil
}

// parseModifyOperation parses MODIFY/ALTER COLUMN operations.
func (p *Parser) parseModifyOperation() (*ast.ModifyColumnOperation, error) {
	operation := strings.ToUpper(p.current.Value)
	p.advance()

	p.skipWhitespace()

	// For ALTER COLUMN, expect COLUMN keyword
	if operation == "ALTER" {
		if err := p.expect(lexer.TokenIdentifier, "COLUMN"); err != nil {
			return nil, fmt.Errorf("expected COLUMN after ALTER: %w", err)
		}
		p.skipWhitespace()
	} else if operation == "MODIFY" {
		// Optional COLUMN keyword for MODIFY
		if p.current.Type == lexer.TokenIdentifier && strings.ToUpper(p.current.Value) == "COLUMN" {
			p.advance()
			p.skipWhitespace()
		}
	}

	// Parse column definition
	column, err := p.parseColumnDefinition()
	if err != nil {
		return nil, err
	}

	return &ast.ModifyColumnOperation{Column: column}, nil
}

// parseCreateIndex parses CREATE INDEX statements.
func (p *Parser) parseCreateIndex() (*ast.IndexNode, error) {
	if err := p.expect(lexer.TokenIdentifier, "INDEX"); err != nil {
		return nil, err
	}

	p.skipWhitespace()

	// Get index name
	if p.current.Type != lexer.TokenIdentifier {
		return nil, fmt.Errorf("expected index name, got %s at position %d", p.current.Type, p.current.Start)
	}
	indexName := p.current.Value
	p.advance()

	p.skipWhitespace()

	if err := p.expect(lexer.TokenIdentifier, "ON"); err != nil {
		return nil, fmt.Errorf("expected ON after index name: %w", err)
	}

	p.skipWhitespace()

	// Get table name
	tableName, err := p.expectIdentifier()
	if err != nil {
		return nil, fmt.Errorf("expected table name: %w", err)
	}

	p.skipWhitespace()

	// Parse column list
	if err := p.expect(lexer.TokenOperator, "("); err != nil {
		return nil, fmt.Errorf("expected '(' for index columns: %w", err)
	}

	var columns []string
	for {
		p.skipWhitespace()

		columnName, err := p.expectIdentifier()
		if err != nil {
			return nil, fmt.Errorf("expected column name: %w", err)
		}
		columns = append(columns, columnName)

		p.skipWhitespace()

		if p.current.Type == lexer.TokenOperator && p.current.Value == "," {
			p.advance()
			continue
		} else if p.current.Type == lexer.TokenOperator && p.current.Value == ")" {
			break
		} else {
			return nil, fmt.Errorf("expected ',' or ')' in column list at position %d", p.current.Start)
		}
	}

	if err := p.expect(lexer.TokenOperator, ")"); err != nil {
		return nil, err
	}

	return ast.NewIndex(indexName, tableName, columns...), nil
}

// parseCreateUniqueIndex parses CREATE UNIQUE INDEX statements.
// Note: The INDEX token has already been consumed by parseCreateStatement
func (p *Parser) parseCreateUniqueIndex() (*ast.IndexNode, error) {
	p.skipWhitespace()

	// Get index name
	if p.current.Type != lexer.TokenIdentifier {
		return nil, fmt.Errorf("expected index name, got %s at position %d", p.current.Type, p.current.Start)
	}
	indexName := p.current.Value
	p.advance()

	p.skipWhitespace()

	if err := p.expect(lexer.TokenIdentifier, "ON"); err != nil {
		return nil, fmt.Errorf("expected ON after index name: %w", err)
	}

	p.skipWhitespace()

	// Get table name
	tableName, err := p.expectIdentifier()
	if err != nil {
		return nil, fmt.Errorf("expected table name: %w", err)
	}

	p.skipWhitespace()

	// Parse column list
	if err := p.expect(lexer.TokenOperator, "("); err != nil {
		return nil, fmt.Errorf("expected '(' for index columns: %w", err)
	}

	var columns []string
	for {
		p.skipWhitespace()

		columnName, err := p.expectIdentifier()
		if err != nil {
			return nil, fmt.Errorf("expected column name: %w", err)
		}
		columns = append(columns, columnName)

		p.skipWhitespace()

		if p.current.Type == lexer.TokenOperator && p.current.Value == "," {
			p.advance()
			continue
		} else if p.current.Type == lexer.TokenOperator && p.current.Value == ")" {
			break
		} else {
			return nil, fmt.Errorf("expected ',' or ')' in column list at position %d", p.current.Start)
		}
	}

	if err := p.expect(lexer.TokenOperator, ")"); err != nil {
		return nil, err
	}

	index := ast.NewIndex(indexName, tableName, columns...)
	index.SetUnique()
	return index, nil
}

// parseCreateType parses CREATE TYPE statements (for enums).
func (p *Parser) parseCreateType() (*ast.EnumNode, error) {
	if err := p.expect(lexer.TokenIdentifier, "TYPE"); err != nil {
		return nil, err
	}

	p.skipWhitespace()

	// Get type name
	typeName, err := p.expectIdentifier()
	if err != nil {
		return nil, fmt.Errorf("expected type name: %w", err)
	}

	p.skipWhitespace()

	if err := p.expect(lexer.TokenIdentifier, "AS"); err != nil {
		return nil, fmt.Errorf("expected AS after type name: %w", err)
	}

	p.skipWhitespace()

	if err := p.expect(lexer.TokenIdentifier, "ENUM"); err != nil {
		return nil, fmt.Errorf("expected ENUM after AS: %w", err)
	}

	p.skipWhitespace()

	// Parse enum values
	if err := p.expect(lexer.TokenOperator, "("); err != nil {
		return nil, fmt.Errorf("expected '(' for enum values: %w", err)
	}

	var values []string
	for {
		p.skipWhitespace()

		if p.current.Type == lexer.TokenOperator && p.current.Value == ")" {
			break
		}

		if p.current.Type != lexer.TokenString {
			return nil, fmt.Errorf("expected string value for enum at position %d", p.current.Start)
		}

		// Remove quotes from string value
		value := p.current.Value
		if len(value) >= 2 && (value[0] == '\'' || value[0] == '"') {
			value = value[1 : len(value)-1]
		}
		values = append(values, value)
		p.advance()

		p.skipWhitespace()

		if p.current.Type == lexer.TokenOperator && p.current.Value == "," {
			p.advance()
			continue
		} else if p.current.Type == lexer.TokenOperator && p.current.Value == ")" {
			break
		} else {
			return nil, fmt.Errorf("expected ',' or ')' in enum values at position %d", p.current.Start)
		}
	}

	if err := p.expect(lexer.TokenOperator, ")"); err != nil {
		return nil, err
	}

	return ast.NewEnum(typeName, values...), nil
}

// parseCreateDomain parses CREATE DOMAIN statements (PostgreSQL).
func (p *Parser) parseCreateDomain() (*ast.CommentNode, error) {
	if err := p.expect(lexer.TokenIdentifier, "DOMAIN"); err != nil {
		return nil, err
	}

	p.skipWhitespace()

	// Get domain name
	domainName, err := p.expectIdentifier()
	if err != nil {
		return nil, fmt.Errorf("expected domain name: %w", err)
	}

	p.skipWhitespace()

	if err := p.expect(lexer.TokenIdentifier, "AS"); err != nil {
		return nil, fmt.Errorf("expected AS after domain name: %w", err)
	}

	p.skipWhitespace()

	// Get base type
	baseType, err := p.parseColumnType()
	if err != nil {
		return nil, fmt.Errorf("expected base type: %w", err)
	}

	// For now, we'll represent domains as comments since they're not in the AST
	// In a full implementation, you'd want to add a DomainNode to the AST
	domainText := fmt.Sprintf("CREATE DOMAIN %s AS %s", domainName, baseType)

	// Parse optional constraints (CHECK, etc.)
	for {
		p.skipWhitespace()

		if p.current.Type != lexer.TokenIdentifier {
			break
		}

		keyword := strings.ToUpper(p.current.Value)
		if keyword == "CHECK" {
			p.advance()
			p.skipWhitespace()
			checkExpr, err := p.parseCheckExpression()
			if err != nil {
				return nil, fmt.Errorf("expected check expression: %w", err)
			}
			domainText += fmt.Sprintf(" CHECK (%s)", checkExpr)
		} else {
			break
		}
	}

	return ast.NewComment(domainText), nil
}

// parseCommentStatement parses COMMENT ON statements (PostgreSQL).
func (p *Parser) parseCommentStatement() (*ast.CommentNode, error) {
	if err := p.expect(lexer.TokenIdentifier, "COMMENT"); err != nil {
		return nil, err
	}

	p.skipWhitespace()

	if err := p.expect(lexer.TokenIdentifier, "ON"); err != nil {
		return nil, fmt.Errorf("expected ON after COMMENT: %w", err)
	}

	p.skipWhitespace()

	// Parse the object type (TABLE, COLUMN, etc.)
	objectType, err := p.expectIdentifier()
	if err != nil {
		return nil, fmt.Errorf("expected object type: %w", err)
	}

	p.skipWhitespace()

	// Parse the object name (could be table.column for columns)
	var objectName strings.Builder
	for {
		if p.current.Type == lexer.TokenIdentifier ||
			(p.current.Type == lexer.TokenOperator && p.current.Value == ".") {
			objectName.WriteString(p.current.Value)
			p.advance()
		} else {
			break
		}
	}

	p.skipWhitespace()

	if err := p.expect(lexer.TokenIdentifier, "IS"); err != nil {
		return nil, fmt.Errorf("expected IS after object name: %w", err)
	}

	p.skipWhitespace()

	// Get the comment text
	if p.current.Type != lexer.TokenString {
		return nil, fmt.Errorf("expected string for comment text at position %d", p.current.Start)
	}

	commentText := fmt.Sprintf("COMMENT ON %s %s IS %s",
		strings.ToUpper(objectType), objectName.String(), p.current.Value)
	p.advance()

	return ast.NewComment(commentText), nil
}
