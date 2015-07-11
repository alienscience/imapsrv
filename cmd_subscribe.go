package imapsrv

import (
	"log"
	"strings"
)

// subscribe is a SUBSCRIBE command
type subscribe struct {
	tag     string
	mailbox string
}

// createSubscribe creates a SUBSCRIBE command
func createSubscribe(p *parser, tag string) command {
	mailbox := p.expectString(p.lexer.astring)
	return &subscribe{tag, mailbox}
}

// Execute a SUBSCRIBE
func (c *subscribe) execute(sess *session, out chan response) {
	defer close(out)

	// Is the user authenticated?
	if sess.st == notAuthenticated {
		out <- mustAuthenticate(sess, c.tag, "SUBSCRIBE")
		return
	}

	// Check if mailbox exists
	box, err := sess.config.Mailstore.Mailbox(sess.user, strings.Split(c.mailbox, string(pathDelimiter)))
	if err != nil {
		out <- no(c.tag, "SUBSCRIBE failed; mailbox not found")
		return
	}
	if err = box.Subscribe(); err != nil {
		log.Println("Subscription error", err)
		out <- no(c.tag, "SUBSCRIBE failed")
		return
	}

	out <- ok(c.tag, "SUBSCRIBE Completed")
}

func init() {
	registerCommand("subscribe", createSubscribe)
}
