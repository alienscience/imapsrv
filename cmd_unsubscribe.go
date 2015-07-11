package imapsrv

import (
	"log"
	"strings"
)

// unsubscribe is a UNSUBSCRIBE command
type unsubscribe struct {
	tag     string
	mailbox string
}

// createUnsubscribe creates a UNSUBSCRIBE command
func createUnsubscribe(p *parser, tag string) command {
	mailbox := p.expectString(p.lexer.astring)
	return &unsubscribe{tag, mailbox}
}

// Execute a UNSUBSCRIBE
func (c *unsubscribe) execute(sess *session, out chan response) {
	defer close(out)

	// Is the user authenticated?
	if sess.st == notAuthenticated {
		out <- mustAuthenticate(sess, c.tag, "UNSUBSCRIBE")
		return
	}

	// Check if mailbox exists
	box, err := sess.config.Mailstore.Mailbox(sess.user, strings.Split(c.mailbox, string(pathDelimiter)))
	if err != nil {
		out <- no(c.tag, "UNSUBSCRIBE failed; mailbox not found")
		return
	}
	if err = box.Unsubscribe(); err != nil {
		log.Println("Unsubscription error", err)
		out <- no(c.tag, "UNSUBSCRIBE failed")
		return
	}

	out <- ok(c.tag, "UNSUBSCRIBE Completed")
}

func init() {
	registerCommand("unsubscribe", createUnsubscribe)
}
