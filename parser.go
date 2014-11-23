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
	tagToken := p.match(stringTokenType, asTag)
	commandToken := p.match(stringTokenType, asAString)

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
	p.matchEOL()
	return &noop{tag: tag}
}

// Create a capability command
func (p *parser) capability(tag string) command {
	p.matchEOL()
	return &capability{tag: tag}
}

// Create a login command
func (p *parser) login(tag string) command {

	// Get the command arguments
	userId := p.match(stringTokenType, asAString).value
	password := p.match(stringTokenType, asAString).value
	p.matchEOL()

	// Create the command
	return &login{tag: tag, userId: userId, password: password}
}

// Create a logout command
func (p *parser) logout(tag string) command {
	p.matchEOL()
	return &logout{tag: tag}
}

// Create a select command
func (p *parser) selectC(tag string) command {
	// Get the mailbox name
	mailbox := p.match(stringTokenType, asAString).value
	p.matchEOL()

	return &selectMailbox{tag: tag, mailbox: mailbox}
}

// Create a list command
func (p *parser) list(tag string) command {
	// Get the command arguments
	reference := p.match(stringTokenType, asAString).value
	if strings.EqualFold(reference, "inbox") {
		reference = "INBOX"
	}
	mailbox := p.match(stringTokenType, asListMailbox).value

	p.matchEOL()

	return &list{tag: tag, reference: reference, mboxPattern: mailbox}
}

// Create a placeholder for an unknown command
func (p *parser) unknown(tag string, cmd string) command {
	for tok := p.lexer.next(asAny); tok.tokType != eolTokenType; tok = p.lexer.next(asAny) {
	}
	return &unknown{tag: tag, cmd: cmd}
}

// Match the given token
func (p *parser) match(expected tokenType, lexAs unquotedLexerFlag) *token {

	// Get the next token from the lexer
	tok := p.lexer.next(lexAs)

	// Is this the expected token?
	if tok.tokType != expected {
		msg := fmt.Sprintf("Parser expected token type %v but got %v",
			expected, tok.tokType)
		err := parseError(msg)
		panic(err)
	}

	return tok
}

// Match end of line
func (p *parser) matchEOL() {
	p.match(eolTokenType, asAny)
}
