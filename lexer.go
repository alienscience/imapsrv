package imapsrv

import (
	"bufio"
	"bytes"
	"fmt"
	"net/textproto"
	"strconv"
)

// lexer is responsible for reading input, and making sense of it
type lexer struct {
	// Line based reader
	reader *textproto.Reader
	// The current line
	line []byte
	// The index to the current character
	idx int
	// The start of tokens, used for rewinding to the previous token
	tokens []int
}

// Ascii codes
const (
	endOfInput       = 0x00
	cr               = 0x0d
	lf               = 0x0a
	space            = 0x20
	doubleQuote      = 0x22
	plus             = 0x2b
	zero             = 0x30
	nine             = 0x39
	leftCurly        = 0x7b
	rightCurly       = 0x7d
	leftParenthesis  = 0x28
	rightParenthesis = 0x29
	rightBracket     = 0x5d
	percent          = 0x25
	asterisk         = 0x2a
	backslash        = 0x5c
)

// astringExceptionsChar is a list of chars that are not present in the astring charset
var astringExceptionsChar = []byte{
	space,
	leftParenthesis,
	rightParenthesis,
	percent,
	asterisk,
	backslash,
	leftCurly,
}

// tagExceptionsChar is a list of chars that are not present in the tag charset
var tagExceptionsChar = []byte{
	space,
	leftParenthesis,
	rightParenthesis,
	percent,
	asterisk,
	backslash,
	leftCurly,
	plus,
}

// listMailboxExceptionsChar is a list of chars that are  not present in the list-mailbox charset
var listMailboxExceptionsChar = []byte{
	space,
	leftParenthesis,
	rightParenthesis,
	rightBracket,
	backslash,
	leftCurly,
}

// createLexer creates a partially initialised IMAP lexer
// lexer.newLine() must be the first call to this lexer
func createLexer(in *bufio.Reader) *lexer {
	return &lexer{reader: textproto.NewReader(in)}
}

//-------- IMAP tokens ---------------------------------------------------------

// astring treats the input as a string
func (l *lexer) astring() (bool, string) {
	l.skipSpace()
	l.startToken()

	return l.generalString("ASTRING", astringExceptionsChar)
}

// tag treats the input as a tag string
func (l *lexer) tag() (bool, string) {
	l.skipSpace()
	l.startToken()

	return l.nonquoted("TAG", tagExceptionsChar)
}

// listMailbox treats the input as a list mailbox
func (l *lexer) listMailbox() (bool, string) {
	l.skipSpace()
	l.startToken()

	return l.generalString("LIST-MAILBOX", listMailboxExceptionsChar)
}

//-------- IMAP token helper functions -----------------------------------------

// generalString handles a string that can be bare, a literal or quoted
func (l *lexer) generalString(name string, exceptions []byte) (bool, string) {

	// Consider the first character - this gives the type of argument
	switch l.current() {
	case doubleQuote:
		l.consume()
		return true, l.qstring()
	case leftCurly:
		l.consume()
		return true, l.literal()
	default:
		return l.nonquoted(name, exceptions)
	}
}

// qstring reads a quoted string
func (l *lexer) qstring() string {

	var buffer = make([]byte, 0, 16)

	c := l.current()

	// Collect the characters that are within double quotes
	for c != doubleQuote {

		switch c {
		case cr, lf:
			err := parseError(fmt.Sprintf(
				"Unexpected character %q in quoted string", c))
			panic(err)
		case backslash:
			c = l.consume()
			buffer = append(buffer, c)
		default:
			buffer = append(buffer, c)
		}

		// Get the next byte
		c = l.consume()
	}

	// Ignore the closing quote
	l.consume()

	return string(buffer)
}

// literal parses a length tagged literal
// TODO: send a continuation request after the first line is read
func (l *lexer) literal() string {

	lengthBuffer := make([]byte, 0, 8)

	c := l.current()

	// Get the length of the literal
	for c != rightCurly {
		if c < zero || c > nine {
			err := parseError(fmt.Sprintf(
				"Unexpected character %q in literal length", c))
			panic(err)
		}

		lengthBuffer = append(lengthBuffer, c)
		c = l.consume()
	}

	// Extract the literal length as an int
	length, err := strconv.ParseInt(string(lengthBuffer), 10, 32)
	if err != nil {
		panic(parseError(err.Error()))
	}

	// Consider the next line
	l.newLine()

	// Does the literal have a valid length?
	if length <= 0 {
		return ""
	}

	// Read the literal
	buffer := make([]byte, 0, length)
	c = l.current()

	for {
		buffer = append(buffer, c)

		// Is this the end of the literal?
		length -= 1
		if length == 0 {
			break
		}

		c = l.consumeAll()
	}

	return string(buffer)
}

// nonquoted reads a non-quoted string
func (l *lexer) nonquoted(name string, exceptions []byte) (bool, string) {

	buffer := make([]byte, 0, 16)

	// Get the current byte
	c := l.current()

	for c > space && c < 0x7f && -1 == bytes.IndexByte(exceptions, c) {

		buffer = append(buffer, c)
		c = l.consume()
	}

	// Check that characters were consumed
	if len(buffer) == 0 {
		return false, ""
	}

	return true, string(buffer)
}

//-------- Low level lexer functions -------------------------------------------

// consume a single byte and return the new character
// Does not go through newlines
func (l *lexer) consume() byte {

	// Is there any line left?
	if l.idx >= len(l.line)-1 {
		// Return linefeed
		return lf
	}

	// Move to the next byte
	l.idx += 1
	return l.current()
}

// consumeAll a single byte and return the new character
// Goes through newlines
func (l *lexer) consumeAll() byte {

	// Is there any line left?
	if l.idx >= len(l.line)-1 {
		l.newLine()
		return l.current()
	}

	// Move to the next byte
	l.idx += 1
	return l.current()
}

// current gets the current byte
func (l *lexer) current() byte {
	return l.line[l.idx]
}

// newLine moves onto a new line
func (l *lexer) newLine() {

	// Read the line
	line, err := l.reader.ReadLineBytes()
	if err != nil {
		panic(parseError(err.Error()))
	}

	// Reset the lexer - we cannot rewind past line boundaries
	l.line = line
	l.idx = 0
	l.tokens = make([]int, 0, 8)
}

// skipSpace skips any spaces
func (l *lexer) skipSpace() {
	c := l.current()

	for c == space {
		c = l.consume()
	}
}

// startToken marks the start a new token
func (l *lexer) startToken() {
	l.tokens = append(l.tokens, l.idx)
}

// pushBack moves back one token
func (l *lexer) pushBack() {
	last := len(l.tokens) - 1
	l.idx = l.tokens[last]
	l.tokens = l.tokens[:last]
}
