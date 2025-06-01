package lexer

import (
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

func (tt TokenType) String() string {
	switch tt {
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
func (l *Lexer) ignore() { //nolint:unused // TODO: not used yet
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
		case ch == '`':
			return l.scanBacktickedIdentifier()
		case ch == '-' && l.peekNext() == '-':
			return l.scanLineComment()
		case ch == '/' && l.peekNext() == '*':
			return l.scanBlockComment()
		case unicode.IsLetter(ch) || ch == '_':
			return l.scanIdentifier()
		case unicode.IsDigit(ch):
			return l.scanNumber()
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

// scanBacktickedIdentifier scans a backtick-quoted identifier (MySQL style)
func (l *Lexer) scanBacktickedIdentifier() Token {
	l.advance() // consume opening backtick

	for {
		ch := l.peek()
		if ch == 0 {
			// Unterminated identifier - return what we have
			break
		}

		if ch == '`' {
			l.advance() // consume closing backtick
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

	return l.emit(TokenIdentifier)
}

// scanNumber scans a numeric literal
func (l *Lexer) scanNumber() Token {
	for {
		ch := l.peek()
		if !unicode.IsDigit(ch) && ch != '.' {
			break
		}
		l.advance()
	}
	return l.emit(TokenIdentifier) // Treat numbers as identifiers for simplicity
}

// scanOperator scans operators and other symbols
func (l *Lexer) scanOperator() Token {
	l.advance()
	return l.emit(TokenOperator)
}
