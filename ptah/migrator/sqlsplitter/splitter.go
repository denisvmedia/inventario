package sqlsplitter

import (
	"strings"
	"unicode"
)

// TokenType represents the type of SQL token
type TokenType int

const (
	TokenUnknown TokenType = iota
	TokenString
	TokenComment
	TokenSemicolon
	TokenWhitespace
	TokenIdentifier
	TokenOperator
	TokenEOF
)

func (tokenType TokenType) String() string {
	switch tokenType {
	case TokenUnknown:
		return "Unknown"
	case TokenString:
		return "String"
	case TokenComment:
		return "Comment"
	case TokenSemicolon:
		return "Semicolon"
	case TokenWhitespace:
		return "Whitespace"
	case TokenIdentifier:
		return "Identifier"
	case TokenOperator:
		return "Operator"
	case TokenEOF:
		return "EOF"
	default:
		return "Unknown"
	}
}

// Token represents a single SQL token
type Token struct {
	Type  TokenType
	Value string
	Start int
	End   int
}

// Lexer tokenizes SQL input
type Lexer struct {
	input string
	pos   int
	start int
}

// NewLexer creates a new SQL lexer
func NewLexer(input string) *Lexer {
	return &Lexer{
		input: input,
		pos:   0,
		start: 0,
	}
}

// peek returns the character at the current position without advancing
func (l *Lexer) peek() rune {
	if l.pos >= len(l.input) {
		return 0
	}
	return rune(l.input[l.pos])
}

// peekNext returns the character at the next position without advancing
func (l *Lexer) peekNext() rune {
	if l.pos+1 >= len(l.input) {
		return 0
	}
	return rune(l.input[l.pos+1])
}

// advance moves to the next character and returns it
func (l *Lexer) advance() rune {
	if l.pos >= len(l.input) {
		return 0
	}
	ch := rune(l.input[l.pos])
	l.pos++
	return ch
}

// emit creates a token with the current accumulated text
func (l *Lexer) emit(tokenType TokenType) Token {
	token := Token{
		Type:  tokenType,
		Value: l.input[l.start:l.pos],
		Start: l.start,
		End:   l.pos,
	}
	l.start = l.pos
	return token
}

// ignore skips the current accumulated text
func (l *Lexer) ignore() {
	l.start = l.pos
}

// NextToken returns the next token from the input
func (l *Lexer) NextToken() Token {
	for {
		ch := l.peek()

		if ch == 0 {
			return l.emit(TokenEOF)
		}

		switch {
		case unicode.IsSpace(ch):
			return l.scanWhitespace()
		case ch == ';':
			l.advance()
			return l.emit(TokenSemicolon)
		case ch == '\'' || ch == '"':
			return l.scanString()
		case ch == '-' && l.peekNext() == '-':
			return l.scanLineComment()
		case ch == '/' && l.peekNext() == '*':
			return l.scanBlockComment()
		case unicode.IsLetter(ch) || ch == '_':
			return l.scanIdentifier()
		default:
			return l.scanOperator()
		}
	}
}

// scanWhitespace scans whitespace characters
func (l *Lexer) scanWhitespace() Token {
	for unicode.IsSpace(l.peek()) {
		l.advance()
	}
	return l.emit(TokenWhitespace)
}

// scanString scans a quoted string literal
func (l *Lexer) scanString() Token {
	quote := l.advance() // consume opening quote

	for {
		ch := l.peek()
		if ch == 0 {
			// Unterminated string - return what we have
			break
		}

		if ch == quote {
			l.advance() // consume closing quote
			break
		}

		if ch == '\\' {
			l.advance() // consume backslash
			if l.peek() != 0 {
				l.advance() // consume escaped character
			}
		} else {
			l.advance()
		}
	}

	return l.emit(TokenString)
}

// scanLineComment scans a line comment (-- comment)
func (l *Lexer) scanLineComment() Token {
	l.advance() // consume first -
	l.advance() // consume second -

	for {
		ch := l.peek()
		if ch == 0 || ch == '\n' || ch == '\r' {
			break
		}
		l.advance()
	}

	return l.emit(TokenComment)
}

// scanBlockComment scans a block comment (/* comment */)
func (l *Lexer) scanBlockComment() Token {
	l.advance() // consume /
	l.advance() // consume *

	for {
		ch := l.peek()
		if ch == 0 {
			// Unterminated comment - return what we have
			break
		}

		if ch == '*' && l.peekNext() == '/' {
			l.advance() // consume *
			l.advance() // consume /
			break
		}

		l.advance()
	}

	return l.emit(TokenComment)
}

// scanIdentifier scans an identifier or keyword
func (l *Lexer) scanIdentifier() Token {
	for {
		ch := l.peek()
		if !unicode.IsLetter(ch) && !unicode.IsDigit(ch) && ch != '_' {
			break
		}
		l.advance()
	}
	return l.emit(TokenIdentifier)
}

// scanOperator scans operators and other symbols
func (l *Lexer) scanOperator() Token {
	l.advance()
	return l.emit(TokenOperator)
}

// RemoveComments removes all SQL comments from the input string using lexer-based parsing.
// This properly handles comments within string literals and preserves the structure of the SQL.
// Both line comments (-- comment) and block comments (/* comment */) are removed.
func RemoveComments(sql string) string {
	if strings.TrimSpace(sql) == "" {
		return sql
	}

	lexer := NewLexer(sql)
	var result strings.Builder

	for {
		token := lexer.NextToken()
		// fmt.Println(token.Type, " -> ", token.Value)

		if token.Type == TokenEOF {
			break
		}

		// Skip comment tokens, include everything else
		if token.Type != TokenComment {
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

	lexer := NewLexer(sql)
	var statements []string
	var currentStatement strings.Builder

	for {
		token := lexer.NextToken()

		if token.Type == TokenEOF {
			break
		}

		if token.Type == TokenSemicolon {
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
