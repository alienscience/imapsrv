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
func (c *noop) execute(s *session, out chan response) {
	defer close(out)
	out <- ok(c.tag, "NOOP Completed")
}

func init() {
	registerCommand("noop", createNoop)
}
