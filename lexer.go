package imapsrv

import (
	"bufio"
	"bytes"
	"fmt"
	"net/textproto"
	"strconv"
	"strings"
)

type lexer struct {
	// Line based reader
	reader *textproto.Reader
	// The current line
	line []byte
	// The index to the current character
	idx int
	// The start of tokens, used for rewinding to the previous
	// token. The lexer does not rewind tokens itself, this is left
	// to the parser.
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
	comma            = 0x2c
	minus            = 0x2d
	dot              = 0x2e
	zero             = 0x30
	nine             = 0x39
	colon            = 0x3a
	leftCurly        = 0x7b
	rightCurly       = 0x7d
	leftParenthesis  = 0x28
	rightParenthesis = 0x29
	ltChar           = 0x3c
	gtChar           = 0x3e
	leftBracketChar  = 0x5b
	rightBracketChar = 0x5d
	percent          = 0x25
	asterisk         = 0x2a
	backslash        = 0x5c
)

// char not present in the astring charset
var astringExceptionsChar = []byte{
	space,
	leftParenthesis,
	rightParenthesis,
	percent,
	asterisk,
	backslash,
	leftCurly,
}

// char not present in the tag charset
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

// char not present in the list-mailbox charset
var listMailboxExceptionsChar = []byte{
	space,
	leftParenthesis,
	rightParenthesis,
	rightBracketChar,
	backslash,
	leftCurly,
}

// Create a partially initialised IMAP lexer
// lexer.newLine() must be the first call to this lexer
func createLexer(in *bufio.Reader) *lexer {
	return &lexer{reader: textproto.NewReader(in)}
}

//-------- IMAP tokens ---------------------------------------------------------

// An astring
func (l *lexer) astring() (bool, string) {
	l.skipSpace()
	l.startToken()

	return l.generalString("ASTRING", astringExceptionsChar)
}

// A tag string
func (l *lexer) tag() (bool, string) {
	l.skipSpace()
	l.startToken()

	return l.nonquoted("TAG", tagExceptionsChar)
}

// A list mailbox
func (l *lexer) listMailbox() (bool, string) {
	l.skipSpace()
	l.startToken()

	return l.generalString("LIST-MAILBOX", listMailboxExceptionsChar)
}

// An integer
func (l *lexer) integer() (bool, int32) {
	l.startToken()

	// Read a sequence of digits with sign
	buffer := make([]byte, 0, 8)

	current := l.current()

	for current >= zero && current <= nine || current == minus {
		buffer = append(buffer, current)
		current = l.consume()
	}

	// Check that at least one character was read
	if len(buffer) == 0 {
		return false, 0
	}

	// Convert to a number
	num, err := strconv.ParseInt(string(buffer), 10, 32)
	if err != nil {
		return false, 0
	}

	return true, int32(num)

}

// A non-zero integer
func (l *lexer) nonZeroInteger() (bool, uint32) {

	l.startToken()

	// Read a sequence of digits
	buffer := make([]byte, 0, 8)

	current := l.current()

	for current >= zero && current <= nine {
		buffer = append(buffer, current)
		current = l.consume()
	}

	// Check that at least one digit was read
	if len(buffer) == 0 {
		return false, 0
	}

	// Convert to a number
	num, err := strconv.ParseUint(string(buffer), 10, 32)
	if err != nil {
		return false, 0
	}

	return true, uint32(num)
}

// A sequence range separator
func (l *lexer) sequenceRangeSeparator() bool {
	if l.current() == colon {
		l.consume()
		return true
	}

	return false
}

// A sequence set delimiter
func (l *lexer) sequenceDelimiter() bool {
	if l.current() == comma {
		l.consume()
		return true
	}

	return false
}

// A sequence wildcard
func (l *lexer) sequenceWildcard() bool {
	if l.current() == asterisk {
		l.consume()
		return true
	}

	return false
}

// A fetch macro
func (l *lexer) fetchMacro() (bool, fetchCommandMacro) {
	l.skipSpace()
	l.startToken()

	ok, word := l.asciiWord()
	if !ok {
		return false, noFetchMacro
	}

	// Convert the word to a fetch macro
	lcWord := strings.ToLower(word)

	switch lcWord {
	case "all":
		return ok, allFetchMacro
	case "full":
		return ok, fullFetchMacro
	case "fast":
		return ok, fastFetchMacro
	default:
		return false, noFetchMacro
	}
}

