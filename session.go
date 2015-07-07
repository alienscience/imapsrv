package imapsrv

import (
	"fmt"
	"log"
	"net"
)

// state is the IMAP session state
type state int

const (
	notAuthenticated state = iota
	authenticated
	selected
)

type encryptionLevel int

const (
	// unencryptedLevel indicates an unencrypted / cleartext connection
	unencryptedLevel encryptionLevel = iota
	// starttlsLevel indicates that an unencrypted connection can be used to start a TLS connection
	starttlsLevel
	// tlsLevel indicates that a secure TLS connection must be set first
	tlsLevel
)

// session represents an IMAP session
type session struct {
	// id is a unique identifier for this session
	id string
	// st indicates the current state of the session
	st state
	// config refers to the IMAP configuration
	config *Config
	// server refers to the server the session is at
	server *Server
	// listener is the listener that's handling this current session
	listener *listener
	// conn is the currently active TCP connection
	conn net.Conn
	// tls indicates whether or not the communication is encrypted
	encryption encryptionLevel
	// mailbox is the currently selected mailbox (if st == selected)
	mailbox *mailboxWrap
	// user is the currently authenticated user
	user string
}

// Create a new IMAP session
func createSession(id string, config *Config, server *Server, listener *listener, conn net.Conn) *session {
	return &session{
		id:       id,
		st:       notAuthenticated,
		config:   config,
		server:   server,
		listener: listener,
		conn:     conn,
	}
}

// log writes the info messages to the logger with session information
func (s *session) log(info ...interface{}) {
	preamble := fmt.Sprintf("IMAP (%s) ", s.id)
	message := []interface{}{preamble}
	message = append(message, info...)
	log.Print(message...)
}

// selectMailbox selects a mailbox - returns true if the mailbox exists
func (s *session) selectMailbox(path []string) (bool, error) {
	// Lookup the mailbox
	mailstore := s.config.Mailstore
	mbox, err := getMailbox(mailstore, s.user, path)

	if err != nil {
		return false, err
	}

	if mbox == nil {
		return false, nil
	}

	// Make note of the mailbox
	s.mailbox = mbox
	return true, nil
}

// list mailboxes matching the given mailbox pattern
func (s *session) list(reference []string, pattern []string) ([]*mailboxWrap, error) {

	ret := make([]*mailboxWrap, 0, 4)
	path := copySlice(reference)

	// Build a path that does not have wildcards
	wildcard := -1
	for i, dir := range pattern {
		if dir == "%" || dir == "*" {
			wildcard = i
			break
		}
		path = append(path, dir)
	}

	// Just return a single mailbox if there are no wildcards
	if wildcard == -1 {
		mbox, err := getMailbox(s.config.Mailstore, s.user, path)
		if err != nil {
			return ret, err
		}
		ret = append(ret, mbox)
		return ret, nil
	}

	// Recursively get a listing
	return s.depthFirstMailboxes(ret, path, pattern[wildcard:])
}

// Fetch a mail message with the given sequence number
func (s *session) fetch(
	resp *partialResponse,
	seqnum int32,
	attachments []fetchAttachment) error {

	// Fetch the message
	mailbox := s.mailbox
	msg, err := mailbox.fetch(seqnum)
	if err != nil {
		return err
	}

	// Extract the fetch attachments
	for _, att := range attachments {
		err := att.extract(resp, msg)
		if err != nil {
			return err
		}
	}

	return nil
}

//---- Helper functions --------------------------------------------------------

// Add mailbox information to the given response
func (s *session) addMailboxInfo(resp *finalResponse) error {

	mailbox := s.mailbox.provider

	// Get the mailbox information from the mailstore
	firstUnseen, err := mailbox.FirstUnseen()
	if err != nil {
		return err
	}
	totalMessages, err := mailbox.TotalMessages()
	if err != nil {
		return err
	}
	recentMessages, err := mailbox.RecentMessages()
	if err != nil {
		return err
	}
	nextUid, err := mailbox.NextUid()
	if err != nil {
		return err
	}
	uidValidity, err := mailbox.UidValidity()
	if err != nil {
		return err
	}
	resp.putLine(fmt.Sprint(totalMessages, " EXISTS"))
	resp.putLine(fmt.Sprint(recentMessages, " RECENT"))
	if firstUnseen >= 0 {
		resp.putLine(fmt.Sprintf("OK [UNSEEN %d] Message %d is first unseen", firstUnseen, firstUnseen))
	}
	resp.putLine(fmt.Sprintf("OK [UIDVALIDITY %d] UIDs valid", uidValidity))
	resp.putLine(fmt.Sprintf("OK [UIDNEXT %d] Predicted next UID", nextUid))
	return nil
}

// copySlice copies a slice
func copySlice(s []string) []string {
	ret := make([]string, len(s), (len(s)+1)*2)
	copy(ret, s)
	return ret
}

// depthFirstMailboxes gets a recursive mailbox listing
// At the moment this doesn't support wildcards such as 'leader%' (are they used in real life?)
func (s *session) depthFirstMailboxes(
	results []*mailboxWrap, path []string, pattern []string) ([]*mailboxWrap, error) {

	mailstore := s.config.Mailstore

	// Stop recursing if the pattern is empty or if the path is too long
	if len(pattern) == 0 || len(path) > 20 {
		return results, nil
	}

	// Consider the next part of the pattern
	ret := results
	var err error
	pat := pattern[0]

	switch pat {
	case "%":
		// Get all the mailboxes at the current path
		all, err := getMailboxes(mailstore, s.user, path)
		if err == nil {
			for _, mbox := range all {
				// Consider the next pattern
				ret = append(ret, mbox)
				ret, err = s.depthFirstMailboxes(
					ret, mbox.provider.Path(), pattern[1:])
				if err != nil {
					break
				}
			}
		}

	case "*":
		// Get all the mailboxes at the current path
		all, err := getMailboxes(mailstore, s.user, path)
		if err == nil {
			for _, mbox := range all {
				// Keep using this pattern
				ret = append(ret, mbox)
				ret, err = s.depthFirstMailboxes(
					ret, mbox.provider.Path(), pattern)
				if err != nil {
					break
				}
			}
		}

	default:
		// Not a wildcard pattern
		mbox, err := getMailbox(mailstore, s.user, path)
		if err == nil {
			ret = append(results, mbox)
			ret, err = s.depthFirstMailboxes(
				ret, mbox.provider.Path(), pattern)
		}
	}

	return ret, err
}
