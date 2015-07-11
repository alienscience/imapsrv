package imapsrv

import (
	"fmt"
	"strings"
)

// delete is a DELETE command
type delete struct {
	tag     string
	mailbox string
}

// createDelete creates a DELETE command
func createDelete(p *parser, tag string) command {
	// Get the mailbox name
	mailbox := p.expectString(p.lexer.astring)

	return &delete{tag: tag, mailbox: mailbox}
}

func (c *delete) execute(sess *session, out chan response) {
	defer close(out)

	if strings.EqualFold(c.mailbox, "INBOX") {
		out <- no(c.tag, "cannot delete INBOX")
		return
	}

	err := sess.config.Mailstore.DeleteMailbox(sess.user, strings.Split(c.mailbox, string(pathDelimiter)))
	if err != nil {
		if _, ok := err.(DeleteError); ok {
			out <- no(c.tag, "delete failure: can't delete mailbox with that name")
			return
		} else {
			out <- bad(c.tag, "unknown error occured")
			return
		}
	}

	out <- ok(c.tag, "DELETE Completed")
}

type DeleteError struct {
	MailboxPath []string
}

func (e DeleteError) Error() string {
	return fmt.Sprintf("cannot delete; mailbox does not exist: %s", strings.Join(e.MailboxPath, string(pathDelimiter)))
}

func init() {
	registerCommand("delete", createDelete)
}