// A fetch attachment
func (l *lexer) fetchAttachment() (bool, fetchAttachmentId) {
	l.skipSpace()
	l.startToken()

	ok, word := l.dottedWord()

	if !ok {
		return false, invalidFetchAtt
	}

	// Convert the word to a fetch attachment
	lcWord := strings.ToLower(word)

	switch lcWord {
	case "envelope":
		return ok, envelopeFetchAtt
	case "flags":
		return ok, flagsFetchAtt
	case "internaldate":
		return ok, internalDateFetchAtt
	case "rfc822.header":
		return ok, rfc822HeaderFetchAtt
	case "rfc822.size":
		return ok, rfc822SizeFetchAtt
	case "rfc822.text":
		return ok, rfc822TextFetchAtt
	case "body":
		// The parser will decide if this is BODY followed by section
		return ok, bodyFetchAtt
	case "bodystructure":
		return ok, bodyStructureFetchAtt
	case "uid":
		return ok, uidFetchAtt
	case "body.peek":
		return ok, bodyPeekFetchAtt
	default:
		return false, invalidFetchAtt
	}

}

// A fetch attachment
func (l *lexer) partSpecifier() (bool, partSpecifier) {
	l.skipSpace()
	l.startToken()

	ok, word := l.dottedWord()
	if !ok {
		return false, invalidPart
	}

	// Convert the word to a part specifier
	lcWord := strings.ToLower(word)

	switch lcWord {
	case "header":
		return ok, headerPart
	case "header.fields":
		return ok, headerFieldsPart
	case "header.fields.not":
		return ok, headerFieldsNotPart
	case "text":
		return ok, textPart
	default:
		return false, invalidPart
	}

}

// The word "MIME"
func (l *lexer) mime() bool {
	l.skipSpace()
	l.startToken()

	ok, word := l.asciiWord()

	if ok && word == "MIME" {
		return true
	}

	return false
}

// A left parenthesis
func (l *lexer) leftParen() bool {
	return l.singleChar(leftParenthesis)
}

// A right parenthesis
func (l *lexer) rightParen() bool {
	return l.singleChar(rightParenthesis)
}

// A less than
func (l *lexer) lessThan() bool {
	return l.singleChar(ltChar)
}

// A greater than
func (l *lexer) greaterThan() bool {
	return l.singleChar(gtChar)
}

// A left [
func (l *lexer) leftBracket() bool {
	return l.singleChar(leftBracketChar)
}

// A right ]
func (l *lexer) rightBracket() bool {
	return l.singleChar(rightBracketChar)
}

// A .
func (l *lexer) dot() bool {
	return l.singleChar(dot)
}

//-------- IMAP token helper functions -----------------------------------------

// Look for a single 8-bit character and say if it was successful or not
func (l *lexer) singleChar(ch byte) bool {
	if l.current() == ch {
		l.consume()
		return true
	}

	return false
}

// Handle a string that can be bare, a literal or quoted
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

// Read a quoted string
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

// Parse a length tagged literal
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

// A non-quoted string
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

// A word containing only ascii letters
func (l *lexer) asciiWord() (bool, string) {

	buffer := make([]byte, 0, 8)

	// Get the current byte
	c := l.current()

	for (c > 0x40 && c < 0x5b) || (c > 0x60 && c < 0x7b) {

		buffer = append(buffer, c)
		c = l.consume()
	}

	// Check that characters were consumed
	if len(buffer) == 0 {
		return false, ""
	}

	return true, string(buffer)
}

// Alpha-numeric containing dots, e.g a fetch attachment word
func (l *lexer) dottedWord() (bool, string) {

	buffer := make([]byte, 0, 16)

	c := l.current()

	// Uppercase alphanumeric or a dot
	for (c > 0x40 && c < 0x5b) || (c > 0x30 && c < 0x3a) || c == dot {

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

// Consume a single byte and return the new character
// Does not go through newlines
func (l *lexer) consume() byte {

	// Move to the next byte if possible
	if l.idx < len(l.line) {
		l.idx += 1
	}
	return l.current()
}

// Consume a single byte and return the new character
// Goes through newlines
func (l *lexer) consumeAll() byte {

	// Is there any line left?
	if l.idx >= len(l.line) {
		l.newLine()
		return l.current()
	}

	// Move to the next byte
	l.idx += 1
	return l.current()
}

// Get the current byte
func (l *lexer) current() byte {
	if l.idx < len(l.line) {
		return l.line[l.idx]
	}

	// Return linefeed if there are no characters left
	return lf
}

// Move onto a new line
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

// Skip spaces
func (l *lexer) skipSpace() {
	c := l.current()

	for c == space {
		c = l.consume()
	}
}

// Mark the start a new token
func (l *lexer) startToken() {
	l.tokens = append(l.tokens, l.idx)
}

// Move back one character
func (l *lexer) pushBack() {
	if l.idx < 1 {
		panic(parseError("pushBack called on first character of line"))
	}

	l.idx -= 1
}

// Move back one token
func (l *lexer) pushBackToken() {
	last := len(l.tokens) - 1
	l.idx = l.tokens[last]
	l.tokens = l.tokens[:last]
}
