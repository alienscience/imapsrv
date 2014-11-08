
package imapsrv

import (
	"bufio"
	"fmt"
	"strconv"
	"bytes"
)

type lexer struct {
	reader  *bufio.Reader
	current byte
}

// The lexer produces tokens
type token struct {
	value   string
	tokType tokenType
}

// Token types
type tokenType int

const (
	stringTokenType = iota
	eolTokenType
	invalidTokenType
)

// Literal tokens
var invalidToken = &token{"", invalidTokenType}
var eolToken = &token{"", eolTokenType}

// Ascii codes
const (
	endOfInput  = 0x00
	cr          = 0x0d
	lf          = 0x0a
	space       = 0x20
	doubleQuote = 0x22
	zero        = 0x30
	nine        = 0x39
	leftCurly   = 0x7b
	rightCurly  = 0x7d
	leftParenthesis  = 0x28
	rightParenthesis  = 0x29
	rightBracket = 0x5d
	percent = 0x25
	asterisk =0x2a
	backslash   = 0x5c
)

// char not present in the astring charset
var astringExceptionsChar = []byte {
	space,
	leftParenthesis,
	rightParenthesis,
	rightBracket,
	percent,
	asterisk,
	backslash,
	leftCurly,
}

// Create an IMAP lexer
func createLexer(in *bufio.Reader) *lexer {

	// Fake the first character - use a space that will be skipped
	return &lexer{reader: in, current: space}
}

// Get the next token
func (l *lexer) next() *token {

	// Skip whitespace
	l.skipSpace()

	// Consider the first character - this gives the type of argument
	switch l.current {
	case cr:
		l.consumeEol()
		return eolToken
	case doubleQuote:
		l.consume()
		return l.qstring()
	case leftCurly:
		l.consume()
		return l.literal()
	default:
		return l.astring()
	}
}

// Read a quoted string
func (l *lexer) qstring() *token {

	var buffer = make([]byte, 0, 16)

	// Collect the characters that are within double quotes
	for l.current != doubleQuote {

		switch l.current {
		case cr, lf:
			err := parseError(fmt.Sprintf(
				"Unexpected character %q in quoted string", l.current))
			panic(err)
		case backslash:
			l.consume()
			buffer = append(buffer, l.current)
		default:
			buffer = append(buffer, l.current)
		}

		// Get the next character
		l.consume()
	}

	// Ignore the closing quote
	l.consume()

	return &token{string(buffer), stringTokenType}
}

// Parse a length tagged literal
func (l *lexer) literal() *token {

	lengthBuffer := make([]byte, 0, 8)

	// Get the length of the literal
	for l.current != rightCurly {
		if l.current < zero || l.current > nine {
			err := parseError(fmt.Sprintf(
				"Unexpected character %q in literal length", l.current))
			panic(err)
		}

		lengthBuffer = append(lengthBuffer, l.current)
		l.consume()
	}

	// Extract the literal length as an int
	length, err := strconv.ParseInt(string(lengthBuffer), 10, 32)
	if err != nil {
		panic(parseError(err.Error()))
	}

	// Consume the right curly and the newline that should follow
	l.consumeEol()

	buffer := make([]byte, 0, 64)

	// Read the literal
	for length > 0 {
		buffer = append(buffer, l.current)
		length -= 1
		l.consume()
	}

	return &token{string(buffer), stringTokenType}
}

// A very loose interpretation of a non-quoted string
func (l *lexer) astring() *token {

	buffer := make([]byte, 0, 16)

	for l.current > space && -1 == bytes.IndexByte(astringExceptionsChar, l.current) && l.current < 0x7f {
		buffer = append(buffer, l.current)
		l.consume()
	}

	// Check that characters were consumed
	if len(buffer) == 0 {
		panic(parseError("Expected ASTRING"))
	}

	return &token{string(buffer), stringTokenType}
}

// Skip whitespace
func (l *lexer) skipSpace() {
	if l.current == space || l.current == lf {
		l.consume()
	}
}

// Consume until end of line
func (l *lexer) consumeEol() {

	// Consume until the linefeed
	for l.current != lf {
		l.consume()
	}
}

// Move forward 1 byte
func (l *lexer) consume() {
	var err error
	l.current, err = l.reader.ReadByte()

	// Panic with a parser error if the read fails
	if err != nil {
		panic(parseError(err.Error()))
	}
}
