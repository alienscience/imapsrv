package imapsrv

type check struct {
	tag string
}

func createCheck(_ *parser, tag string) command {
	return &check{tag}
}

func (c *check) execute(sess *session, out chan response) {
	defer close(out)

	if sess.st != selected {
		out <- bad(c.tag, "CHECK requires SELECTED state")
		return
	}

	sess.mailbox.provider.Checkpoint()

	out <- ok(c.tag, "CHECK Completed")
}

func init() {
	registerCommand("check", createCheck)
}
