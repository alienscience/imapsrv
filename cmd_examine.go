package imapsrv

// examine gives information about a given mailbox
type examine struct {
	tag     string
	mailbox string
}

// createExamine creates an EXAMINE command
func createExamine(p *parser, tag string) command {
	// Get the mailbox name
	mailbox := p.expectString(p.lexer.astring)

	return &examine{tag: tag, mailbox: mailbox}
}

// execute manages the EXAMINE command
func (c *examine) execute(sess *session, out chan response) {
	defer close(out)

	// Is the user authenticated?
	if sess.st == notAuthenticated {
		out <- mustAuthenticate(sess, c.tag, "EXAMINE")
		return
	}

	// Select the mailbox
	mbox := pathToSlice(c.mailbox)
	exists, err := sess.selectMailbox(mbox)

	if err != nil {
		out <- internalError(sess, c.tag, "EXAMINE", err)
		return
	}

	if !exists {
		out <- no(c.tag, "EXAMINE No such mailbox")
		sess.st = authenticated
		return
	}

	res := ok(c.tag, "[READ-ONLY] EXAMINE completed")
	err = sess.addMailboxInfo(res)

	if err != nil {
		out <- internalError(sess, c.tag, "SELECT", err)
		return
	}

	sess.st = selected
	out <- res
}

func init() {
	registerCommand("examine", createExamine)
}
