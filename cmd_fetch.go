package imapsrv

import "fmt"

// A FETCH command
type fetch struct {
	tag         string
	macro       fetchCommandMacro
	sequenceSet []sequenceRange
	attachments []fetchAttachment
}

// Fetch macros
type fetchCommandMacro int

const (
	noFetchMacro = iota
	allFetchMacro
	fullFetchMacro
	fastFetchMacro
)

// Sequence range, end can be nil to specify a sequence number
type sequenceRange struct {
	start int32
	end   *int32
}

// createFetch creates a FETCH command
//    fetch           = "FETCH" SP sequence-set SP ("ALL" / "FULL" / "FAST" /
//                      fetch-att / "(" fetch-att *(SP fetch-att) ")")
func createFetch(p *parser, tag string) command {
	ret := &fetch{
		tag:         tag,
		macro:       noFetchMacro,
		sequenceSet: make([]sequenceRange, 0, 4),
		attachments: make([]fetchAttachment, 0, 4),
	}

	// Get the command arguments
	// The first argument is always a sequence set
	p.lexer.skipSpace()
	ret.sequenceSet = p.expectSequenceSet()

	// The next token can be a fetch macro, a fetch attachment or an open bracket
	ok, macro := p.lexer.fetchMacro()
	if ok {
		ret.macro = macro
		return ret
	} else {
		p.lexer.pushBackToken()
	}

	isMultiple := p.lexer.leftParen()
	ret.attachments = p.expectFetchAttachments(isMultiple)

	return ret

}

// Fetch command
func (c *fetch) execute(sess *session, out chan response) {
	defer close(out)

	// Is the user authenticated?
	if sess.st == notAuthenticated {
		out <- mustAuthenticate(sess, c.tag, "FETCH")
		return
	}

	if sess.st != selected {
		out <- bad(c.tag, "Must SELECT first") // TODO: is this the correct message?
	}

	// If there is a fetch macro - convert it into fetch attachments
	c.expandMacro()

	// Loop through the sequence ranges to fetch
	for _, seqRange := range c.sequenceSet {

		// Loop through the sequence range
		i := seqRange.start
		for {
			// Add the start of the response
			resp := partial()
			resp.put(fmt.Sprint(i, " FETCH"))

			// Execute the fetch command
			err := sess.fetch(resp, i, c.attachments)

			if err != nil {
				out <- internalError(sess, c.tag, "FETCH", err)
				return
			}

			// Output the current fetch
			out <- resp

			// Is this the last value in the range?
			i += 1
			if seqRange.end == nil || i > *seqRange.end {
				break
			}
		}
	}

	out <- ok(c.tag, "FETCH completed")
}

func init() {
	registerCommand("fetch", createFetch)
}
