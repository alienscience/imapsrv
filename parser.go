package imapsrv

import (
	"bufio"
	"fmt"
	"log"
	"strings"
	"sync"
)

// parser can parse IMAP commands
type parser struct {
	lexer *lexer
}

var (
	commands   = make(map[string]commandCreator)
	commandsMu sync.RWMutex
)

// parseError is an Error from the IMAP parser or lexer
type parseError string

// Error returns the string representation of the parseError
func (e parseError) Error() string {
	return string(e)
}

// createParser creates a new IMAP parser, reading from the Reader
func createParser(in *bufio.Reader) *parser {
	lexer := createLexer(in)
	return &parser{
		lexer: lexer,
	}
}

//----- Commands ---------------------------------------------------------------

// next attempts to read the next command
func (p *parser) next() command {

	// All commands start on a new line
	p.lexer.newLine()

	// Expect a tag followed by a command
	tag := p.expectString(p.lexer.tag)

	rawCommand := p.expectString(p.lexer.astring)

	// Parse the command based on its lowercase value
	// This makes typing over telnet easier
	lcCommand := strings.ToLower(rawCommand)

	log.Println("Processing", tag, lcCommand)

	if creator, exists := commands[lcCommand]; exists {
		return creator(p, tag)
	} else {
		return p.unknown(tag, rawCommand)
	}
}

func registerCommand(cmd string, creator commandCreator) {
	commandsMu.Lock()
	defer commandsMu.Unlock()
	commands[strings.ToLower(cmd)] = creator
}

// unknown creates a placeholder for an unknown command
func (p *parser) unknown(tag string, cmd string) command {
	return &unknown{tag: tag, cmd: cmd}
}

//----- Helper functions -------------------------------------------------------

// expectString gets a string token using the given lexer function
// If the lexing fails, then this will panic
func (p *parser) expectString(lex func() (bool, string)) string {
	ok, ret := lex()
	if !ok {
		parserPanic("Parser unexpected %q", p.lexer.current())
	}

	return ret
}

// A sequence set or panic
//
//    sequence-set    = (seq-number / seq-range) *("," sequence-set)
func (p *parser) expectSequenceSet() []sequenceRange {

	ret := make([]sequenceRange, 0, 4)
	ok := false

	// Loop through sequence sets until the end is detected
	for {
		item := sequenceRange{}

		// Get a sequence number
		item.start = p.expectSequenceNumber()

		// Is this a sequence range?
		ok = p.lexer.sequenceRangeSeparator()
		if ok {
			end := p.expectSequenceNumber()
			item.end = &end
		}

		ret = append(ret, item)

		// Is there a sequence set delimiter?
		ok = p.lexer.sequenceDelimiter()
		if !ok {
			// This is the end of the sequence set
			break
		}
	}

	// Check that there is something to return
	if len(ret) == 0 {
		parserPanic("Parser expected sequence set got %q", p.lexer.current())
	}

	return ret
}

// A sequence number or panic
//
//    seq-number      = nz-number / "*"
func (p *parser) expectSequenceNumber() int32 {

	ok, seqnum := p.lexer.nonZeroInteger()

	if !ok {
		// This could be a wildcard
		ok = p.lexer.sequenceWildcard()

		if !ok {
			parserPanic("Parser unexpected %q", p.lexer.current())
		}

		return largestSequenceNumber
	}

	return int32(seqnum)
}

// Expect one or more fetch attachments
//
//    fetch-att / "(" fetch-att *(SP fetch-att) ")")
func (p *parser) expectFetchAttachments(isMultiple bool) []fetchAttachment {

	ret := make([]fetchAttachment, 0, 4)

	for {
		// Check for closing parenthesis
		if isMultiple && p.lexer.rightParen() {
			return ret
		}

		// Get the fetch attachment
		att := p.expectFetchAttachment()

		// Some fetch attachments have section arguments
		switch attStruct := att.(type) {
		case *bodyFetchAtt:
			// Optional section argument
			ok, section := p.section()
			if ok {
				// Change the type of fetch attachment to
				// include a section
				sectionAtt := &bodySectionFetchAtt{
					fetchSection: *section,
				}
				sectionAtt.fetchSection.partial = p.optionalFetchPartial()
				ret = append(ret, sectionAtt)
			} else {
				ret = append(ret, att)
			}
		case *bodyPeekFetchAtt:
			// Mandatory section argument
			ok, section := p.section()
			if !ok {
				parserPanic("BODY.PEEK must be followed by section")
			}
			attStruct.fetchSection = *section
			attStruct.fetchSection.partial = p.optionalFetchPartial()
			ret = append(ret, attStruct)
		default:
			// No section arguments
			ret = append(ret, att)
		}

		// Is there only one fetch attachment?
		if !isMultiple {
			return ret
		}
	}

}

