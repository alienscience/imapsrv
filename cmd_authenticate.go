package imapsrv

import "strings"

type authenticate struct {
	tag       string
	mechanism string
	parser    *parser
}

func createAuthenticate(p *parser, tag string) command {
	mechanism := p.expectString(p.lexer.astring)

	return &authenticate{tag, strings.ToLower(mechanism), p}
}

func (c *authenticate) execute(sess *session, out chan response) {
	defer close(out)

	switch c.mechanism {
	// TODO: implement PLAIN?
	// case: "plain":

	default:
		out <- no(c.tag, "Mechanism not supported")
	}

	return
}

func init() {
	registerCommand("authenticate", createAuthenticate)
}
