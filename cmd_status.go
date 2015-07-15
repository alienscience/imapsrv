package imapsrv

// status gives information about a given mailbox
type status struct {
	tag     string
	mailbox string
}

// createStatus creates an STATUS command
func createStatus(p *parser, tag string) command {
	// Get the mailbox name
	mailbox := p.expectString(p.lexer.astring)

	return &status{tag: tag, mailbox: mailbox}
}

// status manages the STATUS command
func (c *status) execute(sess *session, out chan response) {
	defer close(out)

	// Is the user authenticated?
	if sess.st == notAuthenticated {
		out <- mustAuthenticate(sess, c.tag, "STATUS")
		return
	}

	// Select the mailbox
	mbox := pathToSlice(c.mailbox)
	exists, err := sess.selectMailbox(mbox)

	if err != nil {
		out <- internalError(sess, c.tag, "STATUS", err)
		return
	}

	if !exists {
		out <- no(c.tag, "STATUS No such mailbox")
		sess.st = authenticated
		return
	}

	res := ok(c.tag, "STATUS completed")
	err = sess.addMailboxInfo(res)

	if err != nil {
		out <- internalError(sess, c.tag, "SELECT", err)
		return
	}

	out <- res
}

func init() {
	registerCommand("status", createStatus)
}
