package imapsrv

import (
	"fmt"
	"strings"
)

// create represents the CREATE command
type create struct {
	tag     string
	mailbox string
}

// createCreate creates an CREATE command
func createCreate(p *parser, tag string) command {
	// Get the mailbox name
	mailbox := p.expectString(p.lexer.astring)

	return &create{tag: tag, mailbox: mailbox}
}

// execute handles the CREATE command
func (c *create) execute(sess *session, out chan response) {
	defer close(out)

	err := sess.config.Mailstore.NewMailbox(sess.user, strings.Split(c.mailbox, string(pathDelimiter)))
	if err != nil {
		if _, ok := err.(CreateError); ok {
			out <- no(c.tag, "create failure: can't create mailbox with that name")
			return
		} else {
			out <- bad(c.tag, "Unknown error creating mailbox")
			return
		}
	}

	out <- ok(c.tag, "CREATE completed")
}

type CreateError struct {
	MailboxPath []string
}

func (c CreateError) Error() string {
	return fmt.Sprintf("could not create mailbox: %s", strings.Join(c.MailboxPath, string(pathDelimiter)))
}

func init() {
	registerCommand("create", createCreate)
}
