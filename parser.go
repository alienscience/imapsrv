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

// Parse the next command
func (p *parser) next() command {

	// Expect a tag followed by a command
	tagToken := p.match(stringTokenType)
	commandToken := p.match(stringTokenType)

	// Parse the command based on its lowercase value
	rawCommand := commandToken.value
	lcCommand := strings.ToLower(rawCommand)
	tag := tagToken.value

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
	p.match(eolTokenType)
	return &noop{tag: tag}
}

// Create a capability command
func (p *parser) capability(tag string) command {
	p.match(eolTokenType)
	return &capability{tag: tag}
}

// Create a login command
func (p *parser) login(tag string) command {

	// Get the command arguments
	userId := p.match(stringTokenType).value
	password := p.match(stringTokenType).value
	p.match(eolTokenType)

	// Create the command
	return &login{tag: tag, userId: userId, password: password}
}

// Create a logout command
func (p *parser) logout(tag string) command {
	p.match(eolTokenType)
	return &logout{tag: tag}
}

// Create a select command
func (p *parser) selectC(tag string) command {
	// Get the mailbox name
	mailbox := p.match(stringTokenType).value
	p.match(eolTokenType)

	return &selectMailbox{tag: tag, mailbox: mailbox}
}

// Create a list command
func (p *parser) list(tag string) command {
	// Get the command arguments
	reference := p.match(stringTokenType).value
	if strings.EqualFold(reference, "inbox") {
		reference = "INBOX"
	}
	mailbox := p.match(stringTokenType).value

	p.match(eolTokenType)

	return &list{tag: tag, reference: reference, mboxPattern: mailbox}
}

// Create a placeholder for an unknown command
func (p *parser) unknown(tag string, cmd string) command {
	for tok := p.lexer.next(); tok.tokType != eolTokenType; tok = p.lexer.next() {
	}
	return &unknown{tag: tag, cmd: cmd}
}

// Match the given token
func (p *parser) match(expected tokenType) *token {

	// Get the next token from the lexer
	tok := p.lexer.next()

	// Is this the expected token?
	if tok.tokType != expected {
		msg := fmt.Sprintf("Parser expected token type %v but got %v",
			expected, tok.tokType)
		err := parseError(msg)
		panic(err)
	}

	return tok
}
