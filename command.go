package imapsrv

import (
	"fmt"
	"log"
	"math"
	"strings"
)

// An IMAP command
type command interface {
	// Execute a command and receive imap responses on the given channel
	// The channel is closed by the imap command when it completes
	execute(s *session, out chan response)
}

// Path delimiter
const (
	pathDelimiter = '/'
)

//------------------------------------------------------------------------------

type noop struct {
	tag string
}

// Execute a noop
func (c *noop) execute(s *session, out chan response) {
	defer close(out)
	out <- ok(c.tag, "NOOP Completed")
}

//------------------------------------------------------------------------------

// A CAPABILITY command
type capability struct {
	tag string
}

// Execute a capability
func (c *capability) execute(s *session, out chan response) {
	defer close(out)

	// At the moment the IMAP server is assumed to be running over SSL and so
	// STARTTLS is not supported and LOGIN is not disabled
	out <- ok(c.tag, "CAPABILITY completed").
		put("CAPABILITY IMAP4rev1")
}

//------------------------------------------------------------------------------

// A LOGIN command
type login struct {
	tag      string
	userId   string
	password string
}

// Login command
func (c *login) execute(sess *session, out chan response) {
	defer close(out)

	// Has the user already logged in?
	if sess.st != notAuthenticated {
		message := "LOGIN already logged in"
		sess.log(message)
		out <- bad(c.tag, message)
		return
	}

	// TODO: implement login
	if c.userId == "test" {
		sess.st = authenticated
		out <- ok(c.tag, "LOGIN completed")
		return
	}

	// Fail by default
	out <- no(c.tag, "LOGIN failure")
}

//------------------------------------------------------------------------------

// A LOGOUT command
type logout struct {
	tag string
}

// Logout command
func (c *logout) execute(sess *session, out chan response) {
	defer close(out)

	sess.st = notAuthenticated
	out <- ok(c.tag, "LOGOUT completed").
		shouldClose().
		put("BYE IMAP4rev1 Server logging out")
}

//------------------------------------------------------------------------------

// A SELECT command
type selectMailbox struct {
	tag     string
	mailbox string
}

// Select command
func (c *selectMailbox) execute(sess *session, out chan response) {
	defer close(out)

	// Is the user authenticated?
	if sess.st != authenticated {
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
		return
	}

	// Build a response that includes mailbox information
	res := ok(c.tag, "SELECT completed")

	err = sess.addMailboxInfo(res)

	if err != nil {
		out <- internalError(sess, c.tag, "SELECT", err)
		return
	}
		
	out <- res
}

//------------------------------------------------------------------------------

// A LIST command
type list struct {
	tag         string
	reference   string // Context of mailbox name
	mboxPattern string // The mailbox name pattern
}

// List command
func (c *list) execute(sess *session, out chan response) {
	defer close(out)

	// Is the user authenticated?
	if sess.st != authenticated {
		out <- mustAuthenticate(sess, c.tag, "LIST")
		return
	}

	// Is the mailbox pattern empty? This indicates that we should return
	// the delimiter and the root name of the reference
	if c.mboxPattern == "" {
		res := ok(c.tag, "LIST completed")
		res.put(fmt.Sprintf(`LIST () "%s" %s`, pathDelimiter, c.reference))
		out <- res
		return
	}

	// Convert the reference and mbox pattern into slices
	ref := pathToSlice(c.reference)
	mbox := pathToSlice(c.mboxPattern)

	// Get the list of mailboxes
	mboxes, err := sess.list(ref, mbox)

	if err != nil {
		out <- internalError(sess, c.tag, "LIST", err)
		return
	}

	// Check for an empty response
	if len(mboxes) == 0 {
		out <- no(c.tag, "LIST no results")
		return
	}

	// Respond with the mailboxes
	res := ok(c.tag, "LIST completed")
	for _, mbox := range mboxes {
		res.put(fmt.Sprintf(`LIST (%s) "%s" /%s`,
			joinMailboxFlags(mbox),
			string(pathDelimiter),
			strings.Join(mbox.Path, string(pathDelimiter))))
	}

	out <- res
}

//------------------------------------------------------------------------------

// A FETCH command
type fetch struct {
	tag         string
	macro       fetchCommandMacro
	sequenceSet []sequenceRange
	attachments []fetchAttachment
}

// Fetch attachments
type fetchAttachment struct {
	// What to fetch
	id fetchAttachmentId
	// nil if no fetchSection exists
	section *fetchSection
}

type fetchAttachmentId int

const (
	invalidFetchAtt = iota
	envelopeFetchAtt
	flagsFetchAtt
	internalDateFetchAtt
	rfc822HeaderFetchAtt
	rfc822SizeFetchAtt
	rfc822TextFetchAtt
	bodyFetchAtt
	bodyStructureFetchAtt
	uidFetchAtt
	bodySectionFetchAtt
	bodyPeekFetchAtt
)

// Fetch macros
type fetchCommandMacro int

