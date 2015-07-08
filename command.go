package imapsrv

import (
	"crypto/tls"
	"fmt"
	"log"
	"math"
	"net/textproto"
	"strings"
)

// command represents an IMAP command
type command interface {
	// Execute the command and return an IMAP response
	execute(s *session, out chan response)
}

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

// noop is a NOOP command
type noop struct {
	tag string
}

// Execute a noop
func (c *noop) execute(s *session, out chan response) {
	defer close(out)
	out <- ok(c.tag, "NOOP Completed")
}

//------------------------------------------------------------------------------

// capability is a CAPABILITY command
type capability struct {
	tag string
}

// execute a capability
func (c *capability) execute(s *session, out chan response) {
	defer close(out)
	var commands []string

	if s.st == notAuthenticated {
		switch s.listener.encryption {
		case unencryptedLevel:
		// TODO: do we want to support this?

		case starttlsLevel:
			if s.encryption == tlsLevel {
				commands = append(commands, "AUTH=PLAIN")
			} else {
				commands = append(commands, "STARTTLS")
				commands = append(commands, "LOGINDISABLED")
			}

		case tlsLevel:
			commands = append(commands, "AUTH=PLAIN")
		}
	} else {
		// Things that are supported after authenticating
		// commands = append(commands, "CHILDREN")
	}

	// Return all capabilities
	out <- ok(c.tag, "CAPABILITY completed").
		putLine("CAPABILITY IMAP4rev1 " + strings.Join(commands, " "))
}

//------------------------------------------------------------------------------

type starttls struct {
	tag string
}

func (c *starttls) execute(sess *session, out chan response) {
	defer close(out)

	sess.conn.Write([]byte(fmt.Sprintf("%s Begin TLS negotiation now", c.tag)))

	sess.conn = tls.Server(sess.conn, &tls.Config{Certificates: sess.listener.certificates})
	textConn := textproto.NewConn(sess.conn)

	sess.encryption = tlsLevel
	out <- empty().shouldReplaceBuffers(textConn)
}

//------------------------------------------------------------------------------

// login is a LOGIN command
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

	auth, err := sess.server.config.AuthBackend.Authenticate(c.userId, c.password)
	if auth {
		sess.st = authenticated
		sess.user = c.userId
		out <- ok(c.tag, "LOGIN completed")
		return
	}
	log.Println("Login request:", auth, err)

	// Fail by default
	out <- no(c.tag, "LOGIN failure")
}

//------------------------------------------------------------------------------

// logout is a LOGOUT command
type logout struct {
	tag string
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

//------------------------------------------------------------------------------

// selectMailbox is a SELECT command
type selectMailbox struct {
	tag     string
	mailbox string
}

// execute a SELECT command
func (c *selectMailbox) execute(sess *session, out chan response) {
	defer close(out)

	// Is the user authenticated?
	if sess.st == notAuthenticated {
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
		sess.st = authenticated
		return
	}

	// Build a response that includes mailbox information
	res := ok(c.tag, "SELECT completed")

	err = sess.addMailboxInfo(res)

	if err != nil {
		out <- internalError(sess, c.tag, "SELECT", err)
		return
	}

	sess.st = selected
	out <- res
}

//------------------------------------------------------------------------------

// delete is a DELETE command
type delete struct {
	tag     string
	mailbox string
}

func (c *delete) execute(sess *session, out chan response) {
	defer close(out)

	if strings.EqualFold(c.mailbox, "INBOX") {
		out <- no(c.tag, "cannot delete INBOX")
		return
	}

	err := sess.config.Mailstore.DeleteMailbox(sess.user, strings.Split(c.mailbox, string(pathDelimiter)))
	if err != nil {
		if _, ok := err.(DeleteError); ok {
			out <- no(c.tag, "delete failure: can't delete mailbox with that name")
			return
		} else {
			out <- bad(c.tag, "unknown error occured")
			return
		}
	}

	out <- ok(c.tag, "DELETE Completed")
}

type DeleteError struct {
	MailboxPath []string
}

func (e DeleteError) Error() string {
	return fmt.Sprintf("cannot delete; mailbox does not exist: %s", strings.Join(e.MailboxPath, string(pathDelimiter)))
}

//------------------------------------------------------------------------------

// list is a LIST command
type list struct {
	tag         string
	reference   string // Context of mailbox name
	mboxPattern string // The mailbox name pattern
}

// execute a LIST command
func (c *list) execute(sess *session, out chan response) {
	defer close(out)

	// Is the user authenticated?
	if sess.st == notAuthenticated {
		out <- mustAuthenticate(sess, c.tag, "LIST")
		return
	}

	// Is the mailbox pattern empty? This indicates that we should return
	// the delimiter and the root name of the reference
	if c.mboxPattern == "" {
		if len(c.reference) == 0 {
			c.reference = "\"\""
		}
		res := ok(c.tag, "LIST completed")
		res.putLine(fmt.Sprintf(`LIST () "%s" %s`, string(pathDelimiter), c.reference))
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

	// Respond with the mailboxes
	res := ok(c.tag, "LIST completed")
	for _, mbox := range mboxes {
		res.putLine(fmt.Sprintf(`LIST (%s) "%s" "%s"`,
			joinMailboxFlags(mbox),
			string(pathDelimiter),
			strings.Join(mbox.provider.Path(), string(pathDelimiter))))
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

//------------------------------------------------------------------------------

// examine gives information about a given mailbox
type examine struct {
	tag     string
	mailbox string
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

//------------------------------------------------------------------------------

// create represents the CREATE command
type create struct {
	tag     string
	mailbox string
}

// execute handles the CREATE command
func (c *create) execute(sess *session, out chan response) {
	defer close(out)

	err := sess.config.Mailstore.NewMailbox(sess.user, strings.Split(c.mailbox, string(pathDelimiter)))
	if err != nil {
		if _, ok := err.(CreateError); ok {
			out <- no(c.tag, "create failure: can't create mailbox with that name")
			return
		} else {
			out <- bad(c.tag, "Unknown error creating mailbox")
			return
		}
	}

	out <- ok(c.tag, "CREATE completed")
}

type CreateError struct {
	MailboxPath []string
}

func (c CreateError) Error() string {
	return fmt.Sprintf("could not create mailbox: %s", strings.Join(c.MailboxPath, string(pathDelimiter)))
}

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
