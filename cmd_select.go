package imapsrv

// selectMailbox is a SELECT command
type selectMailbox struct {
	tag     string
	mailbox string
}

// createSelect creates a SELECT command
func createSelect(p *parser, tag string) command {
	// Get the mailbox name
	mailbox := p.ExpectString(p.lexer.astring)

	return &selectMailbox{tag: tag, mailbox: mailbox}
}

// execute a SELECT command
func (c *selectMailbox) execute(sess *session, out chan response) {
	defer close(out)

	// Is the user authenticated?
	if sess.st == notAuthenticated {
		out <- mustAuthenticate(sess, c.tag, "SELECT")
		return
	}

	// Select the mailbox
	mbox := pathToSlice(c.mailbox)
	exists, err := sess.selectMailbox(mbox)

	if err != nil {
		out <- internalError(sess, c.tag, "SELECT", err)
		return
	}

	if !exists {
		out <- no(c.tag, "SELECT No such mailbox")
		sess.st = authenticated
		return
	}

	// Build a response that includes mailbox information
	res := ok(c.tag, "SELECT completed")

	err = sess.addMailboxInfo(res)

	if err != nil {
		out <- internalError(sess, c.tag, "SELECT", err)
		return
	}

	sess.st = selected
	out <- res
}

func init() {
	registerCommand("select", createSelect)
}
