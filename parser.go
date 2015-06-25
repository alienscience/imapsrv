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
