package imapsrv

import (
	"fmt"
	"strings"
)

// command represents an IMAP command
type command interface {
	// Execute the command and return an IMAP response
	execute(s *session) *response
}

const (
	// pathDelimiter is the delimiter used to distinguish between different folders
	pathDelimiter = '/'
)

//------------------------------------------------------------------------------

// noop is a NOOP command
type noop struct {
	tag string
}

// execute a NOOP command
func (c *noop) execute(s *session) *response {
	return ok(c.tag, "NOOP Completed")
}

//------------------------------------------------------------------------------

// capability is a CAPABILITY command
type capability struct {
	tag string
}

// execute a capability
func (c *capability) execute(s *session) *response {
	// The IMAP server is assumed to be running over SSL and so
	// STARTTLS is not supported and LOGIN is not disabled
	return ok(c.tag, "CAPABILITY completed").
		extra("CAPABILITY IMAP4rev1")
}

//------------------------------------------------------------------------------

// login is a LOGIN command
type login struct {
	tag      string
	userId   string
	password string
}

// execute a LOGIN command
func (c *login) execute(sess *session) *response {

	// Has the user already logged in?
	if sess.st != notAuthenticated {
		message := "LOGIN already logged in"
		sess.log(message)
		return bad(c.tag, message)
	}

	// TODO: implement login
	if c.userId == "test" {
		sess.st = authenticated
		return ok(c.tag, "LOGIN completed")
	}

	// Fail by default
	return no(c.tag, "LOGIN failure")
}

//------------------------------------------------------------------------------

// logout is a LOGOUT command
type logout struct {
	tag string
}

// execute a LOGOUT command
func (c *logout) execute(sess *session) *response {

	sess.st = notAuthenticated
	return ok(c.tag, "LOGOUT completed").
		extra("BYE IMAP4rev1 Server logging out").
		shouldClose()
}

//------------------------------------------------------------------------------

// selectMailbox is a SELECT command
type selectMailbox struct {
	tag     string
	mailbox string
}

// execute a SELECT command
func (c *selectMailbox) execute(sess *session) *response {

	// Is the user authenticated?
	if sess.st != authenticated {
		return mustAuthenticate(sess, c.tag, "SELECT")
	}

	// Select the mailbox
	mbox := pathToSlice(c.mailbox)
	exists, err := sess.selectMailbox(mbox)

	if err != nil {
		return internalError(sess, c.tag, "SELECT", err)
	}

	if !exists {
		return no(c.tag, "SELECT No such mailbox")
	}

	// Build a response that includes mailbox information
	res := ok(c.tag, "SELECT completed")

	err = sess.addMailboxInfo(res)

	if err != nil {
		return internalError(sess, c.tag, "SELECT", err)
	}

	return res
}

//------------------------------------------------------------------------------

// list is a LIST command
type list struct {
	tag         string
	reference   string // Context of mailbox name
	mboxPattern string // The mailbox name pattern
}

// execute a LIST command
func (c *list) execute(sess *session) *response {

	// Is the user authenticated?
	if sess.st != authenticated {
		return mustAuthenticate(sess, c.tag, "LIST")
	}

	// Is the mailbox pattern empty? This indicates that we should return
	// the delimiter and the root name of the reference
	if c.mboxPattern == "" {
		res := ok(c.tag, "LIST completed")
		res.extra(fmt.Sprintf(`LIST () "%s" %s`, pathDelimiter, c.reference))
		return res
	}

	// Convert the reference and mbox pattern into slices
	ref := pathToSlice(c.reference)
	mbox := pathToSlice(c.mboxPattern)

	// Get the list of mailboxes
	mboxes, err := sess.list(ref, mbox)

	if err != nil {
		return internalError(sess, c.tag, "LIST", err)
	}

	// Check for an empty response
	if len(mboxes) == 0 {
		return no(c.tag, "LIST no results")
	}

	// Respond with the mailboxes
	res := ok(c.tag, "LIST completed")
	for _, mbox := range mboxes {
		res.extra(fmt.Sprintf(`LIST (%s) "%s" /%s`,
			joinMailboxFlags(mbox),
			string(pathDelimiter),
			strings.Join(mbox.Path, string(pathDelimiter))))
	}

	return res
}

//------------------------------------------------------------------------------

// unknown is an unknown/unsupported command
type unknown struct {
	tag string
	cmd string
}

// execute reports an error for an unknown command
func (c *unknown) execute(s *session) *response {
	message := fmt.Sprintf("%s unknown command", c.cmd)
	s.log(message)
	return bad(c.tag, message)
}

//------ Helper functions ------------------------------------------------------

// internalError logs an error and return an response
func internalError(sess *session, tag string, commandName string, err error) *response {
	message := commandName + " " + err.Error()
	sess.log(message)
	return no(tag, message).shouldClose()
}

// mustAuthenticate indicates a command is invalid because the user has not authenticated
func mustAuthenticate(sess *session, tag string, commandName string) *response {
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
