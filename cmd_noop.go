package imapsrv

// noop is a NOOP command
type noop struct {
	tag string
}

// createNoop creates a NOOP command
func createNoop(p *parser, tag string) command {
	return &noop{tag: tag}
}

// Execute a noop
func (c *noop) execute(sess *session, out chan response) {
	defer close(out)

	res := ok(c.tag, "NOOP Completed")

	if sess.st == selected {
		// TODO: send recent updates as untagged response
		err := sess.addMailboxInfo(res)
		if err != nil {
			out <- no(c.tag, "NOOP failed")
		}
	}

	out <- res
}

func init() {
	registerCommand("noop", createNoop)
}