// Expect a fetch attachment
//
//    fetch-att       = "ENVELOPE" / "FLAGS" / "INTERNALDATE" /
//                      "RFC822" [".HEADER" / ".SIZE" / ".TEXT"] /
//                      "BODY" ["STRUCTURE"] / "UID" /
//                      "BODY"
//                      "BODY.PEEK"
func (p *parser) expectFetchAttachment() fetchAttachment {
	ok, ret := p.lexer.fetchAttachment()
	if !ok {
		parserPanic("Expected fetch attachment")
	}

	return ret
}

// Expect a fetch section
//
//    section         = "[" [section-spec] "]"
//    section-spec    = section-msgtext / (section-part ["." section-text])
func (p *parser) section() (bool, *fetchSection) {

	// The section must start with a [
	if !p.lexer.leftBracket() {
		return false, nil
	}

	ret := &fetchSection{}

	ok := p.sectionMsgText(ret)
	if !ok {
		// This must be a section part
		ret.section = p.expectSectionPart()

		// Followed by an optional "." and section text
		if p.lexer.dot() {
			p.expectSectionText(ret)
		}
	}

	// The section must end with a ]
	if !p.lexer.rightBracket() {
		parserPanic("Expected section to end with ']'")
	}

	return true, ret
}

// Get the section-msgtext that can be part of a section spec
//
//    section-msgtext = "HEADER" / "HEADER.FIELDS" [".NOT"] SP header-list /
//                      "TEXT"
func (p *parser) sectionMsgText(section *fetchSection) bool {

	// The section-msgtext must start with a part specifier
	ok, partSpecifier := p.lexer.partSpecifier()
	if !ok {
		p.lexer.pushBackToken()
		return false
	}

	// Some part specifiers are followed by a header-list
	switch partSpecifier {
	case headerFieldsPart, headerFieldsNotPart:
		p.lexer.skipSpace()
		section.fields = p.expectHeaderList()
	}

	// Success
	section.part = partSpecifier
	return true
}

// Optionally read a fetch partial, returns nil if no fetchPartial exists
//
//   "<" number "." nz-number ">"
func (p *parser) optionalFetchPartial() *fetchPartial {

	// The fetch partial must start with a less than sign
	if !p.lexer.lessThan() {
		return nil
	}

	// Then a number
	ok, n := p.lexer.integer()

	if !ok {
		parserPanic("Expected number in fetch partial")
	}

	ret := &fetchPartial{}
	ret.fromOctet = n

	// Then a non-zero number
	ok, nz := p.lexer.nonZeroInteger()

	if !ok {
		parserPanic("Expected none-zero number in fetch partial")
	}

	ret.length = nz

	// Then a greater than sign
	if !p.lexer.greaterThan() {
		parserPanic("Fetch partial should end with '>'")
	}

	return ret
}

// Parse the section-part
//
//    section-spec    = section-msgtext / (section-part ["." section-text])
//    section-part    = nz-number *("." nz-number)
//    section-text    = section-msgtext / "MIME"
func (p *parser) expectSectionPart() []uint32 {

	ret := make([]uint32, 0, 4)

	// Loop through the section and subsection numbers
	for {
		ok, nz := p.lexer.nonZeroInteger()
		if !ok {
			// This might be the start of the section text
			if len(ret) > 0 {
				// Move back to the "."
				p.lexer.pushBackToken()
				p.lexer.pushBack()
				return ret
			} else {
				parserPanic("Expected a non-zero number in section-part")
			}
		}

		ret = append(ret, nz)

		if !p.lexer.dot() {
			return ret
		}
	}
}

// Parse the section text
//
//    section-text    = section-msgtext / "MIME"
func (p *parser) expectSectionText(section *fetchSection) {

	// Is this section-msgtext?
	if p.sectionMsgText(section) {
		return
	}

	// It must be "MIME"
	if !p.lexer.mime() {
		parserPanic("Expected section-msgtext or MIME")
	}

	section.part = mimePart
}

// Get a list of header fields
//
//    header-list     = "(" header-fld-name *(SP header-fld-name) ")"
//    header-fld-name = astring
func (p *parser) expectHeaderList() []string {

	if !p.lexer.leftParen() {
		parserPanic("Expected open paren at start of header-list")
	}

	ret := make([]string, 0, 4)

	for {
		// Get the header field name
		ok, headerFieldName := p.lexer.astring()

		if !ok {
			parserPanic("Expected header-fld-name in header-list")
		}

		ret = append(ret, headerFieldName)

		// Stop if there is a closing paren
		if p.lexer.rightParen() {
			return ret
		}
	}
}

// Report an error
func parserPanic(format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	err := parseError(msg)
	panic(err)
}
