package imapsrv

import (
	"bufio"
	"fmt"
	"strings"
)

// An IMAP parser
type parser struct {
	lexer *lexer
}

// An Error from the IMAP parser or lexer
type parseError string

// Parse errors satisfy the error interface
func (e parseError) Error() string {
	return string(e)
}

// Create an imap parser
func createParser(in *bufio.Reader) *parser {
	lexer := createLexer(in)
	return &parser{lexer: lexer}
}

//----- Commands ---------------------------------------------------------------

// Parse the next command
func (p *parser) next() command {

	// All commands start on a new line
	p.lexer.newLine()

	// Expect a tag followed by a command
	tag := p.expectString(p.lexer.tag)
	rawCommand := p.expectString(p.lexer.astring)

	// Parse the command based on its lowercase value
	// This makes typing over telnet easier
	lcCommand := strings.ToLower(rawCommand)

	switch lcCommand {
	case "noop":
		return p.noop(tag)
	case "capability":
		return p.capability(tag)
	case "login":
		return p.login(tag)
	case "logout":
		return p.logout(tag)
	case "select":
		return p.selectC(tag)
	case "list":
		return p.list(tag)
	case "fetch":
		return p.fetch(tag)
	default:
		return p.unknown(tag, rawCommand)
	}
}

// Create a NOOP command
func (p *parser) noop(tag string) command {
	return &noop{tag: tag}
}

// Create a capability command
func (p *parser) capability(tag string) command {
	return &capability{tag: tag}
}

// Create a login command
func (p *parser) login(tag string) command {

	// Get the command arguments
	userId := p.expectString(p.lexer.astring)
	password := p.expectString(p.lexer.astring)

	// Create the command
	return &login{tag: tag, userId: userId, password: password}
}

// Create a logout command
func (p *parser) logout(tag string) command {
	return &logout{tag: tag}
}

// Create a select command
func (p *parser) selectC(tag string) command {

	// Get the mailbox name
	mailbox := p.expectString(p.lexer.astring)

	return &selectMailbox{tag: tag, mailbox: mailbox}
}

// Create a list command
func (p *parser) list(tag string) command {

	// Get the command arguments
	reference := p.expectString(p.lexer.astring)

	if strings.EqualFold(reference, "inbox") {
		reference = "INBOX"
	}
	mailbox := p.expectString(p.lexer.listMailbox)

	return &list{tag: tag, reference: reference, mboxPattern: mailbox}
}

// Create a fetch command
func (p *parser) fetch(tag string) command {

	ret := createFetchCommand(tag)

	// Get the command arguments
	// The first argument is always a sequence set
	ret.sequenceSet = p.expectSequenceSet()

	// The next token can be a fetch macro, a fetch attachment or an open bracket
	ok, macro := p.lexer.fetchMacro()
	if ok {
		ret.macro = macro
		return ret
	}

	isMultiple := p.lexer.leftParen()
	ret.attachments = p.expectFetchAttachments(isMultiple)

	return ret

}

// Create a placeholder for an unknown command
func (p *parser) unknown(tag string, cmd string) command {
	return &unknown{tag: tag, cmd: cmd}
}

//----- Helper functions -------------------------------------------------------

// Get a string token using the given lexer function
// If the lexing fails then panic
func (p *parser) expectString(lex func() (bool, string)) string {
	ok, ret := lex()
	if !ok {
		msg := fmt.Sprintf("Parser unexpected %q", p.lexer.current())
		err := parseError(msg)
		panic(err)
	}

	return ret
}

// A sequence set or panic
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
			item.end = p.expectSequenceNumber()
		}

		// Is there a sequence set delimiter?
		ok = p.lexer.sequenceDelimiter()
		if !ok {
			// This is the end of the sequence set
			break
		}
	}

	// Check that there is something to return
	if len(ret) == 0 {
		msg := fmt.Sprintf("Parser expected sequence set got %q", p.lexer.current())
		err := parseError(msg)
		panic(err)
	}

	return ret
}

// A sequence number or panic
func (p *parser) expectSequenceNumber() sequenceNumber {

	ok, seqnum := p.lexer.sequenceNumber()

	if !ok {
		// This could be a wildcard
		ok = p.lexer.sequenceWildcard()

		if !ok {
			msg := fmt.Sprintf("Parser unexpected %q", p.lexer.current())
			err := parseError(msg)
			panic(err)
		}

		return sequenceNumber{isWildcard: true}
	}

	return sequenceNumber{value: seqnum}
}

// Expect one or more fetch attachments
func (p *parser) expectFetchAttachments(isMultiple bool) []fetchAttachment {

	ret := make([]fetchAttachment, 0, 4)

	for {
		att := fetchAttachment{}

		// Check for closing parenthesis
		if isMultiple && p.lexer.rightParen() {
			return ret
		}

		// Get the fetch attachment
		att.id = p.expectFetchAttachment()

		// Some fetch attachments have arguments
		if att.id == bodyFetchAtt {
			ok, section := p.section()
			if ok {
				att.id = bodySectionFetchAtt
				att.partial = p.optionalFetchPartial()
			}
		} else if att.id == bodyPeekFetchAtt {
			ok, section := p.section()
			if !ok {
				err := parseError("BODY.PEEK must be followed by section")
				panic(err)
			}
			att.partial = p.optionalFetchPartial()
		}

		ret = append(ret, att)

		// Is there only one fetch attachment?
		if !isMultiple {
			return ret
		}
	}

}

// Expect a fetch attachment
func (p *parser) expectFetchAttachment() fetchAttachmentId {
	ok, ret := p.lexer.fetchAttachment()
	if !ok {
		err := parseError("Expected fetch attachment")
		panic(err)
	}

	return ret
}

// Expect a fetch section
func (p *parser) section() (bool, *fetchSection) {

	// The section must start with a bracket
	if !p.lexer.leftBracket() {
		return false, nil
	}

	ret := &fetchSection{
		fields: make([]string, 0, 4),
	}

	ok, part := p.lexer.sectionMsgText()
	if ok {
		ret.part = part
	} else {
		// This must be a section part
		ret.numericSpecifier = p.lexer.sectionPart()

		// Followed by an optional "." and section text
		if p.lexer.dot() {
			ret.mime, ret.part = p.expectSectionText()
		}
	}

	return true, ret
}
