package imapsrv

import (
	"bufio"
	"fmt"
	"strings"
)

// parser can parse IMAP commands
type parser struct {
	lexer *lexer
}

// parseError is an Error from the IMAP parser or lexer
type parseError string

// Error returns the string representation of the parseError
func (e parseError) Error() string {
	return string(e)
}

// createParser creates a new IMAP parser, reading from the Reader
func createParser(in *bufio.Reader) *parser {
	lexer := createLexer(in)
	return &parser{lexer: lexer}
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
	lcCommand := strings.ToLower(rawCommand)

	switch lcCommand {
	case "noop":
		return p.noop(tag)
	case "capability":
		return p.capability(tag)
	case "starttls":
		return p.starttls(tag)
	case "login":
		return p.login(tag)
	case "logout":
		return p.logout(tag)
	case "select":
		return p.selectCmd(tag)
	case "list":
		return p.list(tag)
	default:
		return p.unknown(tag, rawCommand)
	}
}

// noop creates a NOOP command
func (p *parser) noop(tag string) command {
	return &noop{tag: tag}
}

// capability creates a CAPABILITY command
func (p *parser) capability(tag string) command {
	return &capability{tag: tag}
}

// login creates a LOGIN command
func (p *parser) login(tag string) command {

	// Get the command arguments
	userId := p.expectString(p.lexer.astring)
	password := p.expectString(p.lexer.astring)

	// Create the command
	return &login{tag: tag, userId: userId, password: password}
}

// starttls creates a starttls command
func (p *parser) starttls(tag string) command {
	return &starttls{tag: tag}
}

// logout creates a LOGOUT command
func (p *parser) logout(tag string) command {
	return &logout{tag: tag}
}

// selectCmd creates a select command
func (p *parser) selectCmd(tag string) command {

	// Get the mailbox name
	mailbox := p.expectString(p.lexer.astring)

	return &selectMailbox{tag: tag, mailbox: mailbox}
}

// list creates a LIST command
func (p *parser) list(tag string) command {

	// Get the command arguments
	reference := p.expectString(p.lexer.astring)

	if strings.EqualFold(reference, "inbox") {
		reference = "INBOX"
	}
	mailbox := p.expectString(p.lexer.listMailbox)

	return &list{tag: tag, reference: reference, mboxPattern: mailbox}
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
		msg := fmt.Sprintf("Parser unexpected %q", p.lexer.current())
		err := parseError(msg)
		panic(err)
	}

	return ret
}
