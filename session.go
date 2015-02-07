package imapsrv

import (
	"fmt"
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
	mailbox *Mailbox
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
	mbox, err := mailstore.GetMailbox(path)

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
func (s *session) list(reference []string, pattern []string) ([]*Mailbox, error) {

	ret := make([]*Mailbox, 0, 4)
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
		mbox, err := s.config.Mailstore.GetMailbox(path)
		if err != nil {
			return ret, err
		}
		ret = append(ret, mbox)
		return ret, nil
	}

	// Recursively get a listing
	return s.depthFirstMailboxes(ret, path, pattern[wildcard:len(pattern)])
}

// Add mailbox information to the given response
func (s *session) addMailboxInfo(resp *finalResponse) error {
	mailstore := s.config.Mailstore

	// Get the mailbox information from the mailstore
	firstUnseen, err := mailstore.FirstUnseen(s.mailbox.Id)
	if err != nil {
		return err
	}
	totalMessages, err := mailstore.TotalMessages(s.mailbox.Id)
	if err != nil {
		return err
	}
	recentMessages, err := mailstore.RecentMessages(s.mailbox.Id)
	if err != nil {
		return err
	}
	nextUid, err := mailstore.NextUid(s.mailbox.Id)
	if err != nil {
		return err
	}

	resp.put(fmt.Sprint(totalMessages, " EXISTS"))
	resp.put(fmt.Sprint(recentMessages, " RECENT"))
	resp.put(fmt.Sprintf("OK [UNSEEN %d] Message %d is first unseen", firstUnseen, firstUnseen))
	resp.put(fmt.Sprintf("OK [UIDVALIDITY %d] UIDs valid", s.mailbox.Id))
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
	results []*Mailbox, path []string, pattern []string) ([]*Mailbox, error) {

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
		all, err := mailstore.GetMailboxes(path)
		if err == nil {
			for _, mbox := range all {
				// Consider the next pattern
				ret = append(ret, mbox)
				ret, err = s.depthFirstMailboxes(ret, mbox.Path, pattern[1:])
				if err != nil {
					break
				}
			}
		}

	case "*":
		// Get all the mailboxes at the current path
		all, err := mailstore.GetMailboxes(path)
		if err == nil {
			for _, mbox := range all {
				// Keep using this pattern
				ret = append(ret, mbox)
				ret, err = s.depthFirstMailboxes(ret, mbox.Path, pattern)
				if err != nil {
					break
				}
			}
		}

	default:
		// Not a wildcard pattern
		mbox, err := mailstore.GetMailbox(path)
		if err == nil {
			ret = append(results, mbox)
			ret, err = s.depthFirstMailboxes(ret, mbox.Path, pattern)
		}
	}

	return ret, err
}
