package imapsrv

import (
	"fmt"
	"math"
	"strings"
)

// command represents an IMAP command
type command interface {
	// Execute the command and return an IMAP response
	execute(s *session, out chan response)
}

type commandCreator func(p *parser, tag string) command

const (
	// pathDelimiter is the delimiter used to distinguish between different folders
	// Imap lets the server choose the path delimiter
	pathDelimiter = '/'
	// A sequence number value specifying the largest sequence number in use
	largestSequenceNumber = math.MaxInt32
)

// Message flags
type messageFlag int

const (
	answered = iota
	flagged
	deleted
	seen
	draft
	recent
)

//------------------------------------------------------------------------------

// unknown is an unknown/unsupported command
type unknown struct {
	tag string
	cmd string
}

// execute reports an error for an unknown command
func (c *unknown) execute(s *session, out chan response) {
	defer close(out)

	message := fmt.Sprintf("%s unknown command", c.cmd)
	s.log(message)
	out <- bad(c.tag, message)
}

//------ Helper functions ------------------------------------------------------

// internalError logs an error and return an response
func internalError(sess *session, tag string, commandName string, err error) *finalResponse {
	message := commandName + " " + err.Error()
	sess.log(message)
	return no(tag, message).shouldClose()
}

// mustAuthenticate indicates a command is invalid because the user has not authenticated
func mustAuthenticate(sess *session, tag string, commandName string) *finalResponse {
	message := commandName + " not authenticated"
	sess.log(message)
	return bad(tag, message)
}

// pathToSlice converts a path to a slice of strings
func pathToSlice(path string) []string {

	// Split the path
	ret := strings.Split(path, string(pathDelimiter))

	if len(ret) == 0 {
		return ret
	}

	// Remove leading and trailing blanks
	if ret[0] == "" {
		if len(ret) > 1 {
			ret = ret[1:]
		} else {
			return []string{}
		}
	}

	lastIndex := len(ret) - 1
	if ret[lastIndex] == "" {
		if len(ret) > 1 {
			ret = ret[0:lastIndex]
		} else {
			return []string{}
		}
	}

	return ret

}

// joinMailboxFlags returns a string of mailbox flags for the given mailbox
func joinMailboxFlags(m *mailboxWrap) string {

	// Convert the mailbox flags into a slice of strings
	ret := make([]string, 0, 4)

	flags, _ := m.provider.Flags()

	for flag, str := range mailboxFlags {
		if flags&flag != 0 {
			ret = append(ret, str)
		}
	}

	// Return a joined string
	return strings.Join(ret, " ")

}

// joinMailboxFlags returns a string of mailbox flags for the given mailbox
func joinMailboxFlag(f MailboxFlag) string {

	// Convert the mailbox flags into a slice of strings
	ret := make([]string, 0, 4)

	for flag, str := range mailboxFlags {
		if f&flag != 0 {
			ret = append(ret, str)
		}
	}

	// Return a joined string
	return strings.Join(ret, " ")
}

// Expand a fetch macro into fetch attachments
func (c *fetch) expandMacro() {

	switch c.macro {
	case allFetchMacro:
		atts := []fetchAttachment{
			&flagsFetchAtt{},
			&internalDateFetchAtt{},
			&rfc822SizeFetchAtt{},
			&envelopeFetchAtt{},
		}
		c.attachments = atts
	case fastFetchMacro:
		atts := []fetchAttachment{
			&flagsFetchAtt{},
			&internalDateFetchAtt{},
			&rfc822SizeFetchAtt{},
		}
		c.attachments = atts
	case fullFetchMacro:
		atts := []fetchAttachment{
			&flagsFetchAtt{},
			&internalDateFetchAtt{},
			&rfc822SizeFetchAtt{},
			&envelopeFetchAtt{},
			&bodyFetchAtt{},
		}
		c.attachments = atts
	default:
		// Do no macro expansion
	}
}
