package imapsrv

import (
	"log"
	"math"
	"strings"
)

// uid is a UID command
type uid struct {
	tag        string
	subcommand string
	seqRange   []sequenceRange
	rawLine    string
}

// createUid creates a UID command
func createUid(p *parser, tag string) command {
	sub := strings.ToLower(p.expectString(p.lexer.astring))

	p.lexer.skipSpace()
	seqRange := p.expectSequenceSet()

	raw := p.expectString(p.lexer.rawLine)

	return &uid{tag, sub, seqRange, raw}
}

// Execute a uid
func (c *uid) execute(sess *session, out chan response) {
	defer close(out)

	for _, seqRange := range c.seqRange {
		switch c.subcommand {
		case "fetch":
			if seqRange.end == nil {
				// single number
				// TODO:
			} else if *seqRange.end == math.MaxInt32 {
				// wildcard
				// TODO:
			} else {
				// an actual number
				// TODO:
			}
		case "copy":

		case "store":

		case "search":

		default:
			log.Println("Unknown sub-command", c.subcommand) // TODO: injection?
			out <- bad(c.tag, "unknown command: UID "+c.subcommand)
			return
		}
	}

	// TODO: remove this once the above is implemented
	log.Println("Command not implemented: UID", c.subcommand) // TODO: injection?
	out <- bad(c.tag, "command not implemented: UID "+c.subcommand)
	return
}

func init() {
	registerCommand("uid", createUid)
}