const (
	noFetchMacro = iota
	allFetchMacro
	fullFetchMacro
	fastFetchMacro
)

// The section of fetch attachment
type fetchSection struct {
	section []uint32
	part    partSpecifier
	fields  []string
	// nil if no fetchPartial exists
	partial *fetchPartial
}

type partSpecifier int

const (
	invalidPart = iota
	headerPart
	headerFieldsPart
	headerFieldsNotPart
	textPart
	mimePart
)

// A byte range
type fetchPartial struct {
	fromOctet int32
	toOctet   uint32
}

// Sequence range, end can be nil to specify a sequence number
type sequenceRange struct {
	start uint32
	end   *uint32
}

// A sequence number value specifying the largest sequence number in use
const largestSequenceNumber = math.MaxUint32

// Creating a fetch command requires a constructor
func createFetchCommand(tag string) *fetch {
	return &fetch{
		tag:         tag,
		macro:       noFetchMacro,
		sequenceSet: make([]sequenceRange, 0, 4),
		attachments: make([]fetchAttachment, 0, 4),
	}
}

// Fetch command
func (c *fetch) execute(sess *session, out chan response) {
	defer close(out)

	// Is the user authenticated?
	if sess.st != authenticated {
		out <- mustAuthenticate(sess, c.tag, "FETCH")
		return
	}

	// TODO: remove this debug code
	log.Print("FETCH ", c.macro, c.sequenceSet)
	for _, att := range c.attachments {
		log.Print("  ", att)
		if att.section != nil {
			log.Print("    Section:", att.section)
		}
	}

	// If there is a fetch macro - convert it into fetch attachments
	c.expandMacro()

	// Loop through the sequence ranges to fetch
	for _, seqRange := range c.sequenceSet {

		// Loop through the sequence range
		i := seqRange.start
		for {
			// Execute the fetch command
			res, err := sess.fetch(i, c.attachments)

			if err != nil {
				out <- internalError(sess, c.tag, "FETCH", err)
				return
			}

			// Output the current fetch
			out <- partialFetchResponse(res)

			// Is this the last value in the range?
			if seqRange.end == nil || i > *seqRange.end {
				break
			}
		}
	}

	out <- ok(c.tag, "FETCH completed")
}

//------------------------------------------------------------------------------

// An unknown/unsupported command
type unknown struct {
	tag string
	cmd string
}

// Report an error for an unknown command
func (c *unknown) execute(s *session, out chan response) {
	defer close(out)

	message := fmt.Sprintf("%s unknown command", c.cmd)
	s.log(message)
	out <- bad(c.tag, message)
}

//------ Helper functions ------------------------------------------------------

// Log an error and return an response
func internalError(sess *session, tag string, commandName string, err error) *finalResponse {
	message := commandName + " " + err.Error()
	sess.log(message)
	return no(tag, message).shouldClose()
}

// Indicate a command is invalid because the user has not authenticated
func mustAuthenticate(sess *session, tag string, commandName string) *finalResponse {
	message := commandName + " not authenticated"
	sess.log(message)
	return bad(tag, message)
}

// Convert a path to a slice of strings
func pathToSlice(path string) []string {

	// Split the path
	ret := strings.Split(path, string(pathDelimiter))

	if len(ret) == 0 {
		return ret
	}

	// Remove leading and trailing blanks
	if ret[0] == "" {
		if len(ret) > 1 {
			ret = ret[1:len(ret)]
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

// Return a string of mailbox flags for the given mailbox
func joinMailboxFlags(m *Mailbox) string {

	// Convert the mailbox flags into a slice of strings
	flags := make([]string, 0, 4)

	for flag, str := range mailboxFlags {
		if m.Flags&flag != 0 {
			flags = append(flags, str)
		}
	}

	// Return a joined string
	return strings.Join(flags, ",")
}

// Expand a fetch macro into fetch attachments
func (c *fetch) expandMacro() {

	switch c.macro {
	case allFetchMacro:
		atts := []fetchAttachment{
			fetchAttachment{id: flagsFetchAtt},
			fetchAttachment{id: internalDateFetchAtt},
			fetchAttachment{id: rfc822SizeFetchAtt},
			fetchAttachment{id: envelopeFetchAtt},
		}
		c.attachments = atts
	case fastFetchMacro:
		atts := []fetchAttachment{
			fetchAttachment{id: flagsFetchAtt},
			fetchAttachment{id: internalDateFetchAtt},
			fetchAttachment{id: rfc822SizeFetchAtt},
		}
		c.attachments = atts
	case fullFetchMacro:
		atts := []fetchAttachment{
			fetchAttachment{id: flagsFetchAtt},
			fetchAttachment{id: internalDateFetchAtt},
			fetchAttachment{id: rfc822SizeFetchAtt},
			fetchAttachment{id: envelopeFetchAtt},
			fetchAttachment{id: bodyFetchAtt},
		}
		c.attachments = atts
	default:
		// Do no macro expansion
	}
}
