package imapsrv

import (
	"fmt"
	"strings"
)

// list is a LIST command
type list struct {
	tag         string
	reference   string // Context of mailbox name
	mboxPattern string // The mailbox name pattern
}

// createList creates a LIST command
//    list            = "LIST" SP mailbox SP list-mailbox
func createList(p *parser, tag string) command {

	// Get the command arguments
	reference := p.ExpectString(p.lexer.astring)

	if strings.EqualFold(reference, "inbox") {
		reference = "INBOX"
	}
	mailbox := p.ExpectString(p.lexer.listMailbox)

	return &list{tag: tag, reference: reference, mboxPattern: mailbox}
}

// execute a LIST command
func (c *list) execute(sess *session, out chan response) {
	defer close(out)

	// Is the user authenticated?
	if sess.st == notAuthenticated {
		out <- mustAuthenticate(sess, c.tag, "LIST")
		return
	}

	// Is the mailbox pattern empty? This indicates that we should return
	// the delimiter and the root name of the reference
	if c.mboxPattern == "" {
		if len(c.reference) == 0 {
			c.reference = "\"\""
		}
		res := ok(c.tag, "LIST completed")
		res.putLine(fmt.Sprintf(`LIST () "%s" %s`, string(pathDelimiter), c.reference))
		out <- res
		return
	}

	// Convert the reference and mbox pattern into slices
	ref := pathToSlice(c.reference)
	mbox := pathToSlice(c.mboxPattern)

	// Get the list of mailboxes
	mboxes, err := sess.list(ref, mbox)

	if err != nil {
		out <- internalError(sess, c.tag, "LIST", err)
		return
	}

	// Respond with the mailboxes
	res := ok(c.tag, "LIST completed")
	for _, mbox := range mboxes {
		res.putLine(fmt.Sprintf(`LIST (%s) "%s" "%s"`,
			joinMailboxFlags(mbox),
			string(pathDelimiter),
			strings.Join(mbox.provider.Path(), string(pathDelimiter))))
	}

	out <- res
}

func init() {
	registerCommand("list", createList)
}
