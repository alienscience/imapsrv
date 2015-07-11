package imapsrv

// logout is a LOGOUT command
type logout struct {
	tag string
}

// createLogout creates a LOGOUT command
func createLogout(p *parser, tag string) command {
	return &logout{tag: tag}
}

// execute a LOGOUT command
func (c *logout) execute(sess *session, out chan response) {
	defer close(out)

	sess.st = notAuthenticated
	sess.user = ""
	out <- ok(c.tag, "LOGOUT completed").
		shouldClose().
		putLine("BYE IMAP4rev1 Server logging out")
}

func init() {
	registerCommand("logout", createLogout)
}
