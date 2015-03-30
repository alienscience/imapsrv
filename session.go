package imapsrv

import (
	"fmt"
	"github.com/jhillyerd/go.enmime"
	"log"
)

// IMAP session states
type state int

const (
	notAuthenticated = iota
	authenticated
	selected
)

// An IMAP session
type session struct {
	// The client id
	id int
	// The state of the session
	st state
	// The currently selected mailbox (if st == selected)
	mailbox *mailboxWrap
	// IMAP configuration
	config *Config
}

// Create a new IMAP session
func createSession(id int, config *Config) *session {
	return &session{
		id:     id,
		st:     notAuthenticated,
		config: config}
}

// Log a message with session information
func (s *session) log(info ...interface{}) {
	preamble := fmt.Sprintf("IMAP (%d) ", s.id)
	message := []interface{}{preamble}
	message = append(message, info...)
	log.Print(message...)
}

// Select a mailbox - returns true if the mailbox exists
func (s *session) selectMailbox(path []string) (bool, error) {
	// Lookup the mailbox
	mailstore := s.config.Mailstore
	mbox, err := getMailbox(mailstore, path)

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

// List mailboxes matching the given mailbox pattern
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
		mbox, err := getMailbox(s.config.Mailstore, path)
		if err != nil {
			return ret, err
		}
		ret = append(ret, mbox)
		return ret, nil
	}

	// Recursively get a listing
	return s.depthFirstMailboxes(ret, path, pattern[wildcard:len(pattern)])
}

// Fetch a mail message with the given sequence number
func (s *session) fetch(seqnum int32, attachments []fetchAttachment) (*messageData, error) {

	// Fetch the message
	mailbox := s.mailbox
	msg, err := mailbox.fetch(seqnum)
	if err != nil {
		return nil, err
	}

	ret := &messageData{
		seqNum: seqnum,
		fields: make(map[string]interface{}),
	}

	// Extract the fetch attachments
	for _, att := range attachments {
		err := extractFetchAttachment(ret, msg, att)
		if err != nil {
			return nil, err
		}
	}

	return ret, nil
}

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

	resp.put(fmt.Sprint(totalMessages, " EXISTS"))
	resp.put(fmt.Sprint(recentMessages, " RECENT"))
	resp.put(fmt.Sprintf("OK [UNSEEN %d] Message %d is first unseen", firstUnseen, firstUnseen))
	resp.put(fmt.Sprintf("OK [UIDVALIDITY %d] UIDs valid", uidValidity))
	resp.put(fmt.Sprintf("OK [UIDNEXT %d] Predicted next UID", nextUid))
	return nil
}

// Copies a slice
func copySlice(s []string) []string {
	ret := make([]string, len(s), (len(s)+1)*2)
	copy(ret, s)
	return ret
}

// Get a recursive mailbox listing
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
		all, err := getMailboxes(mailstore, path)
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
		all, err := getMailboxes(mailstore, path)
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
		mbox, err := getMailbox(mailstore, path)
		if err == nil {
			ret = append(results, mbox)
			ret, err = s.depthFirstMailboxes(
				ret, mbox.provider.Path(), pattern)
		}
	}

	return ret, err
}

// Extract a fetch attachment from a message and update the given messageData
func extractFetchAttachment(dest *messageData, msg *enmime.MIMEBody, att fetchAttachment) error {

	root := msg.Root

	switch att {
	case envelopeFetchAtt:
		// Add header fields
		header := root.Header()
		env := fmt.Print(
			"(", header["Date"], " ",
			header["Subject"], " ",
			header["From"], " ",
			header["Sender"], " ",
			header["Reply-To"], " ",
			header["To"], " ",
			header["Cc"], " ",
			header["Bcc"], " ",
			header["Bcc"], " ",
			header["In-Reply-To"], " ",
			header["Message-ID"], ")")
		dest["ENVELOPE"] = env
	case flagsFetchAtt:
	case internalDateFetchAtt:
	case rfc822HeaderFetchAtt:
	case rfc822SizeFetchAtt:
	case rfc822TextFetchAtt:
	case bodyFetchAtt:
	case bodyStructureFetchAtt:
	case uidFetchAtt:
	case bodySectionFetchAtt:
	case bodyPeekFetchAtt:
	default:
		// Unknown fetch attachment
	}

	// TODO: implement
	return nil
}
