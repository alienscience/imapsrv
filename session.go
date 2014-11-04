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

// An IMAP mailbox
type Mailbox struct {
	Name string // The name of the mailbox
	Id   int64  // The id of the mailbox
}

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
func (s *session) selectMailbox(name string) (bool, error) {
	for _, mailstore := range s.config.Mailstores {
		// Lookup the mailbox
		mbox, err := mailstore.GetMailbox(name)

		if err != nil {
			return false, err
		}

		if mbox == nil {
			return false, nil
		}

		// Make note of the mailbox
		s.mailbox = mbox
		break
	}
	return true, nil
}

// Add mailbox information to the given response
func (s *session) addMailboxInfo(resp *response) error {
	for _, mailstore := range s.config.Mailstores {
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

		resp.extra(fmt.Sprint(totalMessages, " EXISTS"))
		resp.extra(fmt.Sprint(recentMessages, " RECENT"))
		resp.extra(fmt.Sprintf("OK [UNSEEN %d] Message %d is first unseen", firstUnseen, firstUnseen))
		resp.extra(fmt.Sprintf("OK [UIDVALIDITY %d] UIDs valid", s.mailbox.Id))
		resp.extra(fmt.Sprintf("OK [UIDNEXT %d] Predicted next UID", nextUid))
	}
	return nil
}
